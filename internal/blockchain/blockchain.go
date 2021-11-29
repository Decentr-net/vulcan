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

// Stake ...
type Stake struct {
	Address string
	Amount  int64
}

// Blockchain is interface for interacting with the blockchain.
type Blockchain interface {
	SendStakes(stakes []Stake, memo string) error
}

type blockchain struct {
	b *broadcaster.Broadcaster
}

// New returns new instance of Blockchain.
func New(b *broadcaster.Broadcaster) Blockchain {
	return blockchain{
		b: b,
	}
}

// SendStakes ...
func (b blockchain) SendStakes(stakes []Stake, memo string) error {
	msgs := make([]sdk.Msg, len(stakes))
	for idx, stake := range stakes {
		to, err := sdk.AccAddressFromBech32(stake.Address)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidAddress, stake.Address)
		}

		msgs[idx] = bank.NewMsgSend(b.b.From(), to, sdk.Coins{sdk.Coin{
			Denom:  app.DefaultBondDenom,
			Amount: sdk.NewInt(stake.Amount),
		}})
		if err := msgs[idx].ValidateBasic(); err != nil {
			return err
		}
	}

	return b.b.Broadcast(msgs, memo)
}
