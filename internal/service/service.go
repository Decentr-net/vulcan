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

// ErrEmailIsBusy is returned when wallet is already created for requested email.
var ErrEmailIsBusy = fmt.Errorf("email is busy")

// ErrNotFound is returned when request not found for owner/code pair.
var ErrNotFound = fmt.Errorf("not found")

const salt = "decentr-vulcan"

// Service ...
type Service interface {
	Register(ctx context.Context, email string) error
	Confirm(ctx context.Context, owner, code string) (AccountInfo, error)
}

// AccountInfo contains private and public info about created decentr account.
type AccountInfo struct {
	Address  string
	PubKey   string
	Mnemonic []string
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

func (s *service) Register(ctx context.Context, email string) error {
	owner := getEmailHash(email)

	isRegistered, err := s.storage.IsRegistered(ctx, owner)
	if err != nil {
		return fmt.Errorf("failed to check existence: %w", err)
	}

	if isRegistered {
		return ErrEmailIsBusy
	}

	code := randomCode()
	if err := s.storage.CreateRequest(ctx, owner, code); err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if err := s.sender.Send(ctx, email, owner, code); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func (s *service) Confirm(ctx context.Context, owner, code string) (AccountInfo, error) {
	if err := s.storage.CheckRequest(ctx, owner, code); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return AccountInfo{}, ErrNotFound
		}
		return AccountInfo{}, fmt.Errorf("failed to check request: %w", err)
	}

	acc, err := s.bc.CreateWallet(ctx)
	if err != nil {
		return AccountInfo{}, fmt.Errorf("failed to create wallet: %w", err)
	}

	if err := s.bc.SendStakes(ctx, acc.Address, s.initialStakes); err != nil {
		return AccountInfo{}, fmt.Errorf("failed to send stakes to %s: %w", owner, err)
	}

	if err := s.storage.MarkRequestProcessed(ctx, owner); err != nil {
		return AccountInfo{}, fmt.Errorf("failed to mark request processed: %w", err)
	}

	return AccountInfo{
		Address:  acc.Address,
		PubKey:   acc.PubKey,
		Mnemonic: acc.Mnemonic,
	}, nil
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
