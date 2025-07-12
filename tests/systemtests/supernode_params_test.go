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

func TestSupernodeUpdateParamsProposal(t *testing.T) {
	// Initialize and reset chain
	sut.ResetChain(t)

	// Set initial parameters in genesis
	sut.ModifyGenesisJSON(t,
		// Set shorter voting period for faster test execution
		SetGovVotingPeriod(t, 10*time.Second),
		// Set initial supernode parameters
		func(genesis []byte) []byte {
			state, err := sjson.SetRawBytes(genesis, "app_state.supernode.params", []byte(`{
				"minimum_stake_for_sn": {"denom":"ulume","amount":"1000000"},
				"reporting_threshold": "95",
				"slashing_threshold": "80",
				"metrics_thresholds": "cpu:80,memory:80,storage:80",
				"evidence_retention_period": "168h",
				"slashing_fraction": "0.1",
				"inactivity_penalty_period": "24h"
			}`))
			require.NoError(t, err)
			return state
		},
	)

	// Generate default CSV test file for claims
	claimsPath, err := claimtestutils.GenerateDefaultClaimingTestData()
	require.NoError(t, err)

	// Ensure the file is cleaned up after the test
	t.Cleanup(func() {
		claimtestutils.CleanupClaimsCSVFile(claimsPath)
	})

	// Start the chain
	sut.StartChain(t, fmt.Sprintf("--%s=%s", claimtypes.FlagClaimsPath, claimsPath))

	// Create CLI helper
	cli := NewLumeradCLI(t, sut, true)

	// Get and verify initial parameters
	initialParams := cli.CustomQuery("q", "supernode", "params")
	t.Logf("Initial params: %s", initialParams)

	// Verify the coin structure for minimum_stake_for_sn
	require.Equal(t, "ulume", gjson.Get(initialParams, "params.minimum_stake_for_sn.denom").String(), "initial minimum_stake_for_sn denom should be ulume")
	require.Equal(t, "1000000", gjson.Get(initialParams, "params.minimum_stake_for_sn.amount").String(), "initial minimum_stake_for_sn amount should be 1000000")

	require.Equal(t, "95", gjson.Get(initialParams, "params.reporting_threshold").String(), "initial reporting_threshold should be 95")
	require.Equal(t, "80", gjson.Get(initialParams, "params.slashing_threshold").String(), "initial slashing_threshold should be 80")
	require.Equal(t, "cpu:80,memory:80,storage:80", gjson.Get(initialParams, "params.metrics_thresholds").String(), "initial metrics_thresholds should match")
	require.Equal(t, "168h", gjson.Get(initialParams, "params.evidence_retention_period").String(), "initial evidence_retention_period should be 168h")
	require.Equal(t, "0.1", gjson.Get(initialParams, "params.slashing_fraction").String(), "initial slashing_fraction should be 0.1")
	require.Equal(t, "24h", gjson.Get(initialParams, "params.inactivity_penalty_period").String(), "initial inactivity_penalty_period should be 24h")

	// Get gov module account address
	govAcctResp := cli.CustomQuery("q", "auth", "module-account", "gov")
	t.Logf("Gov account response: %s", govAcctResp)
	govAddr := gjson.Get(govAcctResp, "account.value.address").String()
	require.NotEmpty(t, govAddr, "gov module address should not be empty")

	// Create governance proposal to update parameters
	proposalJson := fmt.Sprintf(`{
		"messages": [{
			"@type": "/lumera.supernode.v1.MsgUpdateParams",
			"authority": "%s",
			"params": {
				"minimum_stake_for_sn": {"denom":"ulume","amount":"2000000"},
				"reporting_threshold": "90",
				"slashing_threshold": "75",
				"metrics_thresholds": "cpu:85,memory:85,storage:85",
				"evidence_retention_period": "336h",
				"slashing_fraction": "0.2",
				"inactivity_penalty_period": "48h"
			}
		}],
		"deposit": "100000000stake",
		"metadata": "ipfs://CID",
		"title": "Update Supernode Parameters",
		"summary": "Update supernode module parameters with new values"
	}`, govAddr)

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
	updatedParams := cli.CustomQuery("q", "supernode", "params")
	t.Logf("Updated params: %s", updatedParams)

	// Verify the updated coin structure for minimum_stake_for_sn
	require.Equal(t, "ulume", gjson.Get(updatedParams, "params.minimum_stake_for_sn.denom").String(), "minimum_stake_for_sn denom should be ulume")
	require.Equal(t, "2000000", gjson.Get(updatedParams, "params.minimum_stake_for_sn.amount").String(), "minimum_stake_for_sn amount should be 2000000")

	require.Equal(t, "90", gjson.Get(updatedParams, "params.reporting_threshold").String(), "reporting_threshold should be 90")
	require.Equal(t, "75", gjson.Get(updatedParams, "params.slashing_threshold").String(), "slashing_threshold should be 75")
	require.Equal(t, "cpu:85,memory:85,storage:85", gjson.Get(updatedParams, "params.metrics_thresholds").String(), "metrics_thresholds should match new values")
	require.Equal(t, "336h", gjson.Get(updatedParams, "params.evidence_retention_period").String(), "evidence_retention_period should be 336h")
	require.Equal(t, "0.2", gjson.Get(updatedParams, "params.slashing_fraction").String(), "slashing_fraction should be 0.2")
	require.Equal(t, "48h", gjson.Get(updatedParams, "params.inactivity_penalty_period").String(), "inactivity_penalty_period should be 48h")
}
