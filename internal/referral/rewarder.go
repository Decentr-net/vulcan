// Package referral ...
package referral

import (
	"context"
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	log "github.com/sirupsen/logrus"

	"github.com/Decentr-net/decentr/x/token/types"
	"github.com/Decentr-net/vulcan/internal/blockchain"
	"github.com/Decentr-net/vulcan/internal/blockchain/rest"
	"github.com/Decentr-net/vulcan/internal/storage"
)

// Config ...
type Config struct {
	SenderReward   int
	ReceiverReward int
	ThresholdUPDV  int
	ThresholdDays  int
}

// Rewarder ...
type Rewarder struct {
	storage storage.Storage
	bmc     blockchain.Blockchain
	brc     *rest.BlockchainRESTClient
	rc      Config
}

// NewRewarder creates a new instance of Rewarder.
func NewRewarder(s storage.Storage, b blockchain.Blockchain, brc *rest.BlockchainRESTClient,
	rc Config) *Rewarder {
	return &Rewarder{
		storage: s,
		bmc:     b,
		brc:     brc,
		rc:      rc,
	}
}

// Run runs the rewarder check referral status loop.
func (r *Rewarder) Run(ctx context.Context, interval time.Duration) {
	r.do(ctx)

	ticker := time.NewTicker(interval)
	go func(ticker *time.Ticker) {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.do(ctx)
			}
		}
	}(ticker)
}

func (r *Rewarder) do(ctx context.Context) {
	referrals, err := r.storage.GetUnconfirmedReferralTracking(ctx, r.rc.ThresholdDays)
	if err != nil {
		log.WithError(err).Error("failed to get unconfirmed referrals")
		return
	}

	log.Infof("uncofirmed referrals count: %d", len(referrals))

	for _, ref := range referrals {
		logger := r.getLogger(ref)

		resp, err := r.brc.GetTokenBalance(ctx, ref.Receiver)
		if err != nil {
			logger.WithError(err).Error("failed to get PDV token balance")
			continue
		}

		uPDVBalance := balanceInUPDV(resp)

		if uPDVBalance > int64(r.rc.ThresholdUPDV) {
			r.reward(ctx, ref)
		} else {
			logger.Infof("balance %d less than threshold %d", uPDVBalance, r.rc.ThresholdUPDV)
		}
	}
}

func balanceInUPDV(resp *rest.TokenResponse) int64 {
	return resp.Result.Balance.Sub(sdk.NewDec(1)).QuoInt64(types.Denominator).Int64() / types.Denominator
}

func (r *Rewarder) reward(ctx context.Context, ref *storage.ReferralTracking) {
	logger := r.getLogger(ref)

	if err := r.storage.InTx(ctx, func(s storage.Storage) error {
		if err := r.storage.TransitionReferralTrackingToConfirmed(
			ctx, ref.Receiver, r.rc.SenderReward, r.rc.ReceiverReward); err != nil {
			return fmt.Errorf("failed to transition referral to confirmed: %w", err)
		}

		stakes := []blockchain.Stake{
			{Address: ref.Sender, Amount: int64(r.rc.SenderReward)},
			{Address: ref.Receiver, Amount: int64(r.rc.ReceiverReward)},
		}

		if err := r.bmc.SendStakes(stakes); err != nil {
			return fmt.Errorf("failed to send stakes: %w", err)
		}

		return nil
	}); err != nil {
		logger.WithError(err).Error("failed to reward")
		return
	}

	logger.Infof("rewards sent")
}

func (r *Rewarder) getLogger(ref *storage.ReferralTracking) *log.Entry {
	return log.WithFields(log.Fields{
		"sender":          ref.Sender,
		"receiver":        ref.Receiver,
		"sender reward":   r.rc.SenderReward,
		"receiver reward": r.rc.ReceiverReward,
		"registered at":   ref.RegisteredAt,
		"installed at":    ref.InstalledAt,
	})
}
