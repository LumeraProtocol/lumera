package claim_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	claim "github.com/LumeraProtocol/lumera/x/claim/module"
	claimtestutils "github.com/LumeraProtocol/lumera/x/claim/testutils"
	"github.com/LumeraProtocol/lumera/x/claim/types"
)

func TestGenesis(t *testing.T) {
	// Get the default genesis state first
	defaultGenState := types.DefaultGenesis()

	// Create test genesis state matching the default values
	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
		// Set to 0 in tests to avoid loading external CSV totals
		TotalClaimableAmount: 0,
		// Will be populated during initialization
		ClaimRecords: []types.ClaimRecord{}, // Empty records for test
		ClaimsDenom:  types.DefaultClaimsDenom,
	}

	testData, err := claimtestutils.GenerateClaimingTestData()
	require.NoError(t, err)

	// generate a CSV file with the test data
	claimsPath, err := claimtestutils.GenerateClaimsCSVFile([]claimtestutils.ClaimCSVRecord{
		{OldAddress: testData.OldAddress, Amount: defaultGenState.TotalClaimableAmount},
	}, nil)
	require.NoError(t, err)
	// Ensure the file is cleaned up after the test
	t.Cleanup(func() {
		claimtestutils.CleanupClaimsCSVFile(claimsPath)
	})

	k, ctx := keepertest.ClaimKeeper(t, claimsPath)
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
