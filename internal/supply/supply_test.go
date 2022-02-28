package supply

//import (
//	"context"
//	"testing"
//
//	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
//	"github.com/golang/mock/gomock"
//	"github.com/stretchr/testify/assert"
//	"github.com/stretchr/testify/require"
//	"google.golang.org/grpc"
//)
//
//const nativeNode = "zeus.mainnet.decentr.xyz:9090"
//
//const ethNode = "" // nolint
//
//func getNativeBankClient() banktypes.QueryClient {
//	nativeNodeConn, _ := grpc.Dial(
//		nativeNode,
//		grpc.WithInsecure(),
//	)
//
//	return banktypes.NewQueryClient(nativeNodeConn)
//}
//
//func TestBlockchain_GetNativeCirculating(t *testing.T) {
//	s := supply{nativeBankClient: getNativeBankClient()}
//
//	v, err := s.getNativeCirculatingSupply(context.Background())
//	require.NoError(t, err)
//	require.NotZero(t, v)
//}
//
////uncomment and set eth node addr to test
//func TestBlockchain_GetERC20Circulating(t *testing.T) {
//	s := supply{erc20NodeURL: ethNode}
//
//	v, err := s.getERC20CirculatingSupply(context.Background())
//	require.NoError(t, err)
//	require.NotZero(t, v)
//}
//
//func TestBlockchain_Poll(t *testing.T) {
//	s := supply{erc20NodeURL: ethNode, nativeBankClient: getNativeBankClient()}
//
//	v, err := s.poll()
//	require.NoError(t, err)
//	require.NotZero(t, v)
//
//	t.Logf("supply: %d", v)
//}
//
//func Test_startPollingCirculatingSupply(t *testing.T) {
//	t.Parallel()
//
//	ctrl := gomock.NewController(t)
//	defer ctrl.Finish()
//
//	s := New(getNativeBankClient(), ethNode)
//
//	v, err := s.GetCirculatingSupply()
//	assert.NotZero(t, v)
//	assert.NoError(t, err)
//}
//
//func TestService_GetCirculating(t *testing.T) {
//	s := &supply{}
//
//	_, err := s.GetCirculatingSupply()
//	require.Error(t, err)
//
//	s.circulatingSupply = 5
//	v, err := s.GetCirculatingSupply()
//	require.EqualValues(t, 5, v)
//	require.NoError(t, err)
//}
