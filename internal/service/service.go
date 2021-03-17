// Package service contains business logic of application.
package service

import (
	"context"
	"crypto/md5" // nolint:gosec
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/lib/pq"

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

// ErrNotFound is returned when request not found for owner/code pair.
var ErrNotFound = fmt.Errorf("not found")

// ErrTooManyAttempts is returned when throttling interval didn't pass.
var ErrTooManyAttempts = fmt.Errorf("too many attempts")

// Service ...
type Service interface {
	Register(ctx context.Context, email, address string) error
	Confirm(ctx context.Context, owner, code string) error
}

// Service ...
type service struct {
	storage storage.Storage
	sender  mail.Sender
	bc      blockchain.Blockchain

	initialStakes int64
}

// New creates new instance of service.
func New(storage storage.Storage, sender mail.Sender, b blockchain.Blockchain, initialStakes int64) Service {
	return &service{
		storage:       storage,
		sender:        sender,
		bc:            b,
		initialStakes: initialStakes,
	}
}

func (s *service) Register(ctx context.Context, email, address string) error {
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

	if err := s.storage.UpsertRequest(ctx, owner, email, address, code); err != nil {
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

		if r, err = s.storage.GetRequestByOwner(ctx, getEmailHash(email)); err != nil && !errors.Is(err, storage.ErrNotFound) {
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
			return ErrNotFound
		}
		return fmt.Errorf("failed to check request: %w", err)
	}

	if req.ConfirmedAt.Valid {
		return ErrAlreadyConfirmed
	}

	if req.Code != code {
		return ErrNotFound
	}

	if err := s.bc.SendStakes(req.Address, s.initialStakes); err != nil {
		return fmt.Errorf("failed to send stakes to %s: %w", req.Address, err)
	}

	s.sender.SendWelcomeEmailAsync(ctx, req.Email)

	req.ConfirmedAt = pq.NullTime{
		Time:  time.Now(),
		Valid: true,
	}

	if err := s.storage.SetConfirmed(ctx, req.Owner); err != nil {
		return fmt.Errorf("failed to update request: %w", err)
	}

	return nil
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
