package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	"github.com/LumeraProtocol/lumera/x/lumeraid/types"
	"github.com/LumeraProtocol/lumera/x/lumeraid/keeper"
)

func TestParamsQuery(t *testing.T) {
	k, ctx := keepertest.LumeraidKeeper(t)
	q := keeper.NewQueryServerImpl(k)
	params := types.DefaultParams()
	require.NoError(t, k.SetParams(ctx, params))

	response, err := q.Params(ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, &types.QueryParamsResponse{Params: params}, response)
}
