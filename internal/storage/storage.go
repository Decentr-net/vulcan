package storage

import (
	"context"
	"fmt"
)

//go:generate mockgen -destination=./storage_mock.go -package=storage -source=storage.go

// ErrNotFound ...
var ErrNotFound = fmt.Errorf("not found")

// Storage provides methods for interacting with database.
type Storage interface {
	// IsRegistered checks if email or address has been already registered.
	IsRegistered(ctx context.Context, owner, address string) (bool, error)
	// CreateRequest creates initial registration request.
	CreateRequest(ctx context.Context, owner, address, code string) error
	// GetAccountAddress returns accounts address by owner and code or ErrNotFound if request is not found.
	GetAccountAddress(ctx context.Context, owner, code string) (string, error)
	// MarkRequestProcessed marks request as processed.
	MarkRequestProcessed(ctx context.Context, owner string) error
}
