//+build integration

package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

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
	_, err := db.ExecContext(ctx, "DELETE FROM request")
	require.NoError(t, err)
}

func TestPg_InsertRequest(t *testing.T) {
	defer cleanup(t)

	require.NoError(t, s.UpsertRequest(ctx, "owner", "e@mail.com", "address", "code"))
	r, err := s.GetRequestByOwner(ctx, "owner")
	require.NoError(t, err)

	assert.Equal(t, "owner", r.Owner)
	assert.Equal(t, "e@mail.com", r.Email)
	assert.Equal(t, "address", r.Address)
	assert.Equal(t, "code", r.Code)
	assert.False(t, r.CreatedAt.IsZero())
	assert.False(t, r.ConfirmedAt.Valid)

	require.True(t, errors.Is(storage.ErrAddressIsTaken, s.UpsertRequest(ctx, "own", "em", "address", "code")))
	require.True(t, errors.Is(storage.ErrAddressIsTaken, s.UpsertRequest(ctx, "owner", "em", "address2", "code")))

	require.NoError(t, s.UpsertRequest(ctx, "owner", "e@mail.com", "new", "code2"))
	r, err = s.GetRequestByOwner(ctx, "owner")
	require.NoError(t, err)

	assert.Equal(t, "new", r.Address)
	assert.Equal(t, "code2", r.Code)
}

func TestPg_SetConfirmed(t *testing.T) {
	defer cleanup(t)

	require.NoError(t, s.UpsertRequest(ctx, "owner", "e@mail.com", "address", "code"))
	require.NoError(t, s.SetConfirmed(ctx, "owner"))
	r, err := s.GetRequestByOwner(ctx, "owner")
	require.NoError(t, err)

	assert.True(t, r.ConfirmedAt.Valid)

	assert.True(t, errors.Is(storage.ErrNotFound, s.SetConfirmed(ctx, "owner2")))
}

func TestPg_GetRequestByAddress(t *testing.T) {
	defer cleanup(t)

	require.NoError(t, s.UpsertRequest(ctx, "owner", "e@mail.com", "address", "code"))

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

	require.NoError(t, s.UpsertRequest(ctx, "owner", "e@mail.com", "address", "code"))

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
