package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/action/v1/types"

	"github.com/stretchr/testify/require"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
)

func TestGetParams(t *testing.T) {
	k, ctx := keepertest.ActionKeeper(t)
	params := types.DefaultParams()

	require.NoError(t, k.SetParams(ctx, params))
	require.EqualValues(t, params, k.GetParams(ctx))
}
