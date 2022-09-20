// Package postgres is implementation of storage interface.
package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	log "github.com/sirupsen/logrus"

	"github.com/Decentr-net/vulcan/internal/storage"
)

var errBeginCalledWithinTx = errors.New("can not run in tx")

type pg struct {
	ext sqlx.ExtContext
}

type intDTO sdk.Int

func (i intDTO) Value() (driver.Value, error) {
	return sdk.Int(i).String(), nil
}

func (i *intDTO) Scan(value interface{}) error {
	if value == nil {
		*i = intDTO(sdk.ZeroInt())
	}
	switch t := value.(type) {
	case int64:
		*i = intDTO(sdk.NewInt(value.(int64)))
	default:
		return fmt.Errorf("failed to scan type %T into sdk.INT", t)
	}

	return nil
}

// New creates new instance of pg.
func New(db *sql.DB) storage.Storage {
	return pg{
		ext: sqlx.NewDb(db, "postgres"),
	}
}

func (p pg) InTx(ctx context.Context, f func(s storage.Storage) error) error {
	db, ok := p.ext.(*sqlx.DB)
	if !ok {
		return errBeginCalledWithinTx
	}

	tx, err := db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("failed to create tx: %w", err)
	}

	if err := func(s storage.Storage) error {
		if err := f(s); err != nil {
			return err
		}

		return nil
	}(pg{ext: tx}); err != nil {
		if err := tx.Rollback(); err != nil {
			log.WithError(err).Error("failed to rollback tx")
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commint tx: %w", err)
	}

	return nil
}

func (p pg) GetRequestByOwner(ctx context.Context, owner string) (*storage.Request, error) {
	var r storage.Request
	if err := sqlx.GetContext(ctx, p.ext, &r, `SELECT * FROM request WHERE owner=$1`, owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to exec query: %w", err)
	}

	return &r, nil
}

func (p pg) GetRequestByOwnReferralCode(ctx context.Context, ownReferralCode string) (*storage.Request, error) {
	var r storage.Request
	if err := sqlx.GetContext(ctx, p.ext, &r, `SELECT * FROM request WHERE own_referral_code=$1`, ownReferralCode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrReferralCodeNotFound
		}
		return nil, fmt.Errorf("failed to exec query: %w", err)
	}

	return &r, nil
}

func (p pg) GetRequestByAddress(ctx context.Context, address string) (*storage.Request, error) {
	var r storage.Request
	if err := sqlx.GetContext(ctx, p.ext, &r, `SELECT * FROM request WHERE address=$1`, address); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to exec query: %w", err)
	}

	return &r, nil
}

func (p pg) SetConfirmed(ctx context.Context, owner string) error {
	res, err := p.ext.ExecContext(ctx, `
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
	if _, err := p.ext.ExecContext(ctx, `
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

func (p pg) CreateTestnetConfirmedRequest(ctx context.Context, address string) error {
	uniqueValue := "[testnet]" + uuid.New().String()

	_, err := p.ext.ExecContext(ctx, `
			INSERT INTO request (owner, email, address, code, created_at, confirmed_at, registration_referral_code)
			VALUES($1, $2, $3, $4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, NULL) ON CONFLICT(address) DO NOTHING 
	`, uniqueValue, uniqueValue, address, uniqueValue)
	return err
}

func (p pg) CreateReferralTracking(ctx context.Context, receiver string, referralCode string) error {
	if _, err := p.ext.ExecContext(ctx, `
			INSERT INTO referral_tracking (sender, receiver, registered_at) 
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

func (p pg) CreateDLoan(ctx context.Context, address, firstName, lastName string, pdv float64) error {
	_, err := p.ext.ExecContext(ctx, `
			INSERT INTO dloan (address, first_name, last_name, pdv, created_at)
			VALUES($1, $2, $3, $4, CURRENT_TIMESTAMP) 
	`, address, firstName, lastName, pdv)
	return err
}

func (p pg) GetDLoans(ctx context.Context) (loans []*storage.DLoan, err error) {
	err = sqlx.SelectContext(ctx, p.ext, &loans, `
				SELECT * FROM dloan ORDER BY created_at`)
	return loans, err
}

func (p pg) GetReferralTrackingByReceiver(ctx context.Context, receiver string) (*storage.ReferralTracking, error) {
	var r storage.ReferralTracking
	if err := sqlx.GetContext(ctx, p.ext, &r, `SELECT * FROM referral_tracking WHERE receiver=$1`, receiver); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to exec query: %w", err)
	}

	return &r, nil
}

func (p pg) GetReferralTrackingStats(ctx context.Context, sender string) ([]*storage.ReferralTrackingStats, error) {
	var dto []*struct {
		Registered int    `db:"registered"`
		Installed  int    `db:"installed"`
		Confirmed  int    `db:"confirmed"`
		Reward     intDTO `db:"reward"`
	}
	err := sqlx.SelectContext(ctx, p.ext, &dto, `
				SELECT * FROM referral_tracking_sender_stats($1, NULL)	
				UNION ALL
				SELECT * FROM referral_tracking_sender_stats($1, '30 days'::INTERVAL)`, sender)

	stats := make([]*storage.ReferralTrackingStats, len(dto))
	for i, v := range dto {
		stats[i] = &storage.ReferralTrackingStats{
			Registered: v.Registered,
			Installed:  v.Installed,
			Confirmed:  v.Confirmed,
			Reward:     sdk.Int(v.Reward),
		}
	}

	return stats, err
}

func (p pg) GetConfirmedReferralTrackingCount(ctx context.Context, sender string) (int, error) {
	var count int
	err := sqlx.GetContext(ctx, p.ext, &count, `
		SELECT COUNT(*) FROM referral_tracking 
		WHERE status = 'confirmed' AND sender = $1`, sender)
	return count, err
}

func (p pg) TransitionReferralTrackingToInstalled(ctx context.Context, receiver string) error {
	_, err := p.ext.ExecContext(ctx, `
				UPDATE referral_tracking
				SET status = 'installed',
					installed_at = CURRENT_TIMESTAMP
				WHERE receiver = $1 and status = 'registered'`, receiver)
	if err != nil {
		return fmt.Errorf("failed to exec query: %w", err)
	}
	return nil
}

func (p pg) TransitionReferralTrackingToConfirmed(ctx context.Context, receiver string,
	senderReward, receiverReward sdk.Int) error {
	_, err := p.ext.ExecContext(ctx, `
				UPDATE referral_tracking
				SET status = 'confirmed',
					sender_reward = $2,
					receiver_reward = $3,
					confirmed_at = CURRENT_TIMESTAMP
				WHERE receiver = $1`, receiver, intDTO(senderReward), intDTO(receiverReward))
	if err != nil {
		return fmt.Errorf("failed to exec query: %w", err)
	}
	return nil
}

func (p pg) GetConfirmedRegistrationsStats(ctx context.Context) ([]*storage.RegisterStats, error) {
	var stats []*storage.RegisterStats
	err := sqlx.SelectContext(ctx, p.ext, &stats, `
				SELECT confirmed_at::DATE as date, COUNT(*) as value
				FROM request
				WHERE confirmed_at IS NOT NULL AND confirmed_at > NOW() -'90 day'::INTERVAL
				GROUP BY date
				ORDER BY date DESC
	`)
	return stats, err
}

func (p pg) GetUnconfirmedReferralTracking(ctx context.Context, days int) ([]*storage.ReferralTracking, error) {
	var rt []*storage.ReferralTracking
	err := sqlx.SelectContext(ctx, p.ext, &rt, fmt.Sprintf(`
				SELECT *
				FROM referral_tracking
				WHERE status = 'installed' AND installed_at < NOW() -'%d day'::INTERVAL AND
					sender NOT IN (SELECT address FROM request WHERE referral_banned)
	`, days))
	return rt, err
}

func (p pg) GetConfirmedRegistrationsTotal(ctx context.Context) (int, error) {
	var total int
	err := sqlx.GetContext(ctx, p.ext, &total, `
				SELECT COUNT(*) FROM request
				WHERE confirmed_at IS NOT NULL 
	`)
	return total, err
}

func (p pg) DoesEmailHaveFraudDomain(ctx context.Context, email string) (bool, error) {
	var check bool
	err := sqlx.GetContext(ctx, p.ext, &check,
		`SELECT EXISTS(SELECT * FROM email_fraud_domains WHERE $1 LIKE '%'||domain)`, email)

	return check, err
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
