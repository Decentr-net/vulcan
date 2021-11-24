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
	"github.com/go-chi/chi"
	"github.com/golang-migrate/migrate/v4"
	migratep "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jessevdk/go-flags"
	mc "github.com/keighl/mandrill"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/Decentr-net/decentr/app"
	"github.com/Decentr-net/go-broadcaster"
	"github.com/Decentr-net/logrus/sentry"

	"github.com/Decentr-net/vulcan/internal/blockchain"
	"github.com/Decentr-net/vulcan/internal/health"
	"github.com/Decentr-net/vulcan/internal/mail/mandrill"
	"github.com/Decentr-net/vulcan/internal/referral"
	"github.com/Decentr-net/vulcan/internal/server"
	"github.com/Decentr-net/vulcan/internal/service"
	"github.com/Decentr-net/vulcan/internal/storage/postgres"
	"github.com/Decentr-net/vulcan/internal/supply"
)

// nolint:lll,gochecknoglobals
var opts = struct {
	Host           string        `long:"http.host" env:"HTTP_HOST" default:"0.0.0.0" description:"IP to listen on"`
	Port           int           `long:"http.port" env:"HTTP_PORT" default:"8080" description:"port to listen on for insecure connections, defaults to a random value"`
	RequestTimeout time.Duration `long:"http.request-timeout" env:"HTTP_REQUEST_TIMEOUT" default:"45s" description:"request processing timeout"`

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

	BlockchainTestNode               string `long:"blockchain.test.node" env:"BLOCKCHAIN_TEST_NODE" default:"http://zeus.mainnet.decentr.xyz:26657" description:"decentr node address"`
	BlockchainTestFrom               string `long:"blockchain.test.from" env:"BLOCKCHAIN_TEST_FROM" description:"decentr account name to send stakes" required:"true"`
	BlockchainTestTxMemo             string `long:"blockchain.test.tx_memo" env:"BLOCKCHAIN_TEST_TX_MEMO" description:"decentr tx's memo'"`
	BlockchainTestChainID            string `long:"blockchain.test.chain_id" env:"BLOCKCHAIN_TEST_CHAIN_ID" default:"testnet" description:"decentr chain id"`
	BlockchainTestClientHome         string `long:"blockchain.test.client_home" env:"BLOCKCHAIN_TEST_CLIENT_HOME" default:"~/.decentrcli" description:"decentrcli home directory"`
	BlockchainTestKeyringBackend     string `long:"blockchain.test.keyring_backend" env:"BLOCKCHAIN_TEST_KEYRING_BACKEND" default:"test" description:"decentrcli keyring backend"`
	BlockchainTestKeyringPromptInput string `long:"blockchain.test.keyring_prompt_input" env:"BLOCKCHAIN_TEST_KEYRING_PROMPT_INPUT" description:"decentrcli keyring prompt input"`
	BlockchainTestGas                uint64 `long:"blockchain.test.gas" env:"BLOCKCHAIN_TEST_GAS" default:"10" description:"gas amount"`
	BlockchainTestFee                string `long:"blockchain.test.fee" env:"BLOCKCHAIN_TEST_FEE" default:"1udec" description:"transaction fee"`

	BlockchainMainNode               string `long:"blockchain.main.node" env:"BLOCKCHAIN_MAIN_NODE" default:"http://zeus.testnet.decentr.xyz:26657" description:"decentr node address"`
	BlockchainMainFrom               string `long:"blockchain.main.from" env:"BLOCKCHAIN_MAIN_FROM" description:"decentr account name to send stakes" required:"true"`
	BlockchainMainTxMemo             string `long:"blockchain.main.tx_memo" env:"BLOCKCHAIN_MAIN_TX_MEMO" description:"decentr tx's memo'"`
	BlockchainMainChainID            string `long:"blockchain.main.chain_id" env:"BLOCKCHAIN_MAIN_CHAIN_ID" default:"testnet" description:"decentr chain id"`
	BlockchainMainClientHome         string `long:"blockchain.main.client_home" env:"BLOCKCHAIN_MAIN_CLIENT_HOME" default:"~/.decentrcli" description:"decentrcli home directory"`
	BlockchainMainKeyringBackend     string `long:"blockchain.main.keyring_backend" env:"BLOCKCHAIN_MAIN_KEYRING_BACKEND" default:"test" description:"decentrcli keyring backend"`
	BlockchainMainKeyringPromptInput string `long:"blockchain.main.keyring_prompt_input" env:"BLOCKCHAIN_MAIN_KEYRING_PROMPT_INPUT" description:"decentrcli keyring prompt input"`
	BlockchainMainGas                uint64 `long:"blockchain.main.gas" env:"BLOCKCHAIN_MAIN_GAS" default:"10" description:"gas amount"`
	BlockchainMainFee                string `long:"blockchain.main.fee" env:"BLOCKCHAIN_MAIN_FEE" default:"1udec" description:"transaction fee"`

	LogLevel  string `long:"log.level" env:"LOG_LEVEL" default:"info" description:"Log level" choice:"debug" choice:"info" choice:"warning" choice:"error"`
	SentryDSN string `long:"sentry.dsn" env:"SENTRY_DSN" description:"sentry dsn"`

	InitialTestStakes int64 `long:"blockchain.test.initial_stakes" env:"BLOCKCHAIN_TEST_INITIAL_STAKES" default:"1000000" description:"stakes count to be sent"`
	InitialMainStakes int64 `long:"blockchain.main.initial_stakes" env:"BLOCKCHAIN_MAIN_INITIAL_STAKES" default:"1000000" description:"stakes count to be sent"`

	ReferralSenderReward   int `long:"referral.sender_reward" env:"REFERRAL_SENDER_REWARD" default:"10" description:"referral sender reward uDEC'"`
	ReferralReceiverReward int `long:"referral.receiver_reward" env:"REFERRAL_RECEIVER_REWARD" default:"10"  description:"referral receiver reward uDEC'"`
	ReferralThresholdUPDV  int `long:"referral.threshold_updv" env:"REFERRAL_THRESHOLD_UPDV" default:"100" description:"how many uPDV a user should obtain to get a referral reward'"`
	ReferralThresholdDays  int `long:"referral.threshold_days" env:"REFERRAL_THRESHOLD_DAYS" default:"30" description:"how many days a user should wait to get a referral reward'"`

	SupplyNativeNode string `long:"supply.native_node" env:"SUPPLY_NATIVE_NODE" default:"https://zeus.testnet.decentr.xyz" description:"native rest node address"`
	SupplyERC20Node  string `long:"supply.erc20_node" env:"SUPPLY_ERC20_NODE" default:"" description:"erc20 node address"`
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

	sup := supply.New(opts.SupplyNativeNode, opts.SupplyERC20Node)
	bt := mustGetTestBroadcaster()
	bm := mustGetMainBroadcaster()

	rc := referral.Config{
		SenderReward:   opts.ReferralSenderReward,
		ReceiverReward: opts.ReferralReceiverReward,
		ThresholdUPDV:  opts.ReferralThresholdUPDV,
		ThresholdDays:  opts.ReferralThresholdDays,
	}

	server.SetupRouter(
		service.New(
			postgres.New(db),
			mailSender,
			blockchain.New(bt, opts.BlockchainTestTxMemo),
			blockchain.New(bm, opts.BlockchainMainTxMemo),
			opts.InitialTestStakes,
			opts.InitialMainStakes,
			rc,
		),
		sup,
		r,
		opts.RequestTimeout,
	)
	health.SetupRouter(r,
		health.SubjectPinger("postgres", db.PingContext),
		health.SubjectPinger("mandrill", func(_ context.Context) error {
			_, err := mandrillClient.Ping()
			return err
		}),
		health.SubjectPinger("blockchain_testnet", bt.PingContext),
		health.SubjectPinger("blockchain_mainnet", bm.PingContext),
		health.SubjectPinger("supply", sup.PingContext),
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

func mustGetTestBroadcaster() *broadcaster.Broadcaster {
	fee, err := sdk.ParseCoin(opts.BlockchainTestFee)
	if err != nil {
		logrus.WithError(err).Error("failed to parse fee")
	}

	b, err := broadcaster.New(app.MakeCodec(), broadcaster.Config{
		CLIHome:            opts.BlockchainTestClientHome,
		KeyringBackend:     opts.BlockchainTestKeyringBackend,
		KeyringPromptInput: opts.BlockchainTestKeyringPromptInput,
		NodeURI:            opts.BlockchainTestNode,
		BroadcastMode:      cliflags.BroadcastSync,
		From:               opts.BlockchainTestFrom,
		ChainID:            opts.BlockchainTestChainID,
		GenesisKeyPass:     keys.DefaultKeyPass,
		Gas:                opts.BlockchainTestGas,
		GasAdjust:          1.2,
		Fees:               sdk.Coins{fee},
	})

	if err != nil {
		logrus.WithError(err).Fatal("failed to create test broadcaster")
	}

	return b
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
