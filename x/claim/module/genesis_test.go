package claim_test

import (
	"testing"

	keepertest "github.com/pastelnetwork/pastel/testutil/keeper"
	claim "github.com/pastelnetwork/pastel/x/claim/module"
	"github.com/pastelnetwork/pastel/x/claim/types"
	"github.com/stretchr/testify/require"
)

func TestGenesis(t *testing.T) {
	// Get the default genesis state first
	defaultGenState := types.DefaultGenesis()

	// Create test genesis state matching the default values
	genesisState := types.GenesisState{
		Params:               types.DefaultParams(),
		TotalClaimableAmount: defaultGenState.TotalClaimableAmount, // Match the default amount
		ModuleAccount:        "",                                   // Will be populated during initialization
		ClaimRecords:         []types.ClaimRecord{},                // Empty records for test
		ClaimsDenom:          types.DefaultClaimsDenom,
	}

	k, ctx := keepertest.ClaimKeeper(t)
	claim.InitGenesis(ctx, k, genesisState)
	got := claim.ExportGenesis(ctx, k)
	require.NotNil(t, got)

	// Verify params
	require.Equal(t, genesisState.Params, got.Params)

	// Verify the module account exists
	moduleAcc := k.GetAccountKeeper().GetModuleAccount(ctx, types.ModuleName)
	require.NotNil(t, moduleAcc)
	require.Equal(t, types.ModuleName, moduleAcc.GetName())

	// Verify permissions
	require.Contains(t, moduleAcc.GetPermissions(), "minter")
	require.Contains(t, moduleAcc.GetPermissions(), "burner")

	// Verify total claimable amount matches default genesis state
	require.Equal(t, defaultGenState.TotalClaimableAmount, got.TotalClaimableAmount)
}
