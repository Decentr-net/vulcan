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
	// IsRegistered checks if email has been already registered.
	IsRegistered(ctx context.Context, owner string) (bool, error)
	// CreateRequest creates initial registration request.
	CreateRequest(ctx context.Context, owner, code string) error
	// CheckRequest searches for request with owner/code pair and returns ErrNotFound if such request was not found.
	CheckRequest(ctx context.Context, owner, code string) error
	// MarkRequestProcessed marks request as processed.
	MarkRequestProcessed(ctx context.Context, owner string) error
}
