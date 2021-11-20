package rest

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBlockchainRESTClient_GetTokenBalance(t *testing.T) {
	t.Skip("skip call to real node")
	c := NewBlockchainRESTClient("http://zeus.mainnet.decentr.xyz")
	resp, err := c.GetTokenBalance(context.Background(), "decentr1exttfeeuyqsa9a0g2ghfjjf2py7hxc48xzq7qw")
	require.NoError(t, err)

	require.NotEmpty(t, resp.Height)
	balance, err := strconv.ParseFloat(resp.Result.Balance, 64)
	require.NoError(t, err)
	require.Greater(t, balance, 1.0)
}
