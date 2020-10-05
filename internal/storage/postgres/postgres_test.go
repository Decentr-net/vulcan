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
	"time"

	m "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/lib/pq"
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

func TestPg_SetRequest(t *testing.T) {
	defer cleanup(t)

	r := &storage.Request{
		Owner:       "owner",
		Email:       "e@mail.com",
		Address:     "address",
		Code:        "code",
		CreatedAt:   time.Now().UTC().Truncate(time.Second),
		ConfirmedAt: pq.NullTime{},
	}

	require.NoError(t, s.SetRequest(ctx, r))
	_, err := s.GetRequest(ctx, "owner", "")
	require.NoError(t, err)

	r.ConfirmedAt = pq.NullTime{
		Time:  time.Now().UTC().Truncate(time.Second),
		Valid: true,
	}

	require.NoError(t, s.SetRequest(ctx, r))

	res, err := s.GetRequest(ctx, "", "address")
	require.NoError(t, err)
	equalRequest(t, r, res)

	// invalid by db design but not covered by query
	r.Email = "new"
	err = s.SetRequest(ctx, r)
	require.True(t, errors.Is(err, storage.ErrAddressIsTaken))
}

func TestPg_GetRequest(t *testing.T) {
	defer cleanup(t)

	r := &storage.Request{
		Owner:       "owner",
		Email:       "e@mail.com",
		Address:     "address",
		Code:        "code",
		CreatedAt:   time.Now().UTC().Truncate(time.Second),
		ConfirmedAt: pq.NullTime{},
	}

	require.NoError(t, s.SetRequest(ctx, r))

	res, err := s.GetRequest(ctx, "owner", "")
	require.NoError(t, err)
	equalRequest(t, r, res)

	res, err = s.GetRequest(ctx, "", "address")
	require.NoError(t, err)
	equalRequest(t, r, res)

	_, err = s.GetRequest(ctx, "fsd", "rew")
	require.Error(t, err)
	assert.True(t, errors.Is(err, storage.ErrNotFound))
}

func equalRequest(t *testing.T, expected, actual *storage.Request) {
	assert.Equal(t, expected.Owner, actual.Owner)
	assert.Equal(t, expected.Email, actual.Email)
	assert.Equal(t, expected.Address, actual.Address)
	assert.Equal(t, expected.Code, actual.Code)
	assert.Equal(t, expected.CreatedAt.Unix(), actual.CreatedAt.Unix())
	assert.Equal(t, expected.ConfirmedAt.Time.Unix(), actual.ConfirmedAt.Time.Unix())
}
