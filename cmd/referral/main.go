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

	cliflags "github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/keys"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang-migrate/migrate/v4"
	migratep "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jessevdk/go-flags"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/Decentr-net/decentr/app"
	"github.com/Decentr-net/go-broadcaster"
	"github.com/Decentr-net/logrus/sentry"
	"github.com/Decentr-net/vulcan/internal/blockchain"
	"github.com/Decentr-net/vulcan/internal/blockchain/rest"
	"github.com/Decentr-net/vulcan/internal/health"
	"github.com/Decentr-net/vulcan/internal/referral"
	"github.com/Decentr-net/vulcan/internal/storage/postgres"
)

// nolint:lll,gochecknoglobals
var opts = struct {
	Postgres                   string `long:"postgres" env:"POSTGRES" default:"host=localhost port=5432 user=postgres password=root sslmode=disable" description:"postgres dsn"`
	PostgresMaxOpenConnections int    `long:"postgres.max_open_connections" env:"POSTGRES_MAX_OPEN_CONNECTIONS" default:"0" description:"postgres maximal open connections count, 0 means unlimited"`
	PostgresMaxIdleConnections int    `long:"postgres.max_idle_connections" env:"POSTGRES_MAX_IDLE_CONNECTIONS" default:"5" description:"postgres maximal idle connections count"`
	PostgresMigrations         string `long:"postgres.migrations" env:"POSTGRES_MIGRATIONS" default:"migrations/postgres" description:"postgres migrations directory"`

	BlockchainMainNode               string `long:"blockchain.main.node" env:"BLOCKCHAIN_MAIN_NODE" default:"http://zeus.testnet.decentr.xyz:26657" description:"decentr node address"`
	BlockchainMainFrom               string `long:"blockchain.main.from" env:"BLOCKCHAIN_MAIN_FROM" description:"decentr account name to send stakes" required:"true"`
	BlockchainMainTxMemo             string `long:"blockchain.main.tx_memo" env:"BLOCKCHAIN_MAIN_TX_MEMO" description:"decentr tx's memo'"`
	BlockchainMainChainID            string `long:"blockchain.main.chain_id" env:"BLOCKCHAIN_MAIN_CHAIN_ID" default:"testnet" description:"decentr chain id"`
	BlockchainMainClientHome         string `long:"blockchain.main.client_home" env:"BLOCKCHAIN_MAIN_CLIENT_HOME" default:"~/.decentrcli" description:"decentrcli home directory"`
	BlockchainMainKeyringBackend     string `long:"blockchain.main.keyring_backend" env:"BLOCKCHAIN_MAIN_KEYRING_BACKEND" default:"test" description:"decentrcli keyring backend"`
	BlockchainMainKeyringPromptInput string `long:"blockchain.main.keyring_prompt_input" env:"BLOCKCHAIN_MAIN_KEYRING_PROMPT_INPUT" description:"decentrcli keyring prompt input"`
	BlockchainMainGas                uint64 `long:"blockchain.main.gas" env:"BLOCKCHAIN_MAIN_GAS" default:"10" description:"gas amount"`
	BlockchainMainFee                string `long:"blockchain.main.fee" env:"BLOCKCHAIN_MAIN_FEE" default:"1udec" description:"transaction fee"`
	BlockchainMainRESTNodeURL        string `long:"blockchain.main.rest_node_url" env:"BLOCKCHAIN_MAIN_REST_NODE_URL" default:"http://hera.mainnet.decentr.xyz" description:"REST endpoint URL"`

	ReferralThresholdUPDV int `long:"referral.threshold_updv" env:"REFERRAL_THRESHOLD_UPDV" default:"100" description:"how many uPDV a user should obtain to get a referral reward'"`
	ReferralThresholdDays int `long:"referral.threshold_days" env:"REFERRAL_THRESHOLD_DAYS" default:"30" description:"how many days a user should wait to get a referral reward'"`

	LogLevel  string `long:"log.level" env:"LOG_LEVEL" default:"info" description:"Log level" choice:"debug" choice:"info" choice:"warning" choice:"error"`
	SentryDSN string `long:"sentry.dsn" env:"SENTRY_DSN" description:"sentry dsn"`
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
		logrus.WithError(err).Fatal("error occurred while parsing flags")
	}

	lvl, _ := logrus.ParseLevel(opts.LogLevel) // err will always be nil
	logrus.SetLevel(lvl)

	logrus.Info("service started")
	logrus.Infof("%+v", opts)

	if opts.SentryDSN != "" {
		hook, err := sentry.NewHook(sentry.Options{
			Dsn:              opts.SentryDSN,
			AttachStacktrace: true,
			Release:          health.GetVersion(),
		}, logrus.PanicLevel, logrus.FatalLevel, logrus.ErrorLevel)

		if err != nil {
			logrus.WithError(err).Fatal("failed to init sentry")
		}

		logrus.AddHook(hook)
	} else {
		logrus.Info("empty sentry dsn")
		logrus.Warn("skip sentry initialization")
	}

	db := mustGetDB()
	bm := mustGetMainBroadcaster()

	ctx, cancel := context.WithCancel(context.Background())

	gr, _ := errgroup.WithContext(context.Background())
	gr.Go(func() error {
		b := blockchain.New(bm)
		brc := rest.NewBlockchainRESTClient(opts.BlockchainMainRESTNodeURL)
		referral.NewRewarder(postgres.New(db), b, brc,
			referral.NewConfig(opts.ReferralThresholdUPDV, opts.ReferralThresholdDays)).Run(ctx, time.Hour)
		return nil
	})

	gr.Go(func() error {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

		s := <-sigs

		logrus.Infof("terminating by %s signal", s)

		cancel()

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

func mustGetMainBroadcaster() *broadcaster.Broadcaster {
	fee, err := sdk.ParseCoin(opts.BlockchainMainFee)
	if err != nil {
		logrus.WithError(err).Error("failed to parse fee")
	}

	b, err := broadcaster.New(app.MakeCodec(), broadcaster.Config{
		CLIHome:            opts.BlockchainMainClientHome,
		KeyringBackend:     opts.BlockchainMainKeyringBackend,
		KeyringPromptInput: opts.BlockchainMainKeyringPromptInput,
		NodeURI:            opts.BlockchainMainNode,
		BroadcastMode:      cliflags.BroadcastSync,
		From:               opts.BlockchainMainFrom,
		ChainID:            opts.BlockchainMainChainID,
		GenesisKeyPass:     keys.DefaultKeyPass,
		Gas:                opts.BlockchainMainGas,
		GasAdjust:          1.2,
		Fees:               sdk.Coins{fee},
	})

	if err != nil {
		logrus.WithError(err).Fatal("failed to create main broadcaster")
	}

	return b
}
