//go:build system_test

package system

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Voting Period is set to 10 seconds for faster test execution by default
func TestClaimsUpdateParamsProposal(t *testing.T) {
	// Initialize and reset chain
	sut.ResetChain(t)

	// Set initial parameters in genesis
	initialEndTime := time.Now().Add(48 * time.Hour).Unix()
	sut.ModifyGenesisJSON(t,
		// Set shorter voting period for faster test execution
		SetGovVotingPeriod(t, 10*time.Second),
		// Set initial claim parameters
		func(genesis []byte) []byte {
			state, err := sjson.SetRawBytes(genesis, "app_state.claim.params", []byte(fmt.Sprintf(`{
				"enable_claims": true,
				"claim_end_time": "%d",
				"max_claims_per_block": "75"
			}`, initialEndTime)))
			require.NoError(t, err)
			return state
		},
	)

	// Start the chain
	sut.StartChain(t)

	// Create CLI helper
	cli := NewPasteldCLI(t, sut, true)

	// Get and verify initial parameters
	initialParams := cli.CustomQuery("q", "claim", "params")
	t.Logf("Initial params: %s", initialParams)

	require.True(t, gjson.Get(initialParams, "params.enable_claims").Bool(), "initial enable_claims should be true")
	require.Equal(t, "75", gjson.Get(initialParams, "params.max_claims_per_block").String(), "initial max_claims_per_block should be 75")
	require.Equal(t, fmt.Sprintf("%d", initialEndTime), gjson.Get(initialParams, "params.claim_end_time").String(), "initial claim_end_time should match")

	// Get gov module account address
	govAcctResp := cli.CustomQuery("q", "auth", "module-account", "gov")
	t.Logf("Gov account response: %s", govAcctResp)
	govAddr := gjson.Get(govAcctResp, "account.value.address").String()
	require.NotEmpty(t, govAddr, "gov module address should not be empty")

	// Create governance proposal to update parameters
	proposalJson := fmt.Sprintf(`{
		"messages": [{
			"@type": "/pastel.claim.MsgUpdateParams",
			"authority": "%s",
			"params": {
				"enable_claims": false,
				"claim_end_time": %d,
				"max_claims_per_block": 50
			}
		}],
		"deposit": "100000000stake",
		"metadata": "ipfs://CID",
		"title": "Update Claim Parameters",
		"summary": "Update claims module parameters with new values"
	}`, govAddr, time.Now().Add(72*time.Hour).Unix())

	// Submit proposal and have all validators vote yes
	proposalID := cli.SubmitAndVoteGovProposal(proposalJson)
	require.NotEmpty(t, proposalID)

	// Wait for proposal to be executed
	var proposalPassed bool
	for i := 0; i < 10; i++ {
		sut.AwaitNextBlock(t)
		status := cli.CustomQuery("q", "gov", "proposal", proposalID)
		if gjson.Get(status, "proposal.status").String() == "PROPOSAL_STATUS_PASSED" {
			proposalPassed = true
			break
		}
	}
	require.True(t, proposalPassed, "proposal did not pass")

	// Query and verify updated parameters
	updatedParams := cli.CustomQuery("q", "claim", "params")
	t.Logf("Updated params: %s", updatedParams)

	require.False(t, gjson.Get(updatedParams, "params.enable_claims").Bool(), "enable_claims should be false")
	require.Equal(t, "50", gjson.Get(updatedParams, "params.max_claims_per_block").String(), "max_claims_per_block should be 50")

	// The end time would be approximately 72 hours from when the proposal passed
	updatedEndTime := gjson.Get(updatedParams, "params.claim_end_time").Int()
	expectedEndTime := time.Now().Add(72 * time.Hour).Unix()
	require.InDelta(t, expectedEndTime, updatedEndTime, float64(time.Hour.Seconds()),
		"claim_end_time should be approximately 72 hours from now")
}
