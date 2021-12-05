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

// Bonus ...
type Bonus struct {
	Count  int `json:"count"`
	Reward int `json:"reward"`
}

// RewardLevel ...
type RewardLevel struct {
	From   int  `json:"from"`
	To     *int `json:"to"`
	Reward int  `json:"reward"`
}

// Config ...
// swagger:model
type Config struct {
	ThresholdUPDV      int           `json:"thresholdUpdv"`
	ThresholdDays      int           `json:"thresholdDays"`
	ReceiverReward     int           `json:"receiverReward"`
	SenderBonuses      []Bonus       `json:"senderBonus"`
	SenderRewardLevels []RewardLevel `json:"senderRewardLevels"`
}

// NewConfig creates a new instance of Config.
func NewConfig(thresholdUPDV, thresholdDays int) Config {
	intPrt := func(val int) *int {
		return &val
	}

	toUPDV := func(val float64) int {
		return int(val * float64(types.Denominator))
	}

	return Config{
		ThresholdUPDV:  thresholdUPDV,
		ThresholdDays:  thresholdDays,
		ReceiverReward: 10000000,
		SenderBonuses: []Bonus{
			{Count: 100, Reward: toUPDV(100)},
			{Count: 250, Reward: toUPDV(250)},
			{Count: 500, Reward: toUPDV(500)},
			{Count: 1000, Reward: toUPDV(1000)},
			{Count: 2500, Reward: toUPDV(2500)},
			{Count: 5000, Reward: toUPDV(5000)},
			{Count: 10000, Reward: toUPDV(10000)},
		},
		SenderRewardLevels: []RewardLevel{
			{From: 1, To: intPrt(100), Reward: toUPDV(10)},
			{From: 101, To: intPrt(250), Reward: toUPDV(12.5)},
			{From: 251, To: intPrt(500), Reward: toUPDV(15)},
			{From: 501, To: nil, Reward: toUPDV(20)},
		},
	}
}

// GetSenderBonus returns a bonus reward.
func (c Config) GetSenderBonus(confirmedReferralsCount int) int {
	for _, b := range c.SenderBonuses {
		if b.Count == confirmedReferralsCount {
			return b.Reward
		}
	}
	return 0
}

// GetSenderReward returns a sender reward.
func (c Config) GetSenderReward(confirmedReferralsCount int) int {
	if confirmedReferralsCount == 0 {
		return 0
	}

	for _, r := range c.SenderRewardLevels {
		if confirmedReferralsCount >= r.From && r.To != nil && confirmedReferralsCount <= *r.To {
			return r.Reward
		}
	}

	return c.SenderRewardLevels[len(c.SenderRewardLevels)-1].Reward
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
			count, err := r.storage.GetConfirmedReferralTrackingCount(ctx, ref.Sender)
			if err != nil {
				logger.WithError(err).Error("failed to get confirmed referrals count")
				return
			}
			r.reward(ctx, ref, count+1)
		} else {
			logger.Infof("balance %d less than threshold %d", uPDVBalance, r.rc.ThresholdUPDV)
		}
	}
}

func balanceInUPDV(resp *rest.TokenResponse) int64 {
	return resp.Result.Balance.Sub(sdk.NewDec(1)).QuoInt64(types.Denominator).Int64() / types.Denominator
}

func (r *Rewarder) reward(ctx context.Context, ref *storage.ReferralTracking, confirmedReferralsCount int) {
	logger := r.getLogger(ref)

	senderReward := r.rc.GetSenderReward(confirmedReferralsCount)
	senderBonus := r.rc.GetSenderBonus(confirmedReferralsCount)
	totalSenderReward := senderReward + senderBonus

	memo := "Decentr referral reward"
	if senderBonus != 0 {
		memo = "Decentr referral reward with bonus"
	}

	if err := r.storage.InTx(ctx, func(s storage.Storage) error {
		if err := r.storage.TransitionReferralTrackingToConfirmed(
			ctx, ref.Receiver, totalSenderReward, r.rc.ReceiverReward); err != nil {
			return fmt.Errorf("failed to transition referral to confirmed: %w", err)
		}

		stakes := []blockchain.Stake{
			{Address: ref.Sender, Amount: int64(totalSenderReward)},
			{Address: ref.Receiver, Amount: int64(r.rc.ReceiverReward)},
		}

		if err := r.bmc.SendStakes(stakes, memo); err != nil {
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
		"sender":        ref.Sender,
		"receiver":      ref.Receiver,
		"registered at": ref.RegisteredAt,
		"installed at":  ref.InstalledAt,
	})
}
