// Package blockchain contains code for interacting with the decentr blockchain.
package blockchain

import (
	"errors"
	"fmt"

	"github.com/avast/retry-go"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/Decentr-net/decentr/config"
	"github.com/Decentr-net/go-broadcaster"
)

//go:generate mockgen -destination=./mock/blockchain.go -package=mock -source=blockchain.go

// nolint: gochecknoinits
func init() {
	config.SetAddressPrefixes()
}

// ErrInvalidAddress is returned when address is invalid. It is unexpected situation.
var ErrInvalidAddress = errors.New("invalid address")

// Stake ...
type Stake struct {
	Address string
	Amount  sdk.Int
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
	sendStakes := func() error {
		messages := make([]sdk.Msg, len(stakes))
		for idx, stake := range stakes {
			to, err := sdk.AccAddressFromBech32(stake.Address)
			if err != nil {
				return fmt.Errorf("%w: %s", ErrInvalidAddress, stake.Address)
			}

			messages[idx] = banktypes.NewMsgSend(b.b.From(), to, sdk.Coins{sdk.Coin{
				Denom:  config.DefaultBondDenom,
				Amount: stake.Amount,
			}})
			if err := messages[idx].ValidateBasic(); err != nil {
				return err
			}
		}

		if _, err := b.b.Broadcast(messages, memo); err != nil {
			return fmt.Errorf("failed to broadcast msg: %w", err)
		}

		return nil
	}

	return retry.Do(sendStakes, retry.Attempts(3))
}
