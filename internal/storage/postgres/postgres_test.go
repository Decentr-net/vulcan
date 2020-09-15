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

	migrations := filepath.Join(currFile, "../../../../scripts/migrations/postgres/") // nolint

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

func TestPg_CreateRequest(t *testing.T) {
	defer cleanup(t)

	require.NoError(t, s.CreateRequest(ctx, "owner", "address", "code"))

	assert.True(t, errors.Is(s.CreateRequest(ctx, "owner", "address", "code"), storage.ErrAlreadyExists))
}

func TestPg_GetNotConfirmedAccountAddress(t *testing.T) {
	defer cleanup(t)

	require.NoError(t, s.CreateRequest(ctx, "owner", "address", "code"))

	addr, err := s.GetNotConfirmedAccountAddress(ctx, "owner", "code")
	require.NoError(t, err)
	assert.Equal(t, "address", addr)

	_, err = s.GetNotConfirmedAccountAddress(ctx, "owner", "wrong")
	require.Error(t, err)
	assert.True(t, errors.Is(err, storage.ErrNotFound))
}

func TestPg_MarkConfirmed(t *testing.T) {
	defer cleanup(t)

	require.NoError(t, s.CreateRequest(ctx, "owner", "address", "code"))
	require.NoError(t, s.MarkConfirmed(ctx, "owner"))

	_, err := s.GetNotConfirmedAccountAddress(ctx, "owner", "code")
	require.Error(t, err)
	assert.True(t, errors.Is(err, storage.ErrNotFound))

	err = s.MarkConfirmed(ctx, "unknown")
	require.Error(t, err)
	assert.True(t, errors.Is(err, storage.ErrNotFound))
}
