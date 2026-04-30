package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	"github.com/stretchr/testify/require"
)

func TestParamsQuery(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	response, err := qs.Params(f.ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, &types.QueryParamsResponse{Params: params.WithDefaults()}, response)
}

func TestSetParamsRejectsInvalidParams(t *testing.T) {
	f := initFixture(t)

	before := f.keeper.GetParams(f.ctx)
	invalid := before
	invalid.KeepLastEpochEntries = 1 // below StorageTruthOldClassAFaultWindow/divergence lookbacks

	err := f.keeper.SetParams(f.ctx, invalid)
	require.Error(t, err)
	require.Contains(t, err.Error(), "keep_last_epoch_entries")

	after := f.keeper.GetParams(f.ctx)
	require.Equal(t, before, after)
}
