//go:build system_test

package system

import (
	"fmt"
	"os"
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
	claimAmount := "1000000"
	totalClaimableAmount := 2000000 // Two potential claims

	// Create the CSV file with claim data
	homedir, err := os.UserHomeDir()
	require.NoError(t, err)
	csvPath := homedir + "/claims.csv"
	csvContent := fmt.Sprintf("%s,%s\n%s,%s\n",
		testData.OldAddress, claimAmount,
		"lumc4mple2ddress000000000000000000000000", claimAmount) // Second address that won't be claimed
	err = os.WriteFile(csvPath, []byte(csvContent), 0644)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Remove(csvPath)
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
	sut.StartChain(t)
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
	require.Equal(t, int64(1000000), userBalance, "User should receive claim amount")

	// Verify module balance decreased by claim amount
	midModuleBalance := cli.QueryBalance(moduleAddr, claimtypes.DefaultClaimsDenom)
	require.Equal(t, int64(1000000), midModuleBalance,
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
