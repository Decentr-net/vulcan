// Package blockchain contains code for interacting with the decentr blockchain.
package blockchain

import (
	"errors"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/bank"

	"github.com/Decentr-net/decentr/app"
	"github.com/Decentr-net/go-broadcaster"
)

//go:generate mockgen -destination=./mock/blockchain.go -package=mock -source=blockchain.go

// nolint: gochecknoinits
func init() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(app.Bech32PrefixAccAddr, app.Bech32PrefixAccPub)
	config.Seal()
}

// ErrInvalidAddress is returned when address is invalid. It is unexpected situation.
var ErrInvalidAddress = errors.New("invalid address")

// Blockchain is interface for interacting with the blockchain.
type Blockchain interface {
	SendStakes(address string, amount int64) error
}

type blockchain struct {
	b    *broadcaster.Broadcaster
	memo string
}

// New returns new instance of Blockchain.
func New(b *broadcaster.Broadcaster, memo string) Blockchain {
	return blockchain{
		b:    b,
		memo: memo,
	}
}

func (b blockchain) SendStakes(address string, amount int64) error {
	to, err := sdk.AccAddressFromBech32(address)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidAddress, address)
	}

	msg := bank.NewMsgSend(b.b.From(), to, sdk.Coins{sdk.Coin{
		Denom:  app.DefaultBondDenom,
		Amount: sdk.NewInt(amount),
	}})
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	return b.b.BroadcastMsg(msg, b.memo)
}
