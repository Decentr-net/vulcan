package main

import (
	"context"
	"database/sql"
	"os"

	cliflags "github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/jessevdk/go-flags"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"

	"github.com/Decentr-net/decentr/config"
	"github.com/Decentr-net/go-broadcaster"
)

func init() {
	config.SetAddressPrefixes()
}

var opts = struct {
	Postgres string `long:"postgres" env:"POSTGRES" default:"host=localhost port=5432 user=postgres password=root sslmode=disable" description:"postgres dsn"`

	SkipCount int `long:"skip-count" env:"SKIP_COUNT" `

	BlockchainMainNode               string `long:"blockchain.main.node" env:"BLOCKCHAIN_MAIN_NODE" default:"http://zeus.mainnet.decentr.xyz:26657" description:"decentr node address"`
	BlockchainMainFrom               string `long:"blockchain.main.from" env:"BLOCKCHAIN_MAIN_FROM" description:"decentr account name to send stakes" required:"true"`
	BlockchainMainTxMemo             string `long:"blockchain.main.tx_memo" env:"BLOCKCHAIN_MAIN_TX_MEMO" description:"decentr tx's memo'"`
	BlockchainMainChainID            string `long:"blockchain.main.chain_id" env:"BLOCKCHAIN_MAIN_CHAIN_ID" default:"mainnet-1" description:"decentr chain id"`
	BlockchainMainClientHome         string `long:"blockchain.main.client_home" env:"BLOCKCHAIN_MAIN_CLIENT_HOME" default:"~/.decentrcli" description:"decentrcli home directory"`
	BlockchainMainKeyringBackend     string `long:"blockchain.main.keyring_backend" env:"BLOCKCHAIN_MAIN_KEYRING_BACKEND" default:"test" description:"decentrcli keyring backend"`
	BlockchainMainKeyringPromptInput string `long:"blockchain.main.keyring_prompt_input" env:"BLOCKCHAIN_MAIN_KEYRING_PROMPT_INPUT" description:"decentrcli keyring prompt input"`
	BlockchainMainGas                uint64 `long:"blockchain.main.gas" env:"BLOCKCHAIN_MAIN_GAS" default:"45575udec" description:"gas amount"`
	BlockchainMainFee                string `long:"blockchain.main.fee" env:"BLOCKCHAIN_MAIN_FEE" default:"1822983udec" description:"transaction fee"`
	InitialMainStakes                int64  `long:"blockchain.main.initial_stakes" env:"BLOCKCHAIN_MAIN_INITIAL_STAKES" default:"10000000" description:"stakes count to be sent"`
}{}

func main() {
	parser := flags.NewParser(&opts, flags.Default)

	_, err := parser.Parse()
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			parser.WriteHelp(os.Stdout)
			os.Exit(0)
		}
		logrus.WithError(err).Fatal("error occurred while parsing flags")
	}

	db, err := sql.Open("postgres", opts.Postgres)
	if err != nil {
		logrus.WithError(err).Fatal("failed to create postgres connection")
	}

	if err := db.PingContext(context.Background()); err != nil {
		logrus.WithError(err).Fatal("failed to ping postgres")
	}

	bt := mustGetMainBroadcaster()

	aa, err := getAddresses(sqlx.NewDb(db, "postgres"))
	if err != nil {
		logrus.WithError(err).Fatal("failed to get addresses")
	}

	logrus.WithField("addresses_count", len(aa)).Info("got addresses")

	var msgs []sdk.Msg
	for i, v := range aa {
		if i < opts.SkipCount {
			continue
		}

		to, err := sdk.AccAddressFromBech32(v)
		if err != nil {
			logrus.WithError(err).Errorf("failed to parse: %s", v)
		}

		msgs = append(msgs, banktypes.NewMsgSend(bt.From(), to, sdk.Coins{sdk.Coin{
			Denom:  config.DefaultBondDenom,
			Amount: sdk.NewInt(opts.InitialMainStakes),
		}}))
	}

	logrus.WithField("msgs_count", len(msgs)).Info("to be processed")

	i := 0
	batch := 10
	for len(msgs) > 0 {
		if batch > len(msgs) {
			batch = len(msgs)
		}

		if _, err := bt.Broadcast(msgs[:batch], ""); err != nil {
			logrus.WithError(err).Fatal("failed to broadcast")
		}

		i += batch
		logrus.WithField("processed", i).Info("batch processed")

		msgs = msgs[batch:]
	}

	logrus.Info("done")
}

func getAddresses(db *sqlx.DB) ([]string, error) {
	var aa []string
	if err := db.Select(&aa, `
		SELECT DISTINCT address FROM request WHERE confirmed_at IS NOT NULL ORDER BY address
	`); err != nil {
		return nil, err
	}

	return aa, nil
}

func mustGetMainBroadcaster() broadcaster.Broadcaster {
	fee, err := sdk.ParseCoinNormalized(opts.BlockchainMainFee)
	if err != nil {
		logrus.WithError(err).Error("failed to parse fee")
	}

	b, err := broadcaster.New(broadcaster.Config{
		KeyringRootDir:     opts.BlockchainMainClientHome,
		KeyringBackend:     opts.BlockchainMainKeyringBackend,
		KeyringPromptInput: opts.BlockchainMainKeyringPromptInput,
		NodeURI:            opts.BlockchainMainNode,
		BroadcastMode:      cliflags.BroadcastBlock,
		From:               opts.BlockchainMainFrom,
		ChainID:            opts.BlockchainMainChainID,
		Gas:                opts.BlockchainMainGas,
		GasAdjust:          1.2,
		Fees:               sdk.Coins{fee},
	})

	if err != nil {
		logrus.WithError(err).Fatal("failed to create main broadcaster")
	}

	return b
}
