package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

func TestSetParamsAppliesDefaults(t *testing.T) {
	k, _, ctx := setupMsgServer(t)
	wctx := sdk.UnwrapSDKContext(ctx)

	// store params that omit the newly added metrics fields
	empty := types.Params{}
	require.NoError(t, k.SetParams(wctx, empty))

	stored := k.GetParams(wctx)
	require.Equal(t, types.DefaultMetricsUpdateInterval, stored.MetricsUpdateInterval)
	require.Equal(t, types.DefaultMetricsGracePeriodBlocks, stored.MetricsGracePeriodBlocks)
	require.Equal(t, types.DefaultMetricsFreshnessMaxBlocks, stored.MetricsFreshnessMaxBlocks)
	require.Equal(t, types.DefaultMinSupernodeVersion, stored.MinSupernodeVersion)
	require.Equal(t, types.DefaultRequiredOpenPorts, stored.RequiredOpenPorts)
}
