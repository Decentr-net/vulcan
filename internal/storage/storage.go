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

// Request ...
type Request struct {
	Owner       string       `db:"owner"`
	Email       string       `db:"email"`
	Address     string       `db:"address"`
	Code        string       `db:"code"`
	CreatedAt   time.Time    `db:"created_at"`
	ConfirmedAt sql.NullTime `db:"confirmed_at"`
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
}
