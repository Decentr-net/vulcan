// Package supply provides access to supply amount.
package supply

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/Decentr-net/decentr/config"
)

//go:generate mockgen -destination=./mock/supply.go -package=mock -source=supply.go

const udecDenominator = 1e6

// nolint
var (
	erc20TokenAddr       = common.HexToAddress("0x30f271c9e86d2b7d00a6376cd96a1cfbd5f0b9b3")
	erc20LockedTokenAddr = common.HexToAddress("0x91b028C41b0268d346E78209Eb5EF5579487b639")
)

// Supply ...
type Supply interface {
	GetCirculatingSupply() (int64, error)
}

type supply struct {
	nativeBankClient banktypes.QueryClient
	erc20NodeURL     string

	circulatingSupply int64
}

// New returns new instance of supply.
func New(nativeBankClient banktypes.QueryClient, erc20NodeURL string) *supply { // nolint
	s := &supply{
		nativeBankClient: nativeBankClient,
		erc20NodeURL:     erc20NodeURL,
	}

	s.startPolling()

	return s
}

func (s *supply) PingContext(_ context.Context) error {
	if _, err := s.GetCirculatingSupply(); err != nil {
		return fmt.Errorf("invalid circulating supply: %w", err)
	}
	return nil
}

func (s *supply) GetCirculatingSupply() (int64, error) {
	if s.circulatingSupply == 0 {
		return 0, errors.New("circulating supply is unavailable")
	}
	return s.circulatingSupply, nil
}

func (s *supply) startPolling() {
	refresh := func() {
		val, err := s.poll()
		if err != nil {
			log.WithError(err).Error("failed to get circulating")
			return
		}
		s.circulatingSupply = val
	}

	refresh()

	ticker := time.NewTicker(time.Hour)
	go func() {
		for range ticker.C {
			refresh()
		}
	}()
}

func (s *supply) poll() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	gr, ctx := errgroup.WithContext(ctx)

	var native, erc20 int64
	gr.Go(func() error {
		v, err := s.getNativeCirculatingSupply(ctx)
		if err != nil {
			return fmt.Errorf("failed to get native circulating: %w", err)
		}
		native = v

		return nil
	})
	gr.Go(func() error {
		v, err := s.getERC20CirculatingSupply(ctx)
		if err != nil {
			return fmt.Errorf("failed to get erc20 circulating: %w", err)
		}
		erc20 = v

		return nil
	})

	if err := gr.Wait(); err != nil {
		return 0, err
	}

	return native + erc20, nil
}

func (s supply) getNativeCirculatingSupply(ctx context.Context) (int64, error) {
	resp, err := s.nativeBankClient.SupplyOf(ctx, &banktypes.QuerySupplyOfRequest{
		Denom: config.DefaultBondDenom,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to get supply: %w", err)
	}

	return resp.Amount.Amount.Int64() / udecDenominator, nil
}

func (s supply) getERC20CirculatingSupply(ctx context.Context) (int64, error) {
	client, err := ethclient.DialContext(ctx, s.erc20NodeURL)
	if err != nil {
		return 0, fmt.Errorf("failed to create ethclient: %w", err)
	}

	instance, err := NewDecentr(erc20TokenAddr, client)
	if err != nil {
		return 0, fmt.Errorf("failed to create token instance: %w", err)
	}

	total, err := instance.TotalSupply(&bind.CallOpts{Context: ctx})
	if err != nil {
		return 0, fmt.Errorf("failed to get total supply: %w", err)
	}

	reserved, err := instance.BalanceOf(&bind.CallOpts{Context: ctx}, erc20TokenAddr)
	if err != nil {
		return 0, fmt.Errorf("failed to get reserved: %w", err)
	}

	locked, err := instance.BalanceOf(&bind.CallOpts{Context: ctx}, erc20LockedTokenAddr)
	if err != nil {
		return 0, fmt.Errorf("failed to get reserved: %w", err)
	}

	var denom = big.NewInt(10)
	denom.Exp(denom, big.NewInt(18), nil)

	supply := total.Sub(total, reserved)
	supply = supply.Sub(supply, locked)

	return supply.Quo(supply, denom).Int64(), nil
}
