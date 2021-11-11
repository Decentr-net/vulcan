// Package storage provides datasource functionality.
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

//go:generate mockgen -destination=./mock/storage.go -package=mock -source=storage.go

// ErrNotFound ...
var ErrNotFound = fmt.Errorf("not found")

// ErrAddressIsTaken ...
var ErrAddressIsTaken = fmt.Errorf("address is taken")

// ErrReferralExists ...
var ErrReferralExists = fmt.Errorf("referral exists")

// Request ...
type Request struct {
	Owner        string       `db:"owner"`
	Email        string       `db:"email"`
	Address      string       `db:"address"`
	Code         string       `db:"code"`
	CreatedAt    time.Time    `db:"created_at"`
	ConfirmedAt  sql.NullTime `db:"confirmed_at"`
	ReferralCode string       `db:"referral_code"`
}

// ReferralStatus represents a referral workflow status: registered -> installed -> confirmed.
type ReferralStatus string

const (
	// RegisteredReferralStatus means the receiver registered with the sender referral code.
	RegisteredReferralStatus ReferralStatus = "registered"
	// InstalledReferralStatus means the receiver installed the Browser and restored the account with their seed.
	InstalledReferralStatus ReferralStatus = "installed"
	// ConfirmedReferralStatus means the reward has been sent to the sender and receiver.
	ConfirmedReferralStatus ReferralStatus = "confirmed"
)

// Referral ...
type Referral struct {
	Sender         string         `db:"sender"`
	Receiver       string         `db:"receiver"`
	Status         ReferralStatus `db:"status"`
	RegisteredAt   time.Time      `db:"registered_at"`
	InstalledAt    sql.NullTime   `db:"installed_at"`
	ConfirmedAt    sql.NullTime   `db:"confirmed_at"`
	SenderReward   sql.NullInt32  `db:"sender_reward"`
	ReceiverReward sql.NullInt32  `db:"receiver_reward"`
}

// Storage provides methods for interacting with database.
type Storage interface {
	// GetRequestByOwner returns request by owner.
	GetRequestByOwner(ctx context.Context, owner string) (*Request, error)
	// GetRequestByAddress returns request by address.
	GetRequestByAddress(ctx context.Context, address string) (*Request, error)
	// SetConfirmed sets request confirmed.
	SetConfirmed(ctx context.Context, owner string) error
	// UpsertRequest inserts request into storage.
	UpsertRequest(ctx context.Context, owner, email, address, code string) error

	// CreateReferral creates a new referral
	CreateReferral(ctx context.Context, referral *Referral) error
}
