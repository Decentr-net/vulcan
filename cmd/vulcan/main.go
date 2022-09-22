package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	cliflags "github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/go-chi/chi"
	"github.com/golang-migrate/migrate/v4"
	migratep "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jessevdk/go-flags"
	"github.com/johntdyer/slackrus"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/Decentr-net/go-broadcaster"
	"github.com/Decentr-net/logrus/sentry"
	"github.com/Decentr-net/vulcan/internal/blockchain"
	"github.com/Decentr-net/vulcan/internal/health"
	"github.com/Decentr-net/vulcan/internal/mail/gmail"
	"github.com/Decentr-net/vulcan/internal/referral"
	"github.com/Decentr-net/vulcan/internal/server"
	"github.com/Decentr-net/vulcan/internal/service"
	"github.com/Decentr-net/vulcan/internal/storage/postgres"
	"github.com/Decentr-net/vulcan/internal/supply"
)

// nolint:lll,gochecknoglobals
var opts = struct {
	Host            string        `long:"http.host" env:"HTTP_HOST" default:"0.0.0.0" description:"IP to listen on"`
	Port            int           `long:"http.port" env:"HTTP_PORT" default:"8080" description:"port to listen on for insecure connections, defaults to a random value"`
	RequestTimeout  time.Duration `long:"http.request-timeout" env:"HTTP_REQUEST_TIMEOUT" default:"45s" description:"request processing timeout"`
	RecaptchaSecret string        `long:"http.recaptcha_secret" env:"HTTP_RECAPTCHA_SECRET" required:"true" description:"recaptcha secret"`

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

	GmailVerificationEmailSubject string `long:"gmail.verification_email_subject" env:"GMAIL_VERIFICATION_EMAIL_SUBJECT" default:"Decentr - Verification" description:"subject for verification emails"`
	GmailWelcomeEmailSubject      string `long:"gmail.welcome_email_subject" env:"GMAIL_WELCOME_EMAIL_SUBJECT" default:"Decentr - Verified" description:"subject for welcome emails"`
	GmailFromName                 string `long:"gmail.from_name" env:"GMAIL_FROM_NAME" default:"Decentr" description:"name for emails sender"`
	GmailFromEmail                string `long:"gmail.from_email" env:"GMAIL_FROM_EMAIL" default:"no-reply@decentrdev.com" description:"email for emails sender"`
	GmailFromPassword             string `long:"gmail.from_password" env:"GMAIL_FROM_PASSWORD" default:"" description:"password for emails sender"`
	GmailSMTPHost                 string `long:"gmail.smtp_host" env:"GMAIL_SMTP_HOST" default:"smtp.gmail.com" description:"SMTP host"`
	GmailSMTPPort                 int    `long:"gmail.smtp_port" env:"GMAIL_SMTP_PORT" default:"587" description:"SMTP port"`

	BlockchainNode               string `long:"blockchain.node" env:"BLOCKCHAIN_NODE" default:"http://zeus.testnet.decentr.xyz:26657" description:"decentr node address"`
	BlockchainFrom               string `long:"blockchain.from" env:"BLOCKCHAIN_FROM" description:"decentr account name to send stakes" required:"true"`
	BlockchainTxMemo             string `long:"blockchain.tx_memo" env:"BLOCKCHAIN_TX_MEMO" description:"decentr tx's memo'"`
	BlockchainChainID            string `long:"blockchain.chain_id" env:"BLOCKCHAIN_CHAIN_ID" default:"testnet" description:"decentr chain id"`
	BlockchainClientHome         string `long:"blockchain.client_home" env:"BLOCKCHAIN_CLIENT_HOME" default:"~/.decentrcli" description:"decentrcli home directory"`
	BlockchainKeyringBackend     string `long:"blockchain.keyring_backend" env:"BLOCKCHAIN_KEYRING_BACKEND" default:"test" description:"decentrcli keyring backend"`
	BlockchainKeyringPromptInput string `long:"blockchain.keyring_prompt_input" env:"BLOCKCHAIN_KEYRING_PROMPT_INPUT" description:"decentrcli keyring prompt input"`
	BlockchainGas                uint64 `long:"blockchain.gas" env:"BLOCKCHAIN_GAS" default:"1000" description:"gas amount"`
	BlockchainFee                string `long:"blockchain.fee" env:"BLOCKCHAIN_FEE" default:"5000udec" description:"transaction fee"`

	LogLevel  string `long:"log.level" env:"LOG_LEVEL" default:"info" description:"Log level" choice:"debug" choice:"info" choice:"warning" choice:"error"`
	SentryDSN string `long:"sentry.dsn" env:"SENTRY_DSN" description:"sentry dsn"`

	InitialStakes int64 `long:"blockchain.initial_stakes" env:"BLOCKCHAIN_INITIAL_STAKES" default:"1000000" description:"stakes count to be sent"`

	ReferralThresholdPDV  string `long:"referral.threshold_pdv" env:"REFERRAL_THRESHOLD_PDV" default:"0.000100" description:"how many PDV a user should obtain to get a referral reward'"`
	ReferralThresholdDays int    `long:"referral.threshold_days" env:"REFERRAL_THRESHOLD_DAYS" default:"30" description:"how many days a user should wait to get a referral reward'"`

	SupplyNativeNode string `long:"supply.native_node" env:"SUPPLY_NATIVE_NODE" default:"https://zeus.testnet.decentr.xyz" description:"native rest node address"`
	SupplyERC20Node  string `long:"supply.erc20_node" env:"SUPPLY_ERC20_NODE" default:"" description:"erc20 node address"`

	SlackHookURL string `long:"slack.hook-url" env:"SLACK_HOOK_URL" description:"slack hook url"`
	SlackChannel string `long:"slack.channel" env:"SLACK_CHANNEL" default:"alerts-dloan" description:"slack channel"`
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

	if opts.SlackHookURL != "" && opts.SlackChannel != "" {
		logrus.AddHook(&slackrus.SlackrusHook{
			HookURL:        opts.SlackHookURL,
			AcceptedLevels: []logrus.Level{logrus.InfoLevel},
			Channel:        opts.SlackChannel,
			IconEmoji:      ":bread:",
			Username:       "vulcan",
			Filters: []slackrus.Filter{
				func(entry *logrus.Entry) bool {
					return entry.Data["sender"] == "slack"
				},
			},
		})
	}

	r := chi.NewMux()

	db := mustGetDB()

	mailSender := gmail.New(&gmail.Config{
		VerificationSubject: opts.GmailVerificationEmailSubject,
		WelcomeSubject:      opts.GmailWelcomeEmailSubject,
		FromName:            opts.GmailFromName,
		FromEmail:           opts.GmailFromEmail,
		FromPassword:        opts.GmailFromPassword,

		SMTPPort: opts.GmailSMTPPort,
		SMTPHost: opts.GmailSMTPHost,
	})

	nativeNodeConn, err := grpc.Dial(
		opts.SupplyNativeNode,
		grpc.WithInsecure(),
	)
	if err != nil {
		logrus.WithError(err).Fatal("failed to create grpc conn to native node")
	}

	sup := supply.New(banktypes.NewQueryClient(nativeNodeConn), opts.SupplyERC20Node)
	bc := mustGetBroadcaster()

	rc := referral.NewConfig(sdk.MustNewDecFromStr(opts.ReferralThresholdPDV), opts.ReferralThresholdDays)

	server.SetupRouter(
		service.New(
			postgres.New(db),
			mailSender,
			blockchain.New(bc),
			sdk.NewInt(opts.InitialStakes),
			opts.BlockchainTxMemo,
			rc,
			opts.RecaptchaSecret,
		),
		sup,
		r,
		opts.RequestTimeout,
		strings.Contains(opts.BlockchainNode, "testnet"),
	)

	health.SetupRouter(r,
		health.SubjectPinger("postgres", db.PingContext),
		health.SubjectPinger("blockchain", bc.PingContext),
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

func mustGetBroadcaster() broadcaster.Broadcaster {
	fee, err := sdk.ParseCoinNormalized(opts.BlockchainFee)
	if err != nil {
		logrus.WithError(err).Error("failed to parse fee")
	}

	b, err := broadcaster.New(broadcaster.Config{
		KeyringRootDir:     opts.BlockchainClientHome,
		KeyringBackend:     opts.BlockchainKeyringBackend,
		KeyringPromptInput: opts.BlockchainKeyringPromptInput,
		NodeURI:            opts.BlockchainNode,
		BroadcastMode:      cliflags.BroadcastSync,
		From:               opts.BlockchainFrom,
		ChainID:            opts.BlockchainChainID,
		Gas:                opts.BlockchainGas,
		GasAdjust:          1.2,
		Fees:               sdk.Coins{fee},
	})

	if err != nil {
		logrus.WithError(err).Fatal("failed to create main broadcaster")
	}

	return b
}
