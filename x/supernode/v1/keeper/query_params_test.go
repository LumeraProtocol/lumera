package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
)

func TestParamsQuery(t *testing.T) {
	k, ctx := keepertest.SupernodeKeeper(t)
	params := types.DefaultParams()
	require.NoError(t, k.SetParams(ctx, params))

	q := keeper.NewQueryServerImpl(k)
	response, err := q.Params(ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, &types.QueryParamsResponse{Params: params}, response)
}
