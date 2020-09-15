package service

import (
	"context"
	"crypto/md5" // nolint:gosec
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/Decentr-net/vulcan/internal/blockchain"
	"github.com/Decentr-net/vulcan/internal/mail"
	"github.com/Decentr-net/vulcan/internal/storage"
)

const codeSize = 16

//go:generate mockgen -destination=./service_mock.go -package=service -source=service.go

// ErrAlreadyExists is returned when request is already created for requested email or address.
var ErrAlreadyExists = fmt.Errorf("email or address is busy")

// ErrNotFound is returned when request not found for owner/code pair.
var ErrNotFound = fmt.Errorf("not found")

const salt = "decentr-vulcan"

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
	owner := getEmailHash(email)

	code := randomCode()
	if err := s.storage.CreateRequest(ctx, owner, address, code); err != nil {
		if errors.Is(err, storage.ErrAlreadyExists) {
			return ErrAlreadyExists
		}
		return fmt.Errorf("failed to create request: %w", err)
	}

	if err := s.sender.Send(ctx, email, owner, code); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func (s *service) Confirm(ctx context.Context, owner, code string) error {
	address, err := s.storage.GetNotConfirmedAccountAddress(ctx, owner, code)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to check request: %w", err)
	}

	if err := s.bc.SendStakes(ctx, address, s.initialStakes); err != nil {
		return fmt.Errorf("failed to send stakes to %s: %w", owner, err)
	}

	if err := s.storage.MarkRequestConfirmed(ctx, owner); err != nil {
		return fmt.Errorf("failed to mark request processed: %w", err)
	}

	return nil
}

func getEmailHash(email string) string {
	b := md5.Sum([]byte(salt + email)) // nolint:gosec
	return hex.EncodeToString(b[:])
}

func randomCode() string {
	b := make([]byte, codeSize)
	_, _ = rand.Read(b)

	return hex.EncodeToString(b)
}
