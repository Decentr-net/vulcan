// Package referral ...
package referral

import (
	"context"
	"fmt"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/Decentr-net/vulcan/internal/blockchain"
	"github.com/Decentr-net/vulcan/internal/blockchain/rest"
	"github.com/Decentr-net/vulcan/internal/storage"
)

// Rewarder ...
type Rewarder struct {
	storage storage.Storage
	bmc     blockchain.Blockchain
	brc     *rest.BlockchainRESTClient

	senderReward   int
	receiverReward int
	uPDVThreshold  int
}

// NewRewarder creates a new instance of Rewarder.
func NewRewarder(s storage.Storage, b blockchain.Blockchain, brc *rest.BlockchainRESTClient,
	senderReward, receiverReward, uPDVThreshold int) *Rewarder {
	return &Rewarder{
		storage:        s,
		bmc:            b,
		brc:            brc,
		senderReward:   senderReward,
		receiverReward: receiverReward,
		uPDVThreshold:  uPDVThreshold,
	}
}

// Run runs the rewarder check referral status loop.
func (r *Rewarder) Run(ctx context.Context, interval time.Duration) {
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
	referrals, err := r.storage.GetUnconfirmedReferralTracking(ctx)
	if err != nil {
		log.WithError(err).Error("failed to get unconfirmed referrals")
		return
	}

	log.Infof("uncofirmed referrals count: %d", len(referrals))

	for _, ref := range referrals {
		logger := log.WithFields(log.Fields{
			"sender":          ref.Sender,
			"receiver":        ref.Receiver,
			"sender reward":   r.senderReward,
			"receiver reward": r.receiverReward,
			"registered at":   ref.RegisteredAt,
			"installed at":    ref.InstalledAt,
		})

		resp, err := r.brc.GetTokenBalance(ctx, ref.Receiver)
		if err != nil {
			logger.WithError(err).Error("failed to get PDV token balance")
			continue
		}

		PDVBalanceFloat64, err := strconv.ParseFloat(resp.Result.Balance, 64)
		if err != nil {
			logger.WithError(err).Error("failed to get parse PDV token balance")
			continue
		}

		PDVBalanceFloat64 -= 1.0
		uPDVBalance := int(PDVBalanceFloat64 * 1e6)

		if uPDVBalance > r.uPDVThreshold {
			r.reward(ctx, ref, logger)
		} else {
			logger.Infof("balance %d less than threshold %d", uPDVBalance, r.uPDVThreshold)
		}
	}
}

func (r *Rewarder) reward(ctx context.Context, ref *storage.ReferralTracking, logger *log.Entry) {
	err := r.storage.InTx(ctx, func(s storage.Storage) error {
		if err := r.storage.TransitionReferralTrackingToConfirmed(
			ctx, ref.Receiver, r.senderReward, r.receiverReward); err != nil {
			return fmt.Errorf("failed to transition referral to confirmed: %w", err)
		}

		if err := r.bmc.SendStakes(ref.Sender, int64(r.senderReward)); err != nil {
			return fmt.Errorf("failed to send stakes to the referral sender: %w", err)
		}

		if err := r.bmc.SendStakes(ref.Receiver, int64(r.receiverReward)); err != nil {
			return fmt.Errorf("failed to send stakes to the referral receiver: %w", err)
		}
		return nil
	})

	if err != nil {
		logger.WithError(err).Error("failed to reward")
		return
	}

	logger.Infof("rewards sent")
}
