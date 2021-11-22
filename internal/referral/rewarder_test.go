package referral

import (
	"encoding/json"
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
