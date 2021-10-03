package supply

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

const nativeNode = "https://zeus.mainnet.decentr.xyz"

const ethNode = "" // nolint

func TestBlockchain_GetNativeCirculating(t *testing.T) {
	s := supply{nativeNodeURL: nativeNode}

	v, err := s.getNativeCirculatingSupply(context.Background())
	require.NoError(t, err)
	require.NotZero(t, v)
}

////uncomment and set eth node addr to test
//func TestBlockchain_GetERC20Circulating(t *testing.T) {
//	s := supply{erc20NodeURL: ethNode}
//
//	v, err := s.getERC20CirculatingSupply(context.Background())
//	require.NoError(t, err)
//	require.NotZero(t, v)
//}

//func TestBlockchain_GetERC20Circulating(t *testing.T) {
//	s := supply{erc20NodeURL: ethNode, nativeNodeURL: nativeNode}
//
//	v, err := s.poll()
//	require.NoError(t, err)
//	require.NotZero(t, v)
//}

//
//func Test_startPollingCirculatingSupply(t *testing.T) {
//	t.Parallel()
//
//	ctrl := gomock.NewController(t)
//	defer ctrl.Finish()
//
//	s := New(nativeNode, ethNode)
//
//	v, err := s.GetCirculatingSupply()
//	assert.NotZero(t, v)
//	assert.NoError(t, err)
//}

func TestService_GetCirculating(t *testing.T) {
	s := &supply{}

	_, err := s.GetCirculatingSupply()
	require.Error(t, err)

	s.circulatingSupply = 5
	v, err := s.GetCirculatingSupply()
	require.EqualValues(t, 5, v)
	require.NoError(t, err)
}
