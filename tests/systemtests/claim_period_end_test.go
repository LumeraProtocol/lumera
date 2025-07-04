//go:build system_test

package system

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	claimtestutils "github.com/LumeraProtocol/lumera/x/claim/testutils"
	claimtypes "github.com/LumeraProtocol/lumera/x/claim/types"
)

func TestClaimPeriodEndBurn(t *testing.T) {
	// Reset chain for a clean state
	sut.ResetChain(t)

	// Generate test data for claims
	testData, err := claimtestutils.GenerateClaimingTestData()
	require.NoError(t, err)

	// Setup test parameters
	claimAmount := uint64(1_000_000)
	totalClaimableAmount := 2_000_000 // Two potential claims

	// Generate a CSV file with the test data
	claimsPath, err := claimtestutils.GenerateClaimsCSVFile([]claimtestutils.ClaimCSVRecord{
		{OldAddress: testData.OldAddress, Amount: claimAmount},
		{OldAddress: "lumc4mple2ddress000000000000000000000000", Amount: claimAmount}, // Second address that won't be claimed
	})
	require.NoError(t, err)

	// Ensure the file is cleaned up after the test
	t.Cleanup(func() {
		claimtestutils.CleanupClaimsCSVFile(claimsPath)
	})

	// Set a short claim period (15 seconds)
	claimEndTime := time.Now().Add(15 * time.Second).Unix()

	// Modify genesis to set up test conditions
	sut.ModifyGenesisJSON(t, func(genesis []byte) []byte {
		state := genesis
		var err error

		// Set total claimable amount
		state, err = sjson.SetRawBytes(state, "app_state.claim.total_claimable_amount",
			[]byte(fmt.Sprintf("%d", totalClaimableAmount)))
		require.NoError(t, err)

		// Enable claims and set end time
		state, err = sjson.SetRawBytes(state, "app_state.claim.params.enable_claims", []byte("true"))
		require.NoError(t, err)
		state, err = sjson.SetRawBytes(state, "app_state.claim.params.claim_end_time",
			[]byte(fmt.Sprintf("%d", claimEndTime)))
		require.NoError(t, err)

		return state
	})

	// Start the chain
	sut.StartChain(t, fmt.Sprintf("--%s=%s", claimtypes.FlagClaimsPath, claimsPath))
	cli := NewLumeradCLI(t, sut, true)

	// Get claim module account address
	moduleAcctResp := cli.CustomQuery("q", "auth", "module-account", "claim")
	moduleAddr := gjson.Get(moduleAcctResp, "account.value.address").String()
	require.NotEmpty(t, moduleAddr, "claim module address should not be empty")

	// Verify initial module balance
	initialModuleBalance := cli.QueryBalance(moduleAddr, claimtypes.DefaultClaimsDenom)
	require.Equal(t, int64(totalClaimableAmount), initialModuleBalance,
		"Initial module balance should match total claimable amount")

	// Process claim
	registerCmd := []string{
		"tx", "claim", "claim",
		testData.OldAddress,
		testData.NewAddress,
		testData.PubKey,
		testData.Signature,
		"--from", "node0",
	}
	resp := cli.CustomCommand(registerCmd...)
	RequireTxSuccess(t, resp)

	// Verify user received funds
	userBalance := cli.QueryBalance(testData.NewAddress, claimtypes.DefaultClaimsDenom)
	require.Equal(t, int64(1_000_000), userBalance, "User should receive claim amount")

	// Verify module balance decreased by claim amount
	midModuleBalance := cli.QueryBalance(moduleAddr, claimtypes.DefaultClaimsDenom)
	require.Equal(t, int64(1_000_000), midModuleBalance,
		"Module balance should be reduced by claimed amount")

	// Wait for claim period to end
	t.Log("Waiting for claim period to end...")
	time.Sleep(time.Until(time.Unix(claimEndTime, 0)) + 10*time.Second)

	// Wait additional blocks for EndBlocker to process
	sut.AwaitNextBlock(t)
	sut.AwaitNextBlock(t)
	sut.AwaitNextBlock(t)

	// Verify claims are disabled in params
	paramsResp := cli.CustomQuery("q", "claim", "params")
	enableClaims := gjson.Get(paramsResp, "params.enable_claims").Bool()
	require.False(t, enableClaims, "Claims should be disabled after period end")

	// KEY ASSERTION: Verify all remaining tokens were burned
	finalModuleBalance := cli.QueryBalance(moduleAddr, claimtypes.DefaultClaimsDenom)
	require.Equal(t, int64(0), finalModuleBalance,
		"Module balance should be zero after burn")
}
