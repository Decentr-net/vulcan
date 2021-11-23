package rest

import (
	"context"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/stretchr/testify/require"
)

func TestBlockchainRESTClient_GetTokenBalance(t *testing.T) {
	t.Skip("skip call to real node")
	c := NewBlockchainRESTClient("http://zeus.mainnet.decentr.xyz")
	resp, err := c.GetTokenBalance(context.Background(), "decentr1exttfeeuyqsa9a0g2ghfjjf2py7hxc48xzq7qw")
	require.NoError(t, err)

	require.NotEmpty(t, resp.Height)
	balance := resp.Result.Balance
	require.NoError(t, err)
	require.True(t, balance.GT(sdk.NewDec(1)))
}
