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
	"time"

	"github.com/go-chi/chi"
	"github.com/jessevdk/go-flags"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/Decentr-net/vulcan/internal/health"
	"github.com/Decentr-net/vulcan/internal/mail/sendpulse"
	"github.com/Decentr-net/vulcan/internal/server"
	"github.com/Decentr-net/vulcan/internal/service"
	"github.com/Decentr-net/vulcan/internal/storage/postgres"
)

// nolint:lll,gochecknoglobals
var opts = struct {
	Host string `long:"http.host" env:"HTTP_HOST" default:"localhost" description:"IP to listen on"`
	Port int    `long:"http.port" env:"HTTP_PORT" default:"8080" description:"port to listen on for insecure connections, defaults to a random value"`

	Postgres string `long:"postgres" env:"POSTGRES" default:"host=localhost port=5432 user=postgres password=root sslmode=disable" description:"postgres dsn"`

	SendpulseClientID      string        `long:"sendpulse.client_id" env:"SENDPULSE_CLIENT_ID" description:"client_id for sendpulse.com oauth"`
	SendpulseClientSecret  string        `long:"sendpulse.client_secret" env:"SENDPULSE_CLIENT_SECRET" description:"client_secret for sendpulse.com oauth"`
	SendpulseClientTimeout time.Duration `long:"sendpulse.client_timeout" env:"SENDPULSE_CLIENT_TIMEOUT" default:"10s" description:"timeout for sendpulse's' http client"`
	SendpulseEmailSubject  string        `long:"sendpulse.email_subject" env:"SENDPULSE_EMAIL_SUBJECT" default:"decentr.xyz - Verification" description:"subject for emails"`
	SendpulseEmailTemplate uint64        `long:"sendpulse.email_template" env:"SENDPULSE_EMAIL_TEMPLATE" description:"sendpulse's template to be sent"`
	SendpulseFromName      string        `long:"sendpulse.from_name" env:"SENDPULSE_FROM_NAME" default:"decentr.xyz" description:"name for emails sender"`
	SendpulseFromEmail     string        `long:"sendpulse.from_email" env:"SENDPULSE_FROM_NAME" default:"norepty@decentrdev.com" description:"email for emails sender"`

	LogLevel string `long:"log.level" env:"LOG_LEVEL" default:"info" description:"Log level" choice:"debug" choice:"info" choice:"warning" choice:"error"`

	InitialStakes int64 `long:"blockchain.initial_stakes" env:"BLOCKCHAIN_INITIAL_STAKES" default:"1" description:"stakes count to be sent"`
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

	sp, spp := sendpulse.New(opts.SendpulseClientID, opts.SendpulseClientSecret, opts.SendpulseClientTimeout, sendpulse.Config{
		Subject:    opts.SendpulseEmailSubject,
		TemplateID: opts.SendpulseEmailTemplate,
		FromName:   opts.SendpulseFromName,
		FromEmail:  opts.SendpulseFromEmail,
	})

	server.SetupRouter(service.New(postgres.New(db), sp, nil, opts.InitialStakes), r)
	health.SetupRouter(r,
		health.SubjectPinger("postgres", db.PingContext),
		spp,
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
