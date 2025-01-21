package lumeraid_test

import (
	"testing"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	"github.com/LumeraProtocol/lumera/testutil/nullify"
	lumeraid "github.com/LumeraProtocol/lumera/x/lumeraid/module"
	"github.com/LumeraProtocol/lumera/x/lumeraid/types"
	"github.com/stretchr/testify/require"
)

func TestGenesis(t *testing.T) {
	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
		// this line is used by starport scaffolding # genesis/test/state
	}

	k, ctx := keepertest.LumeraidKeeper(t)
	lumeraid.InitGenesis(ctx, k, genesisState)
	got := lumeraid.ExportGenesis(ctx, k)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)

	// this line is used by starport scaffolding # genesis/test/assert
}
