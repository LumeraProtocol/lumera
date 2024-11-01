package pastelid_test

import (
	"testing"

	keepertest "github.com/pastelnetwork/pasteld/testutil/keeper"
	"github.com/pastelnetwork/pasteld/testutil/nullify"
	pastelid "github.com/pastelnetwork/pasteld/x/pastelid/module"
	"github.com/pastelnetwork/pasteld/x/pastelid/types"
	"github.com/stretchr/testify/require"
)

func TestGenesis(t *testing.T) {
	genesisState := types.GenesisState{
		Params: types.DefaultParams(),

		// this line is used by starport scaffolding # genesis/test/state
	}

	k, ctx := keepertest.PastelidKeeper(t)
	pastelid.InitGenesis(ctx, k, genesisState)
	got := pastelid.ExportGenesis(ctx, k)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)

	// this line is used by starport scaffolding # genesis/test/assert
}
