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

func (p pg) GetRequestByOwnReferralCode(ctx context.Context, ownReferralCode string) (*storage.Request, error) {
	var r storage.Request
	if err := sqlx.GetContext(ctx, p.db, &r, `SELECT * FROM request WHERE own_referral_code=$1`, ownReferralCode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrReferralCodeNotFound
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

func (p pg) UpsertRequest(ctx context.Context, owner, email, address, code string, referralCode sql.NullString) error {
	if _, err := p.db.ExecContext(ctx, `
		INSERT INTO request (owner, email, address, code, created_at, registration_referral_code)
		    VALUES($1, $2, $3, $4, CURRENT_TIMESTAMP, $5) ON CONFLICT(email) DO
			UPDATE SET 
			           address=EXCLUDED.address, 
			           code=EXCLUDED.code, 
			           created_at=EXCLUDED.created_at,
			           registration_referral_code=EXCLUDED.registration_referral_code
	`, owner, email, address, code, referralCode); err != nil {
		if isUniqueViolationErr(err, "request_address_key") ||
			isUniqueViolationErr(err, "request_owner_key") {
			return storage.ErrAddressIsTaken
		}
		return fmt.Errorf("failed to exec query: %w", err)
	}

	return nil
}

func (p pg) CreateReferralTracking(ctx context.Context, receiver string, referralCode string) error {
	if _, err := p.db.ExecContext(ctx,
		`INSERT INTO referral_tracking (sender, receiver, registered_at) 
                VALUES (
                        (SELECT address FROM request WHERE own_referral_code = $2), 
                        $1, 
                        CURRENT_TIMESTAMP
			        )`,
		receiver, referralCode); err != nil {
		switch {
		case isNotNullViolationError(err, "sender"):
			return storage.ErrReferralCodeNotFound
		case isUniqueViolationErr(err, "referral_tracking_pkey"):
			return storage.ErrReferralTrackingExists
		default:
			return fmt.Errorf("failed to exec query: %w", err)
		}
	}
	return nil
}

func (p pg) GetReferralTrackingByReceiver(ctx context.Context, receiver string) (*storage.ReferralTracking, error) {
	var r storage.ReferralTracking
	if err := sqlx.GetContext(ctx, p.db, &r, `SELECT * FROM referral_tracking WHERE receiver=$1`, receiver); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to exec query: %w", err)
	}

	return &r, nil
}

func (p pg) MarkReferralTrackingInstalled(ctx context.Context, receiver string) error {
	_, err := p.db.ExecContext(ctx,
		`UPDATE referral_tracking
               SET status = 'installed',
                   installed_at = CURRENT_TIMESTAMP
                WHERE receiver = $1 and status = 'registered'`, receiver)
	if err != nil {
		return fmt.Errorf("failed to exec query: %w", err)
	}
	return nil
}

func isUniqueViolationErr(err error, constraint string) bool {
	if err1, ok := err.(*pq.Error); ok &&
		err1.Code == "23505" && err1.Constraint == constraint {
		return true
	}
	return false
}

func isNotNullViolationError(err error, column string) bool {
	if err1, ok := err.(*pq.Error); ok &&
		err1.Code == "23502" && err1.Column == column {
		return true
	}
	return false
}
