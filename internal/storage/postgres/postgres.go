// Package postgres is implementation of storage interface.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"

	"github.com/Decentr-net/vulcan/internal/storage"
)

const uniqueViolation = "unique_violation"

type pg struct {
	db *sql.DB
}

// New creates new instance of pg.
func New(db *sql.DB) storage.Storage {
	return pg{
		db: db,
	}
}

func (p pg) CreateRequest(ctx context.Context, owner, address, code string) error {
	if _, err := p.db.ExecContext(ctx, `
		INSERT INTO request VALUES($1, $2, $3, current_timestamp)
	`, owner, address, code); err != nil {
		if pqErr, isPqError := err.(*pq.Error); isPqError && pqErr.Code.Name() == uniqueViolation {
			return storage.ErrAlreadyExists
		}
		return fmt.Errorf("failed to insert request: %w", err)
	}

	return nil
}

func (p pg) GetNotConfirmedAccountAddress(ctx context.Context, owner, code string) (string, error) {
	var address string

	if err := p.db.QueryRowContext(ctx, `
		SELECT address FROM request WHERE owner=$1 AND code=$2 AND confirmed_at IS NULL
	`, owner, code).Scan(&address); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", storage.ErrNotFound
		}
		return "", fmt.Errorf("failed to exec query: %w", err)
	}

	return address, nil
}

func (p pg) MarkRequestConfirmed(ctx context.Context, owner string) error {
	res, err := p.db.ExecContext(ctx, `
		UPDATE request SET confirmed_at = current_timestamp WHERE owner = $1
	`, owner)

	if err != nil {
		return fmt.Errorf("failed to exec query: %w", err)
	}

	if c, _ := res.RowsAffected(); c == 0 {
		return storage.ErrNotFound
	}

	return nil
}
