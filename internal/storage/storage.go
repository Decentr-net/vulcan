package storage

import (
	"context"
	"fmt"
)

//go:generate mockgen -destination=./storage_mock.go -package=storage -source=storage.go

// ErrNotFound ...
var ErrNotFound = fmt.Errorf("not found")

// ErrAlreadyExists ...
var ErrAlreadyExists = fmt.Errorf("email or address have been already used")

// Storage provides methods for interacting with database.
type Storage interface {
	// CreateRequest creates initial registration request.
	// It returns ErrAlreadyExists if email or address have been already used.
	CreateRequest(ctx context.Context, owner, address, code string) error
	// GetNotConfirmedAccountAddress returns accounts address by owner and code or ErrNotFound if request is not found.
	GetNotConfirmedAccountAddress(ctx context.Context, owner, code string) (string, error)
	// MarkConfirmed marks request as confirmed.
	MarkConfirmed(ctx context.Context, owner string) error
}
