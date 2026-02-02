package keeper_test

import (
	"testing"

	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestHandleSupernodeMetricsStaleness_NoMetrics_Postpones(t *testing.T) {
	f := initFixture(t)

	// Advance chain height far enough to trigger staleness.
	f.ctx = f.ctx.WithBlockHeight(100)

	valAddr := sdk.ValAddress([]byte("validator_address_20"))
	valAddrStr := valAddr.String()

	f.supernodeKeeper.EXPECT().
		GetParams(gomock.Any()).
		Return(sntypes.Params{
			MetricsUpdateIntervalBlocks: 2,
			MetricsGracePeriodBlocks:    3,
		})

	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.Any()).
		Return([]sntypes.SuperNode{
			{
				ValidatorAddress: valAddrStr,
				States: []*sntypes.SuperNodeStateRecord{
					{State: sntypes.SuperNodeStateActive, Height: 90},
				},
			},
		}, nil)

	f.supernodeKeeper.EXPECT().
		GetMetricsState(gomock.Any(), gomock.Any()).
		Return(sntypes.SupernodeMetricsState{}, false)

	f.supernodeKeeper.EXPECT().
		SetSuperNodePostponed(gomock.Any(), gomock.Any(), "no metrics reported").
		DoAndReturn(func(_ sdk.Context, got sdk.ValAddress, _ string) error {
			require.Equal(t, valAddr, got)
			return nil
		})

	require.NoError(t, f.keeper.HandleSupernodeMetricsStaleness(f.ctx))
}
