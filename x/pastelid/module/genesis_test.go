package pastelid_test

import (
	"testing"

	keepertest "github.com/pastelnetwork/pastel/testutil/keeper"
	"github.com/pastelnetwork/pastel/testutil/nullify"
	pastelid "github.com/pastelnetwork/pastel/x/pastelid/module"
	"github.com/pastelnetwork/pastel/x/pastelid/types"
	"github.com/stretchr/testify/require"
)

func TestGenesis(t *testing.T) {
	genesisState := types.GenesisState{
		Params: types.DefaultParams(),

		PastelidEntryList: []types.PastelidEntry{
			{
				Address: "0",
			},
			{
				Address: "1",
			},
		},
		// this line is used by starport scaffolding # genesis/test/state
	}

	k, ctx := keepertest.PastelidKeeper(t)
	pastelid.InitGenesis(ctx, k, genesisState)
	got := pastelid.ExportGenesis(ctx, k)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)

	require.ElementsMatch(t, genesisState.PastelidEntryList, got.PastelidEntryList)
	// this line is used by starport scaffolding # genesis/test/assert
}
