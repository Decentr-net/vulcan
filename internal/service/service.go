// Package service contains business logic of application.
package service

import (
	"context"
	"crypto/md5" // nolint:gosec
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/Decentr-net/vulcan/internal/blockchain"
	"github.com/Decentr-net/vulcan/internal/mail"
	"github.com/Decentr-net/vulcan/internal/storage"
)

const codeBytesSize = 3
const throttlingInterval = time.Minute

var plustPartRegexp = regexp.MustCompile(`\+.+\@`) // nolint

//go:generate mockgen -destination=./mock/service.go -package=mock -source=service.go

// ErrAlreadyExists is returned when request is already created for requested email or address.
var ErrAlreadyExists = fmt.Errorf("email or address is already taken")

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

// Service ...
type Service interface {
	Register(ctx context.Context, email, address string, referralCode *string) error
	Confirm(ctx context.Context, owner, code string) error
	GetRegisterStats(ctx context.Context) ([]*storage.RegisterStats, int, error)
	GetOwnReferralCode(ctx context.Context, address string) (string, error)
	GetRegistrationReferralCode(ctx context.Context, address string) (string, error)
	TrackReferralBrowserInstallation(ctx context.Context, address string) error
	GetReferralTrackingStats(ctx context.Context, address string) ([]*storage.ReferralTrackingStats, error)
}

// Service ...
type service struct {
	storage storage.Storage
	sender  mail.Sender
	btc     blockchain.Blockchain
	bmc     blockchain.Blockchain

	initialTestStakes int64
	initialMainStakes int64
}

// New creates new instance of service.
func New(
	storage storage.Storage,
	sender mail.Sender,
	bt, bm blockchain.Blockchain,
	initialTestNetStakes, initialMainNetStakes int64,
) Service {
	s := &service{
		storage:           storage,
		sender:            sender,
		btc:               bt,
		bmc:               bm,
		initialTestStakes: initialTestNetStakes,
		initialMainStakes: initialMainNetStakes,
	}

	return s
}

func (s *service) Register(ctx context.Context, email, address string, referralCode *string) error {
	var (
		owner = getEmailHash(truncatePlusPart(email))
		code  = randomCode()
	)

	if err := s.checkRegistrationConflicts(ctx, email, address); err != nil {
		return err
	}

	if err := s.sender.SendVerificationEmail(ctx, email, code); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	var referralCodeAsNullString sql.NullString
	if referralCode != nil {
		referralCodeAsNullString = sql.NullString{Valid: true, String: *referralCode}

		// check the given referral code exists
		if _, err := s.storage.GetRequestByOwnReferralCode(ctx, *referralCode); err != nil {
			if errors.Is(err, storage.ErrReferralCodeNotFound) {
				log.WithField("referral_code", *referralCode).
					Warn("referral code not found")
				referralCodeAsNullString = sql.NullString{Valid: false}
			} else {
				return fmt.Errorf("failed to get request by own referral code: %w", err)
			}
		}
	}

	if err := s.storage.UpsertRequest(ctx, owner, email, address, code, referralCodeAsNullString); err != nil {
		if errors.Is(err, storage.ErrAddressIsTaken) {
			return ErrAlreadyExists
		}
		return fmt.Errorf("failed to create request: %w", err)
	}

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

	if err := s.btc.SendStakes([]blockchain.Stake{{Address: req.Address, Amount: s.initialTestStakes}}); err != nil {
		log.WithError(err).WithField("address", req.Address).Error("failed to send stakes on testnet")
	}

	if err := s.bmc.SendStakes([]blockchain.Stake{{Address: req.Address, Amount: s.initialMainStakes}}); err != nil {
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

	if req.RegistrationReferralCode.Valid {
		// referral code has been provided during the registration, start tracking
		if err := s.storage.CreateReferralTracking(ctx, req.Owner, req.RegistrationReferralCode.String); err != nil {
			logger := log.WithField("referral_code", req.RegistrationReferralCode.String)

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
		return nil, fmt.Errorf("unexpected number of stats item: %d", len(stats)) // nolint:err113
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
	return stats, total, nil
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
