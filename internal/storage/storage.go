package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/lib/pq"
)

//go:generate mockgen -destination=./storage_mock.go -package=storage -source=storage.go

// ErrNotFound ...
var ErrNotFound = fmt.Errorf("not found")

// Request ...
type Request struct {
	Owner       string      `db:"owner"`
	Email       string      `db:"email"`
	Address     string      `db:"address"`
	Code        string      `db:"code"`
	CreatedAt   time.Time   `db:"created_at"`
	ConfirmedAt pq.NullTime `db:"confirmed_at"`
}

// Storage provides methods for interacting with database.
type Storage interface {
	// GetRequest returns request by owner or address.
	GetRequest(ctx context.Context, owner, address string) (*Request, error)
	// SetRequest sets request.
	SetRequest(ctx context.Context, r *Request) error
}
