// Package storage provides datasource functionality.
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

//go:generate mockgen -destination=./mock/storage.go -package=mock -source=storage.go

// ErrNotFound ...
var ErrNotFound = fmt.Errorf("not found")

// ErrAddressIsTaken ...
var ErrAddressIsTaken = fmt.Errorf("address is taken")

// ErrReferralTrackingExists ...
var ErrReferralTrackingExists = fmt.Errorf("referral tracking exists")

// ErrReferralCodeNotFound ...
var ErrReferralCodeNotFound = fmt.Errorf("referral code not found")

// Request ...
type Request struct {
	Owner                    string         `db:"owner"`
	Email                    string         `db:"email"`
	Address                  string         `db:"address"`
	Code                     string         `db:"code"`
	CreatedAt                time.Time      `db:"created_at"`
	ConfirmedAt              sql.NullTime   `db:"confirmed_at"`
	OwnReferralCode          string         `db:"own_referral_code"`
	RegistrationReferralCode sql.NullString `db:"registration_referral_code"`
	ReferralBanned           bool           `db:"referral_banned"`
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

// ReferralTracking ...
type ReferralTracking struct {
	Sender         string         `db:"sender"`
	Receiver       string         `db:"receiver"`
	Status         ReferralStatus `db:"status"`
	RegisteredAt   time.Time      `db:"registered_at"`
	InstalledAt    sql.NullTime   `db:"installed_at"`
	ConfirmedAt    sql.NullTime   `db:"confirmed_at"`
	SenderReward   sql.NullInt32  `db:"sender_reward"`
	ReceiverReward sql.NullInt32  `db:"receiver_reward"`
}

// ReferralTrackingStats ...
type ReferralTrackingStats struct {
	Registered int     `db:"registered"`
	Installed  int     `db:"installed"`
	Confirmed  int     `db:"confirmed"`
	Reward     sdk.Int `db:"reward"`
}

// RegisterStats ...
type RegisterStats struct {
	Date  time.Time `json:"date"`
	Value int       `json:"value"`
}

// Storage provides methods for interacting with database.
type Storage interface {
	// InTx runs code in transaction
	InTx(ctx context.Context, f func(s Storage) error) error
	// GetConfirmedRegistrationsTotal return a total number of all confirmed accounts (requests)
	GetConfirmedRegistrationsTotal(ctx context.Context) (int, error)
	// GetConfirmedRegistrationsStats return confirmed accounts stats for the last 30 days
	GetConfirmedRegistrationsStats(ctx context.Context) ([]*RegisterStats, error)
	// GetRequestByOwner returns request by owner.
	GetRequestByOwner(ctx context.Context, owner string) (*Request, error)
	// GetRequestByOwnReferralCode returns request by referral code.
	GetRequestByOwnReferralCode(ctx context.Context, ownReferralCode string) (*Request, error)
	// GetRequestByAddress returns request by address.
	GetRequestByAddress(ctx context.Context, address string) (*Request, error)
	// SetConfirmed sets request confirmed.
	SetConfirmed(ctx context.Context, owner string) error
	// CreateTestnetConfirmedRequest creates a confirmed request. Must be used only in Testnet.
	CreateTestnetConfirmedRequest(ctx context.Context, address string) error
	// UpsertRequest inserts request into storage.
	UpsertRequest(ctx context.Context, owner, email, address, code string, referralCode sql.NullString) error
	// CreateReferralTracking creates a new referral tracking
	CreateReferralTracking(ctx context.Context, receiver string, referralCode string) error
	// TransitionReferralTrackingToInstalled transitions referral tracking of the given referral code receiver as installed
	TransitionReferralTrackingToInstalled(ctx context.Context, receiver string) error
	// TransitionReferralTrackingToConfirmed transitions referral tracking as confirmed
	TransitionReferralTrackingToConfirmed(ctx context.Context, receiver string, senderReward, receiverReward sdk.Int) error
	// GetReferralTrackingByReceiver returns referral tracking by the given receiver address
	GetReferralTrackingByReceiver(ctx context.Context, receiver string) (*ReferralTracking, error)
	// GetReferralTrackingStats returns referral tracking stats: total + 30 last days
	GetReferralTrackingStats(ctx context.Context, sender string) ([]*ReferralTrackingStats, error)
	// GetUnconfirmedReferralTracking returns referral tracking installed more than given days  ago
	GetUnconfirmedReferralTracking(ctx context.Context, days int) ([]*ReferralTracking, error)
	// GetConfirmedReferralTrackingCount returns count of confirmed referrals
	GetConfirmedReferralTrackingCount(ctx context.Context, sender string) (int, error)
}
