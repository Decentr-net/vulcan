package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	clicontext "github.com/cosmos/cosmos-sdk/client/context"
	cliflags "github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/crypto/keys"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/client/utils"
	"github.com/go-chi/chi"
	migrate "github.com/golang-migrate/migrate/v4"
	migratep "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jessevdk/go-flags"
	mc "github.com/keighl/mandrill"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/Decentr-net/decentr/app"

	"github.com/Decentr-net/vulcan/internal/blockchain"
	"github.com/Decentr-net/vulcan/internal/health"
	"github.com/Decentr-net/vulcan/internal/mail/mandrill"
	"github.com/Decentr-net/vulcan/internal/server"
	"github.com/Decentr-net/vulcan/internal/service"
	"github.com/Decentr-net/vulcan/internal/storage/postgres"
)

// nolint:lll,gochecknoglobals
var opts = struct {
	Host string `long:"http.host" env:"HTTP_HOST" default:"localhost" description:"IP to listen on"`
	Port int    `long:"http.port" env:"HTTP_PORT" default:"8080" description:"port to listen on for insecure connections, defaults to a random value"`

	Postgres                   string `long:"postgres" env:"POSTGRES" default:"host=localhost port=5432 user=postgres password=root sslmode=disable" description:"postgres dsn"`
	PostgresMaxOpenConnections int    `long:"postgres.max_open_connections" env:"POSTGRES_MAX_OPEN_CONNECTIONS" default:"0" description:"postgres maximal open connections count, 0 means unlimited"`
	PostgresMaxIdleConnections int    `long:"postgres.max_idle_connections" env:"POSTGRES_MAX_IDLE_CONNECTIONS" default:"5" description:"postgres maximal idle connections count"`
	PostgresMigrations         string `long:"postgres.migrations" env:"POSTGRES_MIGRATIONS" default:"migrations/postgres" description:"postgres migrations directory"`

	MandrillAPIKey                        string `long:"mandrill.api_key" env:"MANDRILL_API_KEY" description:"mandrillapp.com api key" required:"true"`
	MandrillVerificationEmailSubject      string `long:"mandrill.verification_email_subject" env:"MANDRILL_VERIFICATION_EMAIL_SUBJECT" default:"decentr.xyz - Verification" description:"subject for verification emails"`
	MandrillVerificationEmailTemplateName string `long:"mandrill.verification_email_template_name" env:"MANDRILL_VERIFICATION_EMAIL_TEMPLATE_NAME" description:"mandrill's verification template to be sent" required:"true"`
	MandrillWelcomeEmailSubject           string `long:"mandrill.welcome_email_subject" env:"MANDRILL_WELCOME_EMAIL_SUBJECT" default:"decentr.xyz - Verified" description:"subject for welcome emails"`
	MandrillWelcomeEmailTemplateName      string `long:"mandrill.welcome_email_template_name" env:"MANDRILL_WELCOME_EMAIL_TEMPLATE_NAME" description:"mandrill's welcome template to be sent" required:"true"`
	MandrillFromName                      string `long:"mandrill.from_name" env:"MANDRILL_FROM_NAME" default:"decentr.xyz" description:"name for emails sender"`
	MandrillFromEmail                     string `long:"mandrill.from_email" env:"MANDRILL_FROM_EMAIL" default:"noreply@decentrdev.com" description:"email for emails sender"`

	BlockchainNode               string `long:"blockchain.node" env:"BLOCKCHAIN_NODE" default:"zeus.testnet.decentr.xyz:26656" description:"decentr node address"`
	BlockchainFrom               string `long:"blockchain.from" env:"BLOCKCHAIN_FROM" description:"decentr account name to send stakes" required:"true"`
	BlockchainTxMemo             string `long:"blockchain.tx_memo" env:"BLOCKCHAIN_TX_MEMO" description:"decentr tx's memo'"`
	BlockchainChainID            string `long:"blockchain.chain_id" env:"BLOCKCHAIN_CHAIN_ID" default:"testnet" description:"decentr chain id"`
	BlockchainClientHome         string `long:"blockchain.client_home" env:"BLOCKCHAIN_CLIENT_HOME" default:"~/.decentrcli" description:"decentrcli home directory"`
	BlockchainKeyringBackend     string `long:"blockchain.keyring_backend" env:"BLOCKCHAIN_KEYRING_BACKEND" default:"test" description:"decentrcli keyring backend"`
	BlockchainKeyringPromptInput string `long:"blockchain.keyring_prompt_input" env:"BLOCKCHAIN_KEYRING_PROMPT_INPUT" description:"decentrcli keyring prompt input"`

	LogLevel string `long:"log.level" env:"LOG_LEVEL" default:"info" description:"Log level" choice:"debug" choice:"info" choice:"warning" choice:"error"`

	InitialStakes int64 `long:"blockchain.initial_stakes" env:"BLOCKCHAIN_INITIAL_STAKES" default:"1000000" description:"stakes count to be sent"`
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

	db := mustGetDB()

	mandrillClient := mc.ClientWithKey(opts.MandrillAPIKey)

	mailSender := mandrill.New(mandrillClient, &mandrill.Config{
		VerificationSubject:      opts.MandrillVerificationEmailSubject,
		VerificationTemplateName: opts.MandrillVerificationEmailTemplateName,
		WelcomeSubject:           opts.MandrillWelcomeEmailSubject,
		WelcomeTemplateName:      opts.MandrillWelcomeEmailTemplateName,
		FromEmail:                opts.MandrillFromEmail,
	})

	server.SetupRouter(service.New(postgres.New(db), mailSender, mustGetBlockchain(), opts.InitialStakes), r)
	health.SetupRouter(r,
		health.SubjectPinger("postgres", db.PingContext),
		health.SubjectPinger("mandrill", func(_ context.Context) error {
			_, err := mandrillClient.Ping()
			return err
		}),
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

func mustGetDB() *sql.DB {
	db, err := sql.Open("postgres", opts.Postgres)
	if err != nil {
		logrus.WithError(err).Fatal("failed to create postgres connection")
	}
	db.SetMaxOpenConns(opts.PostgresMaxOpenConnections)
	db.SetMaxIdleConns(opts.PostgresMaxIdleConnections)

	if err := db.PingContext(context.Background()); err != nil {
		logrus.WithError(err).Fatal("failed to ping postgres")
	}

	driver, err := migratep.WithInstance(db, &migratep.Config{})
	if err != nil {
		logrus.WithError(err).Fatal("failed to create database migrate driver")
	}

	migrator, err := migrate.NewWithDatabaseInstance(fmt.Sprintf("file://%s", opts.PostgresMigrations), "postgres", driver)
	if err != nil {
		logrus.WithError(err).Fatal("failed to create migrator")
	}

	switch v, d, err := migrator.Version(); err {
	case nil:
		logrus.Infof("database version %d with dirty state %t", v, d)
	case migrate.ErrNilVersion:
		logrus.Info("database version: nil")
	default:
		logrus.WithError(err).Fatal("failed to get version")
	}

	switch err := migrator.Up(); err {
	case nil:
		logrus.Info("database was migrated")
	case migrate.ErrNoChange:
		logrus.Info("database is up-to-date")
	default:
		logrus.WithError(err).Fatal("failed to migrate db")
	}

	return db
}

func mustGetBlockchain() blockchain.Blockchain {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(app.Bech32PrefixAccAddr, app.Bech32PrefixAccPub)
	config.Seal()

	cdc := app.MakeCodec()

	kb, err := keys.NewKeyring(sdk.KeyringServiceName(),
		opts.BlockchainKeyringBackend,
		opts.BlockchainClientHome,
		bytes.NewBufferString(opts.BlockchainKeyringPromptInput),
	)
	if err != nil {
		logrus.WithError(err).Fatal("failed to create keyring")
	}

	acc, err := kb.Get(opts.BlockchainFrom)
	if err != nil {
		logrus.WithError(err).Fatal("failed to get blockchain account info")
	}

	cliCtx := clicontext.NewCLIContext().
		WithCodec(cdc).
		WithBroadcastMode(cliflags.BroadcastBlock).
		WithNodeURI(opts.BlockchainNode).
		WithFrom(acc.GetName()).
		WithFromName(acc.GetName()).
		WithFromAddress(acc.GetAddress()).
		WithChainID(opts.BlockchainChainID)
	cliCtx.Keybase = kb

	txBldr := auth.NewTxBuilder(utils.GetTxEncoder(cdc), 0, 0, 0, 1.0, false,
		opts.BlockchainChainID, opts.BlockchainTxMemo, nil, nil).WithKeybase(kb)

	return blockchain.NewBlockchain(cliCtx, txBldr)
}
