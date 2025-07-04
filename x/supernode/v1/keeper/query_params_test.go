package keeper_test

import (
	"testing"

	types2 "github.com/LumeraProtocol/lumera/x/action/v1/types"

	"github.com/stretchr/testify/require"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
<<<<<<<< HEAD:x/action/v1/keeper/query_params_test.go
)

func TestParamsQuery(t *testing.T) {
	keeper, ctx := keepertest.ActionKeeper(t)
	params := types2.DefaultParams()
	require.NoError(t, keeper.SetParams(ctx, params))

	response, err := keeper.Params(ctx, &types2.QueryParamsRequest{})
========
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
)

func TestParamsQuery(t *testing.T) {
	k, ctx := keepertest.SupernodeKeeper(t)
	params := types.DefaultParams()
	require.NoError(t, k.SetParams(ctx, params))

	q := keeper.NewQueryServerImpl(k)
	response, err := q.Params(ctx, &types.QueryParamsRequest{})
>>>>>>>> f8437a0 (IBC & Wasm upgrade):x/supernode/v1/keeper/query_params_test.go
	require.NoError(t, err)
	require.Equal(t, &types2.QueryParamsResponse{Params: params}, response)
}
