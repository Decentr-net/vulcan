// Package postgres is implementation of storage interface.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/Decentr-net/vulcan/internal/storage"
)

const uniqueViolationErrorCode = "23505"

type pg struct {
	db *sqlx.DB
}

// New creates new instance of pg.
func New(db *sql.DB) storage.Storage {
	return pg{
		db: sqlx.NewDb(db, "postgres"),
	}
}

func (p pg) GetRequestByOwner(ctx context.Context, owner string) (*storage.Request, error) {
	var r storage.Request
	if err := sqlx.GetContext(ctx, p.db, &r, `SELECT * FROM request WHERE owner=$1`, owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to exec query: %w", err)
	}

	return &r, nil
}

func (p pg) GetRequestByAddress(ctx context.Context, address string) (*storage.Request, error) {
	var r storage.Request
	if err := sqlx.GetContext(ctx, p.db, &r, `SELECT * FROM request WHERE address=$1`, address); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to exec query: %w", err)
	}

	return &r, nil
}

func (p pg) SetConfirmed(ctx context.Context, owner string) error {
	res, err := p.db.ExecContext(ctx, `
		UPDATE request SET confirmed_at=CURRENT_TIMESTAMP WHERE owner=$1
	`, owner)

	if err != nil {
		return fmt.Errorf("failed to exec query: %w", err)
	}

	if c, _ := res.RowsAffected(); c == 0 {
		return storage.ErrNotFound
	}

	return nil
}

func (p pg) UpsertRequest(ctx context.Context, owner, email, address, code string) error {
	if _, err := p.db.ExecContext(ctx, `
		INSERT INTO request VALUES($1, $2, $3, $4, CURRENT_TIMESTAMP) ON CONFLICT(email) DO
			UPDATE SET address=EXCLUDED.address, code=EXCLUDED.code, created_at=EXCLUDED.created_at
	`, owner, email, address, code); err != nil {
		if err, ok := err.(*pq.Error); ok && err.Code == uniqueViolationErrorCode {
			return storage.ErrAddressIsTaken
		}
		return fmt.Errorf("failed to exec query: %w", err)
	}

	return nil
}
