package action_test

import (
	"testing"

	action "github.com/LumeraProtocol/lumera/x/action/v1/module"
	types2 "github.com/LumeraProtocol/lumera/x/action/v1/types"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	"github.com/LumeraProtocol/lumera/testutil/nullify"
	"github.com/stretchr/testify/require"
)

func TestGenesis(t *testing.T) {
	genesisState := types2.GenesisState{
		Params: types2.DefaultParams(),

		// this line is used by starport scaffolding # genesis/test/state
	}

	k, ctx := keepertest.ActionKeeper(t)
	action.InitGenesis(ctx, k, genesisState)
	got := action.ExportGenesis(ctx, k)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)

	// this line is used by starport scaffolding # genesis/test/assert
}
