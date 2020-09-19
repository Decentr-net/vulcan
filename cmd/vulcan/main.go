package main

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path"
	"syscall"

	clicontext "github.com/cosmos/cosmos-sdk/client/context"
	cliflags "github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/client/utils"
	"github.com/go-chi/chi"
	"github.com/jessevdk/go-flags"
	mc "github.com/keighl/mandrill"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/tendermint/tendermint/libs/cli"
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

	Postgres string `long:"postgres" env:"POSTGRES" default:"host=localhost port=5432 user=postgres password=root sslmode=disable" description:"postgres dsn"`

	MandrillAPIKey            string `long:"mandrill.api_key" env:"MANDRILL_API_KEY" description:"mandrillapp.com api key"`
	MandrillEmailSubject      string `long:"mandrill.email_subject" env:"MANDRILL_API_KEY_EMAIL_SUBJECT" default:"decentr.xyz - Verification" description:"subject for emails"`
	MandrillEmailTemplateName string `long:"mandrill.email_template_id" env:"MANDRILL_API_KEY_EMAIL_TEMPLATE_ID" description:"sendpulse's template to be sent"`
	MandrillFromName          string `long:"mandrill.from_name" env:"MANDRILL_API_KEY_FROM_NAME" default:"decentr.xyz" description:"name for emails sender"`
	MandrillFromEmail         string `long:"mandrill.from_email" env:"MANDRILL_API_KEY_FROM_NAME" default:"noreply@decentrdev.com" description:"email for emails sender"`

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

	mandrillClient := mc.ClientWithKey(opts.MandrillAPIKey)

	mailSender := mandrill.New(mandrillClient, mandrill.Config{
		Subject:      opts.MandrillEmailSubject,
		TemplateName: opts.MandrillEmailTemplateName,
		FromEmail:    opts.MandrillFromEmail,
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

func mustGetBlockchain() blockchain.Blockchain {
	cdc := app.MakeCodec()

	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(app.Bech32PrefixAccAddr, app.Bech32PrefixAccPub)
	config.Seal()

	cfgFile := path.Join(viper.GetString(cli.HomeFlag), "config", "config.toml")
	if _, err := os.Stat(cfgFile); err == nil {
		viper.SetConfigFile(cfgFile)

		if err := viper.ReadInConfig(); err != nil {
			logrus.WithError(err).Fatal("failed to read config")
		}
	}

	in := bufio.NewReader(os.Stdin)
	cliCtx := clicontext.NewCLIContextWithInputAndFrom(in, viper.GetString(cliflags.FlagFrom)).
		WithCodec(cdc).WithBroadcastMode(cliflags.BroadcastSync)
	txBldr := auth.NewTxBuilderFromCLI(in).WithTxEncoder(utils.GetTxEncoder(cdc))

	return blockchain.NewBlockchain(cliCtx, txBldr)
}
