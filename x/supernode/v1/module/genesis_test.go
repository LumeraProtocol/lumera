package supernode_test

import (
	"github.com/LumeraProtocol/lumera/x/supernode/v1/module"
	types2 "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	"testing"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	"github.com/LumeraProtocol/lumera/testutil/nullify"
	"github.com/stretchr/testify/require"
)

func TestGenesis(t *testing.T) {
	genesisState := types2.GenesisState{
		Params: types2.DefaultParams(),

		// this line is used by starport scaffolding # genesis/test/state
	}

	k, ctx := keepertest.SupernodeKeeper(t)
	supernode.InitGenesis(ctx, k, genesisState)
	got := supernode.ExportGenesis(ctx, k)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)

	// this line is used by starport scaffolding # genesis/test/assert
}
