// Package postgres is implementation of storage interface.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/Decentr-net/vulcan/internal/storage"
)

type pg struct {
	db *sqlx.DB
}

// New creates new instance of pg.
func New(db *sql.DB) storage.Storage {
	return pg{
		db: sqlx.NewDb(db, "postgres"),
	}
}

func (p pg) GetRequest(ctx context.Context, owner, address string) (*storage.Request, error) {
	var r storage.Request
	if err := sqlx.GetContext(ctx, p.db, &r, `SELECT * FROM request WHERE owner=$1 OR address=$2`, owner, address); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to exec query: %w", err)
	}

	return &r, nil
}

func (p pg) SetRequest(ctx context.Context, r *storage.Request) error {
	if _, err := sqlx.NamedExecContext(ctx, p.db, `
		INSERT INTO request VALUES(:owner, :email, :address, :code, :created_at, :confirmed_at) ON CONFLICT(address) DO
			UPDATE SET code=EXCLUDED.code, created_at=EXCLUDED.created_at, confirmed_at=EXCLUDED.confirmed_at
	`, r); err != nil {
		return fmt.Errorf("failed to exec query: %w", err)
	}

	return nil
}
