package referral

import (
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestConfig_GetSenderBonus(t *testing.T) {
	tt := []struct {
		count int
		want  sdk.Int
	}{
		{1, sdk.NewInt(0)},
		{100, sdk.NewInt(100000000)},
		{101, sdk.NewInt(0)},
		{500, sdk.NewInt(500000000)},
		{510, sdk.NewInt(0)},
	}

	c := NewConfig(sdk.NewDec(100), 30)

	for i := range tt {
		tc := tt[i]
		t.Run(fmt.Sprintf("count=%d", tc.count), func(t *testing.T) {
			reward := c.GetSenderBonus(tc.count)
			require.Truef(t, tc.want.Equal(reward), "%s != %s", tc.want, reward)
		})
	}
}

func TestConfig_GetSenderReward(t *testing.T) {
	tt := []struct {
		count int
		want  sdk.Int
	}{
		{0, sdk.NewInt(0)},
		{1, sdk.NewInt(10000000)},
		{100, sdk.NewInt(10000000)},
		{150, sdk.NewInt(12500000)},
		{350, sdk.NewInt(15000000)},
		{12500, sdk.NewInt(20000000)},
	}

	c := NewConfig(sdk.NewDec(100), 30)

	for i := range tt {
		tc := tt[i]
		t.Run(fmt.Sprintf("count=%d", tc.count), func(t *testing.T) {
			reward := c.GetSenderReward(tc.count)
			require.Truef(t, tc.want.Equal(reward), "%s != %s", tc.want, reward)
		})
	}
}
