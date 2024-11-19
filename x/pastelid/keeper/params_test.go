package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	keepertest "github.com/pastelnetwork/pastel/testutil/keeper"
	"github.com/pastelnetwork/pastel/x/pastelid/types"
)

func TestGetParams(t *testing.T) {
	k, ctx := keepertest.PastelidKeeper(t)
	params := types.DefaultParams()

	require.NoError(t, k.SetParams(ctx, params))
	require.EqualValues(t, params, k.GetParams(ctx))
}
