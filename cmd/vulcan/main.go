package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi"
	"github.com/jessevdk/go-flags"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/Decentr-net/vulcan/internal/health"
	"github.com/Decentr-net/vulcan/internal/server"
	"github.com/Decentr-net/vulcan/internal/service"
	"github.com/Decentr-net/vulcan/internal/storage/postgres"
)

// nolint:lll,gochecknoglobals
var opts = struct {
	Host string `long:"http.host" env:"HTTP_HOST" default:"localhost" description:"IP to listen on"`
	Port int    `long:"http.port" env:"HTTP_PORT" default:"8080" description:"port to listen on for insecure connections, defaults to a random value"`

	Postgres string `long:"postgres" env:"POSTGRES" default:"host=localhost port=5432 user=postgres password=root sslmode=disable" description:"postgres dsn"`

	LogLevel string `long:"log.level" env:"LOG_LEVEL" default:"info" description:"Log level" choice:"debug" choice:"info" choice:"warning" choice:"error"`

	InitialStakes int64 `long:"blockchain.initial_stakes" env:"BLOCKCHAIN_INITIAL_STAKES" default:"10000" description:"initial stakes for a new wallet. A denominator is 1000"`
}{}

var errTerminated = errors.New("terminated")

func main() {
	parser := flags.NewParser(&opts, flags.Default)
	parser.ShortDescription = "Vulcan"
	parser.LongDescription = "Vulcan"

	_, err := parser.Parse()

	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			parser.WriteHelp(os.Stdout)
			os.Exit(0)
		}
		logrus.WithError(err).Warn("error occurred while parsing flags")
	}

	lvl, _ := logrus.ParseLevel(opts.LogLevel) // err will always be nil
	logrus.SetLevel(lvl)

	logrus.Info("service started")
	logrus.Infof("%+v", opts)

	r := chi.NewMux()

	db, err := sql.Open("postgres", opts.Postgres)
	if err != nil {
		logrus.WithError(err).Fatal("failed to create postgres connection")
	}
	if err := db.PingContext(context.Background()); err != nil {
		logrus.WithError(err).Fatal("failed to ping postgres")
	}

	server.SetupRouter(service.New(postgres.New(db), nil, nil, opts.InitialStakes), r)
	health.SetupRouter(r,
		health.SubjectPinger("postgres", db.PingContext),
	)

	srv := http.Server{
		Addr:    fmt.Sprintf("%s:%d", opts.Host, opts.Port),
		Handler: r,
	}

	gr, _ := errgroup.WithContext(context.Background())
	gr.Go(srv.ListenAndServe)

	gr.Go(func() error {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

		s := <-sigs

		logrus.Infof("terminating by %s signal", s)

		if err := srv.Shutdown(context.Background()); err != nil {
			logrus.WithError(err).Error("failed to gracefully shutdown server")
		}

		return errTerminated
	})

	logrus.Info("service started")

	if err := gr.Wait(); err != nil && !errors.Is(err, errTerminated) && !errors.Is(err, http.ErrServerClosed) {
		logrus.WithError(err).Fatal("service unexpectedly closed")
	}
}
