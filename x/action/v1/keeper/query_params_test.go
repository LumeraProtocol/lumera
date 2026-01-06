package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	"github.com/LumeraProtocol/lumera/x/action/v1/types"
)

func TestParamsQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	k, ctx := keepertest.ActionKeeper(t, ctrl)

	params := types.DefaultParams()
	params.BaseActionFee = sdk.NewInt64Coin("stake", 100)
	err := k.SetParams(ctx, params)
	require.NoError(t, err)
	
	q := keeper.NewQueryServerImpl(k)

	response, err := q.Params(ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, &types.QueryParamsResponse{Params: params}, response)
}
