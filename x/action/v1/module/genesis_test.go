package action_test

import (
	"testing"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	"github.com/LumeraProtocol/lumera/testutil/nullify"
	actionmodulev1 "github.com/LumeraProtocol/lumera/x/action/v1/module"
	"github.com/LumeraProtocol/lumera/x/action/v1/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGenesis(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
	
		// this line is used by starport scaffolding # genesis/test/state
	}

	k, ctx := keepertest.ActionKeeper(t, ctrl)
	actionmodulev1.InitGenesis(ctx, k, genesisState)
	got := actionmodulev1.ExportGenesis(ctx, k)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)

	// this line is used by starport scaffolding # genesis/test/assert
}
