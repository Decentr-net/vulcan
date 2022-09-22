// Package service contains business logic of application.
package service

import (
	"context"
	"crypto/md5" // nolint:gosec
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	log "github.com/sirupsen/logrus"

	"github.com/Decentr-net/vulcan/internal/blockchain"
	"github.com/Decentr-net/vulcan/internal/mail"
	"github.com/Decentr-net/vulcan/internal/referral"
	"github.com/Decentr-net/vulcan/internal/storage"
)

const codeBytesSize = 3
const throttlingInterval = time.Minute

// nolint
var giveStakesAmount = sdk.NewInt(1e9) // only for testnet

var plustPartRegexp = regexp.MustCompile(`\+.+\@`) // nolint

//go:generate mockgen -destination=./mock/service.go -package=mock -source=service.go

// ErrAlreadyExists is returned when request is already created for requested email or address.
var ErrAlreadyExists = fmt.Errorf("email or address is already taken")

// ErrRecaptcha is returned when captcha isn't passed.
var ErrRecaptcha = fmt.Errorf("recaptcha error")

// ErrAlreadyConfirmed is returned when request is already confirmed.
var ErrAlreadyConfirmed = fmt.Errorf("already confirmed")

// ErrRequestNotFound is returned when request not found for owner/code pair.
var ErrRequestNotFound = fmt.Errorf("request not found")

// ErrTooManyAttempts is returned when throttling interval didn't pass.
var ErrTooManyAttempts = fmt.Errorf("too many attempts")

// ErrReferralTrackingNotFound ...
var ErrReferralTrackingNotFound = fmt.Errorf("referral tracking not found")

// ErrReferralTrackingInvalidStatus ...
var ErrReferralTrackingInvalidStatus = fmt.Errorf("referral tracking has invalid status")

// ErrReferralCodeNotFound ...
var ErrReferralCodeNotFound = fmt.Errorf("referral code not found")

// ErrFraudEmail ...
var ErrFraudEmail = fmt.Errorf("email from fraud domain")

// Service ...
type Service interface {
	Register(ctx context.Context, email, address string, referralCode *string) error
	Confirm(ctx context.Context, owner, code string) error
	GetRegisterStats(ctx context.Context) ([]*storage.RegisterStats, int, error)
	GetOwnReferralCode(ctx context.Context, address string) (string, error)
	GetReferralConfig() referral.Config
	GetRegistrationReferralCode(ctx context.Context, address string) (string, error)
	TrackReferralBrowserInstallation(ctx context.Context, address string) error
	GetReferralTrackingStats(ctx context.Context, address string) ([]*storage.ReferralTrackingStats, error)
	CreateDLoanRequest(ctx context.Context, address, firstName, lastName string, pdv float64) error

	RegisterTestnetAccount(ctx context.Context, address string) error

	CheckRecaptcha(ctx context.Context, action, recaptchaResponse string) error
}

// Service ...
type service struct {
	storage storage.Storage
	sender  mail.Sender
	bc      blockchain.Blockchain

	rc              referral.Config
	recaptchaSecret string

	initialStakes sdk.Int
	initialMemo   string
}

// New creates new instance of service.
func New(
	storage storage.Storage,
	sender mail.Sender,
	bc blockchain.Blockchain,
	initialStakes sdk.Int,
	initialMemo string,
	rc referral.Config,
	recaptchaSecret string,
) Service {
	s := &service{
		storage:         storage,
		sender:          sender,
		bc:              bc,
		rc:              rc,
		recaptchaSecret: recaptchaSecret,
		initialStakes:   initialStakes,
		initialMemo:     initialMemo,
	}

	return s
}

func (s *service) GetReferralConfig() referral.Config {
	return s.rc
}

func (s *service) Register(ctx context.Context, email, address string, referralCode *string) error {
	var (
		owner = getEmailHash(truncatePlusPart(email))
		code  = randomCode()
	)

	if err := s.checkRegistrationConflicts(ctx, email, address); err != nil {
		return err
	}

	isFraud, err := s.storage.DoesEmailHaveFraudDomain(ctx, email)
	if err != nil {
		return fmt.Errorf("failed to check for fraud: %w", err)
	}

	if isFraud {
		return ErrFraudEmail
	}

	var referralCodeAsNullString sql.NullString
	if referralCode != nil {
		referralCodeAsNullString = sql.NullString{Valid: true, String: *referralCode}

		// check the given referral code exists
		var (
			req *storage.Request
			err error
		)
		if req, err = s.storage.GetRequestByOwnReferralCode(ctx, *referralCode); err != nil {
			if errors.Is(err, storage.ErrReferralCodeNotFound) {
				return ErrReferralCodeNotFound
			}
			return fmt.Errorf("failed to get request by own referral code: %w", err)
		}

		if req.ReferralBanned {
			log.WithField("address", req.Address).WithField("referral code", req.OwnReferralCode).
				Warn("referral code banned")
			return ErrReferralCodeNotFound
		}
	}

	if err := s.storage.UpsertRequest(ctx, owner, email, address, code, referralCodeAsNullString); err != nil {
		if errors.Is(err, storage.ErrAddressIsTaken) {
			return ErrAlreadyExists
		}
		return fmt.Errorf("failed to create request: %w", err)
	}

	s.sender.SendVerificationEmailAsync(ctx, email, code)

	return nil
}

func (s *service) CreateDLoanRequest(ctx context.Context, address, firstName, lastName string, pdv float64) error {
	if err := s.storage.CreateDLoan(ctx, address, firstName, lastName, pdv); err != nil {
		if errors.Is(err, storage.ErrAddressIsTaken) {
			return ErrAlreadyExists
		}
		return fmt.Errorf("failed to create dLoan request: %w", err)
	}

	log.WithFields(log.Fields{
		"sender":    "slack",
		"address":   address,
		"firstName": firstName,
		"lastName":  lastName,
		"pdv":       pdv,
	}).Info("dLoan request")

	return nil
}

func (s *service) checkRegistrationConflicts(ctx context.Context, email, address string) error {
	r, err := s.storage.GetRequestByAddress(ctx, address)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("failed to check conflicts: %w", err)
		}

		if r, err = s.storage.GetRequestByOwner(ctx, getEmailHash(truncatePlusPart(email))); err != nil && !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("failed to check conflicts: %w", err)
		}
	}

	if errors.Is(err, storage.ErrNotFound) {
		return nil
	}

	if r.Email != email {
		return fmt.Errorf("%w: address is already taken", ErrAlreadyExists)
	}

	if r.CreatedAt.Add(throttlingInterval).After(time.Now()) {
		return ErrTooManyAttempts
	}
	if r.ConfirmedAt.Valid {
		return ErrAlreadyExists
	}

	return nil
}

func (s *service) Confirm(ctx context.Context, email, code string) error {
	req, err := s.storage.GetRequestByOwner(ctx, getEmailHash(truncatePlusPart(email)))
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return ErrRequestNotFound
		}
		return fmt.Errorf("failed to check request: %w", err)
	}

	if req.ConfirmedAt.Valid {
		return ErrAlreadyConfirmed
	}

	if req.Code != code {
		return ErrRequestNotFound
	}

	if err := s.bc.SendStakes([]blockchain.Stake{{Address: req.Address, Amount: s.initialStakes}}, s.initialMemo); err != nil {
		return fmt.Errorf("failed to send stakes to %s on mainnet: %w", req.Address, err)
	}

	s.sender.SendWelcomeEmailAsync(ctx, req.Email)

	req.ConfirmedAt = sql.NullTime{
		Time:  time.Now(),
		Valid: true,
	}

	if err := s.storage.SetConfirmed(ctx, req.Owner); err != nil {
		return fmt.Errorf("failed to update request: %w", err)
	}

	logger := log.WithFields(log.Fields{
		"code":          req.Code,
		"address":       req.Address,
		"created_at":    req.CreatedAt,
		"email":         req.Email,
		"owner":         req.Owner,
		"referral_code": req.RegistrationReferralCode.String,
	})

	if req.RegistrationReferralCode.Valid {
		// referral code has been provided during the registration, start tracking
		if err := s.storage.CreateReferralTracking(ctx, req.Address, req.RegistrationReferralCode.String); err != nil {
			switch err {
			case storage.ErrReferralTrackingExists:
				logger.Warn("referral tracking already exists")
			case storage.ErrReferralCodeNotFound:
				logger.Warn("referral code not found")
			default:
				return fmt.Errorf("failed to create a  referral tracking: %w", err)
			}
		}
	}

	logger.Info("registration complete")

	return nil
}

func (s *service) GetOwnReferralCode(ctx context.Context, address string) (string, error) {
	req, err := s.storage.GetRequestByAddress(ctx, address)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return "", ErrRequestNotFound
		}
		return "", fmt.Errorf("failed to get referral code: %w", err)
	}

	return req.OwnReferralCode, nil
}

func (s *service) TrackReferralBrowserInstallation(ctx context.Context, address string) error {
	rt, err := s.storage.GetReferralTrackingByReceiver(ctx, address)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return ErrReferralTrackingNotFound
		}
		return err
	}

	if rt.Status != storage.RegisteredReferralStatus {
		return ErrReferralTrackingInvalidStatus
	}

	if err := s.storage.TransitionReferralTrackingToInstalled(ctx, address); err != nil {
		return fmt.Errorf("failed to mark referral tracking installed: %w", err)
	}

	log.WithFields(log.Fields{
		"sender":        rt.Sender,
		"receiver":      rt.Receiver,
		"registered_at": rt.RegisteredAt,
	}).Info("referral tracking installed")

	return nil
}

func (s *service) GetRegistrationReferralCode(ctx context.Context, address string) (string, error) {
	req, err := s.storage.GetRequestByAddress(ctx, address)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return "", ErrRequestNotFound
		}
		return "", fmt.Errorf("failed to get referral code: %w", err)
	}

	if !req.RegistrationReferralCode.Valid {
		return "", ErrRequestNotFound
	}

	return req.RegistrationReferralCode.String, nil
}

func (s *service) GetReferralTrackingStats(ctx context.Context, address string) ([]*storage.ReferralTrackingStats, error) {
	_, err := s.storage.GetRequestByAddress(ctx, address)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrRequestNotFound
		}
		return nil, fmt.Errorf("failed to get request by address: %w", err)
	}

	stats, err := s.storage.GetReferralTrackingStats(ctx, address)
	if err != nil {
		return nil, err
	}

	if len(stats) != 2 {
		return nil, fmt.Errorf("unexpected number of stats item: %d", len(stats))
	}

	return stats, err
}

func (s *service) GetRegisterStats(ctx context.Context) ([]*storage.RegisterStats, int, error) {
	stats, err := s.storage.GetConfirmedRegistrationsStats(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get stats:%w", err)
	}
	total, err := s.storage.GetConfirmedRegistrationsTotal(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get total:%w", err)
	}

	transformStatsAsGrowth(stats, total)
	return stats, total, nil
}

func (s *service) RegisterTestnetAccount(ctx context.Context, address string) error {
	if err := s.bc.SendStakes([]blockchain.Stake{
		{
			Address: address,
			Amount:  giveStakesAmount,
		},
	}, ""); err != nil {
		return fmt.Errorf("failed to give stakes: %w", err)
	}

	if err := s.storage.CreateTestnetConfirmedRequest(ctx, address); err != nil {
		return fmt.Errorf("failed to create confirmed request: %w", err)
	}

	return nil
}

func (s *service) CheckRecaptcha(ctx context.Context, action, recaptchaResponse string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://www.google.com/recaptcha/api/siteverify", nil)
	if err != nil {
		return err
	}

	// Add necessary request parameters.
	q := url.Values{}
	q.Add("secret", s.recaptchaSecret)
	q.Add("response", recaptchaResponse)
	req.URL.RawQuery = q.Encode()

	// Make request
	c := http.Client{Timeout: time.Second * 5}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() // nolint

	// Decode response.
	var body struct {
		Success     bool      `json:"success"`
		Score       float64   `json:"score"`
		Action      string    `json:"action"`
		ChallengeTS time.Time `json:"challenge_ts"`
		Hostname    string    `json:"hostname"`
		ErrorCodes  []string  `json:"error-codes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return fmt.Errorf("failed to unmarshal recaptcha response: %w", err)
	}

	// Check recaptcha verification success.
	if !body.Success {
		return fmt.Errorf("%w: unsuccessful recaptcha verify request", ErrRecaptcha)
	}

	// Check response score.
	if body.Score < 0.8 {
		return fmt.Errorf("%w: lower received score than expected", ErrRecaptcha)
	}

	// Check response action.
	if body.Action != action {
		return fmt.Errorf("%w: mismatched recaptcha action", ErrRecaptcha)
	}

	return nil
}

func transformStatsAsGrowth(stats []*storage.RegisterStats, total int) {
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Date.Before(stats[j].Date)
	})

	for i := len(stats) - 1; i >= 0; i-- {
		v := stats[i].Value
		stats[i].Value = total
		total -= v
	}
}

func truncatePlusPart(email string) string {
	return plustPartRegexp.ReplaceAllString(email, "@")
}

func getEmailHash(email string) string {
	b := md5.Sum([]byte(strings.ToLower(email))) // nolint:gosec
	return hex.EncodeToString(b[:])
}

func randomCode() string {
	b := make([]byte, codeBytesSize)
	_, _ = rand.Read(b)

	return hex.EncodeToString(b)
}
