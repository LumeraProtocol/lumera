package keeper_test

import (
	"testing"

	types2 "github.com/LumeraProtocol/lumera/x/action/v1/types"

	"github.com/stretchr/testify/require"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
)

func TestParamsQuery(t *testing.T) {
	keeper, ctx := keepertest.ActionKeeper(t)
	params := types2.DefaultParams()
	require.NoError(t, keeper.SetParams(ctx, params))

	response, err := keeper.Params(ctx, &types2.QueryParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, &types2.QueryParamsResponse{Params: params}, response)
}
