//go:build integration
// +build integration

package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	m "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/Decentr-net/vulcan/internal/storage"
)

var (
	db  *sql.DB
	ctx = context.Background()
	s   storage.Storage
)

func TestMain(m *testing.M) {
	shutdown := setup()

	s = New(db)

	code := m.Run()
	shutdown()
	os.Exit(code)
}

func setup() func() {
	req := testcontainers.ContainerRequest{
		Image:        "postgres:12",
		Env:          map[string]string{"POSTGRES_PASSWORD": "root"},
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor:   wait.ForListeningPort("5432/tcp"),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
	})
	if err != nil {
		logrus.WithError(err).Fatalf("failed to create container")
	}

	if err := c.Start(ctx); err != nil {
		logrus.WithError(err).Fatal("failed to start container")
	}

	host, err := c.Host(ctx)
	if err != nil {
		logrus.WithError(err).Fatal("failed to get host")
	}

	port, err := c.MappedPort(ctx, "5432")
	if err != nil {
		logrus.WithError(err).Fatal("failed to map port")
	}

	dsn := fmt.Sprintf("host=%s port=%d user=postgres password=root sslmode=disable", host, port.Int())

	db, err = sql.Open("postgres", dsn)
	if err != nil {
		logrus.WithError(err).Fatal("failed to open connection")
	}

	if err := db.Ping(); err != nil {
		logrus.WithError(err).Fatal("failed to ping postgres")
	}

	shutdownFn := func() {
		if c != nil {
			c.Terminate(ctx)
		}
	}

	migrate("postgres", "root", host, "postgres", port.Int())

	return shutdownFn
}

func migrate(username, password, hostname, dbname string, port int) {
	_, currFile, _, ok := runtime.Caller(0)
	if !ok {
		logrus.Fatal("failed to get current file location")
	}

	migrations := filepath.Join(currFile, "../../../../scripts/migrations/postgres/")

	migrator, err := m.New(
		fmt.Sprintf("file://%s", migrations),
		fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
			username, password, hostname, port, dbname),
	)
	if err != nil {
		logrus.WithError(err).Fatal("failed to create migrator")
	}
	defer migrator.Close()

	if err := migrator.Up(); err != nil {
		logrus.WithError(err).Fatal("failed to migrate")
	}
}

func cleanup(t *testing.T) {
	_, err := db.ExecContext(ctx, "DELETE FROM referral_tracking")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "DELETE FROM request")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "DELETE FROM dloan")
	require.NoError(t, err)
}

func TestPg_InsertRequest(t *testing.T) {
	defer cleanup(t)

	require.NoError(t, s.UpsertRequest(ctx, "owner",
		"e@mail.com", "address", "code", sql.NullString{},
	))
	r, err := s.GetRequestByOwner(ctx, "owner")
	require.NoError(t, err)

	assert.Equal(t, "owner", r.Owner)
	assert.Equal(t, "e@mail.com", r.Email)
	assert.Equal(t, "address", r.Address)
	assert.Equal(t, "code", r.Code)
	assert.Equal(t, sql.NullString{}, r.RegistrationReferralCode)
	assert.False(t, r.CreatedAt.IsZero())
	assert.False(t, r.ConfirmedAt.Valid)
	assert.NotEmpty(t, r.OwnReferralCode)
	assert.Len(t, r.OwnReferralCode, 8)

	require.True(t, errors.Is(storage.ErrAddressIsTaken, s.UpsertRequest(ctx, "own", "em", "address", "code", sql.NullString{})))
	require.True(t, errors.Is(storage.ErrAddressIsTaken, s.UpsertRequest(ctx, "owner", "em", "address2", "code", sql.NullString{})))

	require.NoError(t, s.UpsertRequest(ctx, "owner", "e@mail.com",
		"new", "code2", sql.NullString{}))
	r, err = s.GetRequestByOwner(ctx, "owner")
	require.NoError(t, err)

	assert.Equal(t, "new", r.Address)
	assert.Equal(t, "code2", r.Code)
}

func TestPg_GetConfirmedRegistrationsTotal(t *testing.T) {
	defer cleanup(t)

	count, err := s.GetConfirmedRegistrationsTotal(ctx)
	require.NoError(t, err)
	require.Zero(t, count)

	require.NoError(t, s.UpsertRequest(ctx, "owner",
		"e@mail.com", "address", "code", sql.NullString{},
	))
	require.NoError(t, s.SetConfirmed(ctx, "owner"))

	count, err = s.GetConfirmedRegistrationsTotal(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestPg_GetConfirmedAccountStats(t *testing.T) {
	defer cleanup(t)

	stats, err := s.GetConfirmedRegistrationsStats(ctx)
	require.NoError(t, err)
	require.Len(t, stats, 0)

	for i := 0; i < 10; i++ {
		is := strconv.Itoa(i)
		require.NoError(t, s.UpsertRequest(ctx, "owner"+is,
			"e@mail.com"+is, "address"+is, "code"+is, sql.NullString{},
		))
		require.NoError(t, s.SetConfirmed(ctx, "owner"+is))
	}

	stats, err = s.GetConfirmedRegistrationsStats(ctx)
	require.NoError(t, err)
	require.Len(t, stats, 1)
	require.True(t, dateEqual(time.Now().UTC(), stats[0].Date))
	require.Equal(t, 10, stats[0].Value)
}

func dateEqual(date1, date2 time.Time) bool {
	y1, m1, d1 := date1.Date()
	y2, m2, d2 := date2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func TestPg_CreateDLoan(t *testing.T) {
	defer cleanup(t)

	require.NoError(t, s.CreateDLoan(ctx, "address",
		"firstName", "lastName", 50.56))

	loans, err := s.GetDLoans(ctx)
	require.NoError(t, err)
	require.Len(t, loans, 1)

	loan := loans[0]

	assert.Equal(t, "address", loan.Address)
	assert.Equal(t, "firstName", loan.FirstName)
	assert.Equal(t, "lastName", loan.LastName)
	assert.False(t, loan.CreatedAt.IsZero())
	assert.NotZero(t, loan.ID)
}

func TestPg_CreateReferralTracking(t *testing.T) {
	defer cleanup(t)

	require.NoError(t, s.UpsertRequest(ctx, "owner",
		"e@mail.com", "address", "code",
		sql.NullString{},
	))

	r, err := s.GetRequestByOwner(ctx, "owner")
	require.NoError(t, err)

	require.NoError(t, s.UpsertRequest(ctx, "owner2",
		"e2@mail.com", "address2", "code2",
		sql.NullString{Valid: true, String: r.OwnReferralCode},
	))

	require.Equal(t, storage.ErrReferralCodeNotFound, s.CreateReferralTracking(ctx, "receiver", "not exists"))
	require.NoError(t, s.CreateReferralTracking(ctx, "receiver", r.OwnReferralCode))
	require.Equal(t, storage.ErrReferralTrackingExists, s.CreateReferralTracking(ctx, "receiver", r.OwnReferralCode))
}

func TestPg_SetConfirmed(t *testing.T) {
	defer cleanup(t)

	require.NoError(t, s.UpsertRequest(ctx, "owner", "e@mail.com", "address", "code", sql.NullString{}))
	require.NoError(t, s.SetConfirmed(ctx, "owner"))
	r, err := s.GetRequestByOwner(ctx, "owner")
	require.NoError(t, err)

	assert.True(t, r.ConfirmedAt.Valid)

	assert.True(t, errors.Is(storage.ErrNotFound, s.SetConfirmed(ctx, "owner2")))
}

func TestPg_GetRequestByAddress(t *testing.T) {
	defer cleanup(t)

	require.NoError(t, s.UpsertRequest(ctx, "owner", "e@mail.com", "address", "code", sql.NullString{}))

	r, err := s.GetRequestByOwner(ctx, "owner")
	require.NoError(t, err)

	assert.Equal(t, "owner", r.Owner)
	assert.Equal(t, "e@mail.com", r.Email)
	assert.Equal(t, "address", r.Address)
	assert.Equal(t, "code", r.Code)
	assert.False(t, r.CreatedAt.IsZero())
	assert.False(t, r.ConfirmedAt.Valid)

	_, err = s.GetRequestByOwner(ctx, "not_exists")
	assert.True(t, errors.Is(err, storage.ErrNotFound))
}

func TestPg_GetRequestByOwner(t *testing.T) {
	defer cleanup(t)

	require.NoError(t, s.UpsertRequest(ctx, "owner", "e@mail.com", "address", "code", sql.NullString{}))

	r, err := s.GetRequestByAddress(ctx, "address")
	require.NoError(t, err)

	assert.Equal(t, "owner", r.Owner)
	assert.Equal(t, "e@mail.com", r.Email)
	assert.Equal(t, "address", r.Address)
	assert.Equal(t, "code", r.Code)
	assert.False(t, r.CreatedAt.IsZero())
	assert.False(t, r.ConfirmedAt.Valid)

	_, err = s.GetRequestByAddress(ctx, "not_exists")
	assert.True(t, errors.Is(err, storage.ErrNotFound))
}

func TestPg_CreateConfirmedRequest(t *testing.T) {
	defer cleanup(t)

	const addr1 = "addr1"

	require.NoError(t, s.CreateTestnetConfirmedRequest(context.Background(), addr1))
	require.NoError(t, s.CreateTestnetConfirmedRequest(context.Background(), addr1))
	require.NoError(t, s.CreateTestnetConfirmedRequest(context.Background(), "addr2"))

	req, err := s.GetRequestByAddress(ctx, addr1)
	require.NoError(t, err)
	require.Equal(t, addr1, req.Address)
}

func TestPg_GetReferralTrackingByReceiver(t *testing.T) {
	defer cleanup(t)

	const (
		receiverAddr = "receiver"
		senderArr    = "sender"
	)

	require.NoError(t, s.UpsertRequest(ctx, "owner",
		"e@mail.com", senderArr, "code",
		sql.NullString{},
	))

	r, err := s.GetRequestByOwner(ctx, "owner")
	require.NoError(t, err)

	require.NoError(t, s.CreateReferralTracking(ctx, receiverAddr, r.OwnReferralCode))
	rt, err := s.GetReferralTrackingByReceiver(ctx, receiverAddr)
	require.NoError(t, err)

	assert.Equal(t, storage.RegisteredReferralStatus, rt.Status)
	assert.Equal(t, receiverAddr, rt.Receiver)
	assert.Equal(t, senderArr, rt.Sender)
}

func TestPg_MarkReferralTrackingInstalled(t *testing.T) {
	defer cleanup(t)

	const (
		receiverAddr = "receiver"
		senderArr    = "sender"
	)

	require.NoError(t, s.UpsertRequest(ctx, "owner",
		"e@mail.com", senderArr, "code",
		sql.NullString{},
	))

	r, err := s.GetRequestByOwner(ctx, "owner")
	require.NoError(t, err)

	require.NoError(t, s.CreateReferralTracking(ctx, receiverAddr, r.OwnReferralCode))
	require.NoError(t, s.TransitionReferralTrackingToInstalled(ctx, receiverAddr))

	rt, err := s.GetReferralTrackingByReceiver(ctx, receiverAddr)
	require.NoError(t, err)

	assert.Equal(t, storage.InstalledReferralStatus, rt.Status)
	assert.Equal(t, receiverAddr, rt.Receiver)
	assert.Equal(t, senderArr, rt.Sender)

	// second time, should be no err
	require.NoError(t, s.TransitionReferralTrackingToInstalled(ctx, receiverAddr))
}

func TestPg_GetConfirmedReferralTrackingCount(t *testing.T) {
	defer cleanup(t)

	// zero
	count, err := s.GetConfirmedReferralTrackingCount(ctx, "sender")
	require.NoError(t, err)
	require.Zero(t, count)

	const (
		receiverAddr = "receiver"
		senderArr    = "sender"
	)

	// registered
	require.NoError(t, s.UpsertRequest(ctx, "owner",
		"e@mail.com", senderArr, "code",
		sql.NullString{},
	))

	r, err := s.GetRequestByOwner(ctx, "owner")
	require.NoError(t, err)

	require.NoError(t, s.CreateReferralTracking(ctx, receiverAddr, r.OwnReferralCode))
	require.NoError(t, s.TransitionReferralTrackingToConfirmed(ctx, receiverAddr, sdk.NewInt(10), sdk.NewInt(10)))

	count, err = s.GetConfirmedReferralTrackingCount(ctx, "sender")
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestPg_GetUnconfirmedReferralTracking(t *testing.T) {
	defer cleanup(t)

	requireNoUnconfirmed := func() {
		referrals, err := s.GetUnconfirmedReferralTracking(ctx, 30)
		require.NoError(t, err)
		require.Len(t, referrals, 0)
	}

	const (
		receiverAddr = "receiver"
		senderArr    = "sender"
	)

	requireNoUnconfirmed()

	// registered
	require.NoError(t, s.UpsertRequest(ctx, "owner",
		"e@mail.com", senderArr, "code",
		sql.NullString{},
	))

	r, err := s.GetRequestByOwner(ctx, "owner")
	require.NoError(t, err)

	require.NoError(t, s.CreateReferralTracking(ctx, receiverAddr, r.OwnReferralCode))
	requireNoUnconfirmed()

	require.NoError(t, s.TransitionReferralTrackingToInstalled(ctx, receiverAddr))
	requireNoUnconfirmed()

	_, err = db.ExecContext(ctx, `UPDATE referral_tracking SET installed_at = NOW() - '31 day'::interval`)
	require.NoError(t, err)

	referrals, err := s.GetUnconfirmedReferralTracking(ctx, 30)
	require.NoError(t, err)
	require.Len(t, referrals, 1)

	//banned
	_, err = db.ExecContext(ctx, `UPDATE request SET referral_banned = TRUE WHERE address = $1`, senderArr)
	require.NoError(t, err)

	// when banned there should be no unconfirmed referrals
	requireNoUnconfirmed()
}

func TestPg_DoesEmailHaveFraudDomain(t *testing.T) {
	check, err := s.DoesEmailHaveFraudDomain(context.Background(), "valid@gmail.com")
	require.NoError(t, err)
	require.False(t, check)

	check, err = s.DoesEmailHaveFraudDomain(context.Background(), "forbidden@aircase.tk")
	require.NoError(t, err)
	require.True(t, check)
}

func TestPg_GetReferralTrackingStats(t *testing.T) {
	defer cleanup(t)

	const (
		receiverAddr = "receiver"
		senderArr    = "sender"
	)

	statsEqual := func(exp, act storage.ReferralTrackingStats) {
		require.Equal(t, exp.Installed, act.Installed)
		require.Equal(t, exp.Registered, act.Registered)
		require.Equal(t, exp.Confirmed, act.Confirmed)
		if exp.Reward.IsNil() {
			require.True(t, act.Reward.IsZero())
		} else {
			require.True(t, exp.Reward.Equal(act.Reward))
		}
	}

	// empty
	stats, err := s.GetReferralTrackingStats(ctx, senderArr)
	require.NoError(t, err)
	require.Len(t, stats, 2)
	statsEqual(storage.ReferralTrackingStats{}, *stats[0])
	statsEqual(storage.ReferralTrackingStats{}, *stats[1])

	// registered
	require.NoError(t, s.UpsertRequest(ctx, "owner",
		"e@mail.com", senderArr, "code",
		sql.NullString{},
	))

	r, err := s.GetRequestByOwner(ctx, "owner")
	require.NoError(t, err)

	require.NoError(t, s.CreateReferralTracking(ctx, receiverAddr, r.OwnReferralCode))
	stats, err = s.GetReferralTrackingStats(ctx, senderArr)
	require.NoError(t, err)
	require.Len(t, stats, 2)
	statsEqual(storage.ReferralTrackingStats{
		Registered: 1,
	}, *stats[0])
	statsEqual(storage.ReferralTrackingStats{
		Registered: 1,
	}, *stats[1])

	// installed
	require.NoError(t, s.TransitionReferralTrackingToInstalled(ctx, receiverAddr))
	stats, err = s.GetReferralTrackingStats(ctx, senderArr)
	require.NoError(t, err)
	require.Len(t, stats, 2)
	statsEqual(storage.ReferralTrackingStats{
		Registered: 1,
		Installed:  1,
	}, *stats[0])
	statsEqual(storage.ReferralTrackingStats{
		Registered: 1,
		Installed:  1,
	}, *stats[1])

	// confirmed
	require.NoError(t, s.TransitionReferralTrackingToConfirmed(ctx, receiverAddr, sdk.NewInt(10), sdk.NewInt(5)))
	stats, err = s.GetReferralTrackingStats(ctx, senderArr)
	require.NoError(t, err)
	require.Len(t, stats, 2)
	statsEqual(storage.ReferralTrackingStats{
		Registered: 1,
		Installed:  1,
		Confirmed:  1,
		Reward:     sdk.NewInt(10),
	}, *stats[0])
	statsEqual(storage.ReferralTrackingStats{
		Registered: 1,
		Installed:  1,
		Confirmed:  1,
		Reward:     sdk.NewInt(10),
	}, *stats[1])
}
