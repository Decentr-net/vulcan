package referral

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Decentr-net/vulcan/internal/blockchain/rest"
)

func Test_balanceInUPDV(t *testing.T) {
	var resp rest.TokenResponse
	require.NoError(t, json.Unmarshal([]byte(`{
  "height": "806305",
  "result": {
    "balance": "1.000126000000000000",
    "balanceDelta": "0.000600000000000000"
  }
}`), &resp))

	balance := balanceInUPDV(&resp)
	require.Equal(t, int64(126), balance)
}

func TestConfig_GetSenderBonus(t *testing.T) {
	tt := []struct {
		count int
		want  int
	}{
		{1, 0},
		{100, 100000000},
		{101, 0},
		{500, 500000000},
		{510, 0},
	}

	c := NewConfig(100, 30)

	for i := range tt {
		tc := tt[i]
		t.Run(fmt.Sprintf("count=%d", tc.count), func(t *testing.T) {
			require.Equal(t, tc.want, c.GetSenderBonus(tc.count))
		})
	}
}

func TestConfig_GetSenderReward(t *testing.T) {
	tt := []struct {
		count int
		want  int
	}{
		{1, 10000000},
		{100, 10000000},
		{150, 12500000},
		{350, 15000000},
		{12500, 20000000},
	}

	c := NewConfig(100, 30)

	for i := range tt {
		tc := tt[i]
		t.Run(fmt.Sprintf("count=%d", tc.count), func(t *testing.T) {
			require.Equal(t, tc.want, c.GetSenderReward(tc.count))
		})
	}
}
