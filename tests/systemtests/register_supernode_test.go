//go:build system_test

package system

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

func TestSupernodeRegistrationSuccess(t *testing.T) {
	testCases := []struct {
		name    string
		setupFn func(t *testing.T, cli *LumeradCli) string
	}{
		{
			name: "register_with_validator_account",
			setupFn: func(t *testing.T, cli *LumeradCli) string {
				return cli.GetKeyAddr("node0") // return validator account
			},
		},
		{
			name: "register_with_new_account",
			setupFn: func(t *testing.T, cli *LumeradCli) string {
				return cli.AddKey("supernode_account") // create and return new account
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			const minimumStake = "1000000"

			// Initialize and reset chain
			sut.ResetChain(t)

			// 1. Set minimum supernode stake in genesis
			sut.ModifyGenesisJSON(t, func(genesis []byte) []byte {
				// Update the supernode module params to set minimum stake
				state, err := sjson.SetRawBytes(genesis, "app_state.supernode.params.minimum_stake_for_sn", []byte(`"`+minimumStake+`"`))
				require.NoError(t, err)
				return state
			})

			// Start the chain with modified genesis
			sut.StartChain(t)

			// Create CLI helper
			cli := NewLumeradCLI(t, sut, true)

			// Get validator address and create/get supernode account
			accountAddr := cli.GetKeyAddr("node0")
			valAddrOutput := cli.Keys("keys", "show", "node0", "--bech", "val", "-a")
			valAddr := strings.TrimSpace(valAddrOutput)

			// Get supernode account from setup function
			supernodeAccount := tc.setupFn(t, cli)

			t.Logf("Validator Account address: %s", accountAddr)
			t.Logf("Validator Operator address: %s", valAddr)
			t.Logf("Supernode Account address: %s", supernodeAccount)

			// Check initial self-delegation
			initialDelegation := cli.CustomQuery("query", "staking", "delegation", accountAddr, valAddr)
			t.Logf("Initial self-delegation: %s", initialDelegation)

			// Parse and verify delegation amount is greater than minimum stake
			delegationAmountStr := gjson.Get(initialDelegation, "delegation_response.balance.amount").String()
			delegationAmount, err := strconv.ParseInt(delegationAmountStr, 10, 64)
			require.NoError(t, err, "Failed to parse delegation amount")

			minStakeRequired, err := strconv.ParseInt(minimumStake, 10, 64)
			require.NoError(t, err, "Failed to parse minimum stake")

			require.Greater(t, delegationAmount, minStakeRequired,
				"Self-delegation amount (%d) must be greater than minimum stake requirement (%d)",
				delegationAmount, minStakeRequired)

			// Register supernode
			registerCmd := []string{
				"tx", "supernode", "register-supernode",
				valAddr,          // validator address
				"192.168.1.1",    // IP address
				"1.0.0",          // version
				supernodeAccount, // supernode account
				"--from", "node0",
			}

			resp := cli.CustomCommand(registerCmd...)
			RequireTxSuccess(t, resp)

			// Wait for registration to be processed
			sut.AwaitNextBlock(t)

			// Check supernode registration
			supernode := GetSuperNodeResponse(t, cli, valAddr)
			require.Equal(t, valAddr, supernode.ValidatorAddress)
			require.Equal(t, "1.0.0", supernode.Version)
			require.Equal(t, supernodeAccount, supernode.SupernodeAccount)
			require.NotEmpty(t, supernode.States)
			require.Equal(t, types.SuperNodeStateActive, supernode.States[0].State)
		})
	}
}

func TestSupernodeRegistrationFailures(t *testing.T) {
	testCases := []struct {
		name          string
		minimumStake  string
		setupFn       func(t *testing.T, cli *LumeradCli) (string, string, string) // returns (valAddr, accountAddr, keyName)
		expectedError string
	}{
		{
			name:         "insufficient_self_stake",
			minimumStake: "100000000000", // Set very high minimum stake requirement
			setupFn: func(t *testing.T, cli *LumeradCli) (string, string, string) {
				valAddr := strings.TrimSpace(cli.Keys("keys", "show", "node0", "--bech", "val", "-a"))
				accountAddr := cli.GetKeyAddr("node0")
				return valAddr, accountAddr, "node0"
			},
			expectedError: "does not meet minimum self stake requirement",
		},
		{
			name:         "non_validator_registration",
			minimumStake: "1000000", // Normal minimum stake
			setupFn: func(t *testing.T, cli *LumeradCli) (string, string, string) {
				keyName := "non_validator"
				// Create new account that is not a validator
				accountAddr := cli.AddKey(keyName)

				// Fund the account
				cli.FundAddress(accountAddr, "1000000stake")

				// Get this account's validator address format (even though it's not a validator)
				nonValAddr := strings.TrimSpace(cli.Keys("keys", "show", keyName, "--bech", "val", "-a"))

				return nonValAddr, accountAddr, keyName
			},
			expectedError: "validator does not exist",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Initialize and reset chain
			t.Logf("Test case: %s - Resetting chain", tc.name)
			sut.ResetChain(t)

			// Set minimum stake in genesis
			t.Logf("Setting minimum stake to %s", tc.minimumStake)

			sut.ModifyGenesisJSON(t, func(genesis []byte) []byte {
				state, err := sjson.SetRawBytes(genesis, "app_state.supernode.params.minimum_stake_for_sn", []byte(`"`+tc.minimumStake+`"`))
				require.NoError(t, err)

				minStakeSet := gjson.GetBytes(state, "app_state.supernode.params.minimum_stake_for_sn")
				t.Logf("Genesis minimum_stake_for_sn set to: %s", minStakeSet.String())

				return state
			})

			// Start the chain
			t.Log("Starting chain")
			sut.StartChain(t)

			// Create CLI helper
			cli := NewLumeradCLI(t, sut, true)

			// Run test-specific setup and get addresses
			valAddr, accountAddr, keyName := tc.setupFn(t, cli)
			t.Logf("Using validator address: %s", valAddr)
			t.Logf("Using account address: %s", accountAddr)
			t.Logf("Using key name: %s", keyName)

			// Attempt to register supernode
			t.Log("Attempting to register supernode")
			registerResp := cli.CustomCommand(
				"tx", "supernode", "register-supernode",
				valAddr,       // validator address
				"192.168.1.1", // IP address
				"1.0.0",       // version
				accountAddr,   // supernode account
				"--from", keyName,
			)
			t.Logf("Registration response: %s", registerResp)

			// Verify transaction failed with correct error
			t.Log("Verifying transaction failure")
			RequireTxFailure(t, registerResp, tc.expectedError)

			// Verify no supernode was registered
			t.Log("Verifying no supernode was registered")
			supernodeResp := cli.WithRunErrorsIgnored().CustomQuery(
				"query", "supernode", "get-super-node", valAddr,
			)
			t.Logf("Supernode query response: %s", supernodeResp)

			require.True(t,
				strings.Contains(supernodeResp, "not found") ||
					strings.Contains(supernodeResp, "no supernode found") ||
					strings.Contains(supernodeResp, "key not found"),
				"supernode should not be registered, got response: %s", supernodeResp)
		})
	}
}
