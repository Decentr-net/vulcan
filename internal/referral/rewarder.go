// Package referral ...
package referral

import (
	"context"
	"fmt"
	"strconv"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	log "github.com/sirupsen/logrus"

	tokentypes "github.com/Decentr-net/decentr/x/token/types"

	"github.com/Decentr-net/vulcan/internal/blockchain"
	"github.com/Decentr-net/vulcan/internal/storage"
)

const denominator = 6

// Bonus ...
type Bonus struct {
	Count  int     `json:"count"`
	Reward sdk.Int `json:"reward"`
}

// RewardLevel ...
type RewardLevel struct {
	From   int     `json:"from"`
	To     *int    `json:"to"`
	Reward sdk.Int `json:"reward"`
}

// Config ...
// swagger:model
type Config struct {
	ThresholdPDV       sdk.Dec       `json:"thresholdPDV"`
	ThresholdDays      int           `json:"thresholdDays"`
	ReceiverReward     sdk.Int       `json:"receiverReward"`
	SenderBonuses      []Bonus       `json:"senderBonus"`
	SenderRewardLevels []RewardLevel `json:"senderRewardLevels"`
}

// NewConfig creates a new instance of Config.
func NewConfig(thresholdPDV sdk.Dec, thresholdDays int) Config {
	intPrt := func(val int) *int {
		return &val
	}

	toReward := func(val float64) sdk.Int {
		s := strconv.FormatFloat(val, 'f', -1, 64)
		return sdk.MustNewDecFromStr(s).Mul(sdk.NewIntWithDecimal(1, denominator).ToDec()).TruncateInt()
	}

	return Config{
		ThresholdPDV:   thresholdPDV,
		ThresholdDays:  thresholdDays,
		ReceiverReward: sdk.NewIntWithDecimal(10, 6),
		SenderBonuses: []Bonus{
			{Count: 100, Reward: toReward(100)},
			{Count: 250, Reward: toReward(250)},
			{Count: 500, Reward: toReward(500)},
			{Count: 1000, Reward: toReward(1000)},
			{Count: 2500, Reward: toReward(2500)},
			{Count: 5000, Reward: toReward(5000)},
			{Count: 10000, Reward: toReward(10000)},
		},
		SenderRewardLevels: []RewardLevel{
			{From: 1, To: intPrt(100), Reward: toReward(10)},
			{From: 101, To: intPrt(250), Reward: toReward(12.5)},
			{From: 251, To: intPrt(500), Reward: toReward(15)},
			{From: 501, To: nil, Reward: toReward(20)},
		},
	}
}

// GetSenderBonus returns a bonus reward.
func (c Config) GetSenderBonus(confirmedReferralsCount int) sdk.Int {
	for _, b := range c.SenderBonuses {
		if b.Count == confirmedReferralsCount {
			return b.Reward
		}
	}
	return sdk.ZeroInt()
}

// GetSenderReward returns a sender reward.
func (c Config) GetSenderReward(confirmedReferralsCount int) sdk.Int {
	if confirmedReferralsCount == 0 {
		return sdk.ZeroInt()
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
	brc     tokentypes.QueryClient
	rc      Config
}

// NewRewarder creates a new instance of Rewarder.
func NewRewarder(s storage.Storage, b blockchain.Blockchain, brc tokentypes.QueryClient,
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

		address, err := sdk.AccAddressFromBech32(ref.Receiver)
		if err != nil {
			logger.WithError(err).Error("failed to parse address")
			continue
		}

		resp, err := r.brc.Balance(ctx, &tokentypes.BalanceRequest{
			Address: address,
		})
		if err != nil {
			logger.WithError(err).Error("failed to get PDV token balance")
			continue
		}

		if resp.Balance.Dec.GT(r.rc.ThresholdPDV) {
			count, err := r.storage.GetConfirmedReferralTrackingCount(ctx, ref.Sender)
			if err != nil {
				logger.WithError(err).Error("failed to get confirmed referrals count")
				return
			}
			r.reward(ctx, ref, count+1)
		} else {
			logger.Infof("balance %d less than threshold %d", resp.Balance.Dec, r.rc.ThresholdPDV)
		}
	}
}

func (r *Rewarder) reward(ctx context.Context, ref *storage.ReferralTracking, confirmedReferralsCount int) {
	logger := r.getLogger(ref)

	senderReward := r.rc.GetSenderReward(confirmedReferralsCount)
	senderBonus := r.rc.GetSenderBonus(confirmedReferralsCount)
	totalSenderReward := senderReward.Add(senderBonus)

	memo := "Decentr referral reward"
	if !senderBonus.IsZero() {
		memo = "Decentr referral reward with bonus"
	}

	if err := r.storage.InTx(ctx, func(s storage.Storage) error {
		if err := r.storage.TransitionReferralTrackingToConfirmed(
			ctx, ref.Receiver, totalSenderReward, r.rc.ReceiverReward); err != nil {
			return fmt.Errorf("failed to transition referral to confirmed: %w", err)
		}

		stakes := []blockchain.Stake{
			{Address: ref.Sender, Amount: totalSenderReward},
			{Address: ref.Receiver, Amount: r.rc.ReceiverReward},
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
