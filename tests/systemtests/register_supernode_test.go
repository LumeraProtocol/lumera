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
		name                 string
		setupFn              func(t *testing.T, cli *LumeradCli) string
		minimumStake         string
		additionalSetupFn    func(t *testing.T, cli *LumeradCli, valAddr string, supernodeAccount string)
		additionalValidateFn func(t *testing.T, cli *LumeradCli, valAddr string, supernodeAccount string, accountAddr string)
	}{
		{
			name: "register_with_validator_account",
			setupFn: func(t *testing.T, cli *LumeradCli) string {
				return cli.GetKeyAddr("node0") // return validator account
			},
			minimumStake: "1000000",
		},
		{
			name: "register_with_new_account",
			setupFn: func(t *testing.T, cli *LumeradCli) string {
				return cli.AddKey("supernode_account") // create and return new account
			},
			minimumStake: "1000000",
		},
		{
			name: "register_with_insufficient_self_delegation_but_sufficient_supernode_delegation",
			setupFn: func(t *testing.T, cli *LumeradCli) string {
				return cli.AddKey("supernode_account") // create and return new account
			},
			minimumStake: "100000000", // Set high minimum stake that exceeds self-delegation
			additionalSetupFn: func(t *testing.T, cli *LumeradCli, valAddr string, supernodeAccount string) {
				// Fund the supernode account
				cli.FundAddress(supernodeAccount, "200000000stake")

				// Delegate from supernode account to validator to meet the minimum stake requirement
				delegateCmd := []string{
					"tx", "staking", "delegate",
					valAddr,          // validator address
					"150000000stake", // delegation amount (more than minimum - self delegation)
					"--from", "supernode_account",
				}
				resp := cli.CustomCommand(delegateCmd...)
				RequireTxSuccess(t, resp)

				// Wait for delegation to be processed
				sut.AwaitNextBlock(t)
			},
			additionalValidateFn: func(t *testing.T, cli *LumeradCli, valAddr string, supernodeAccount string, accountAddr string) {
				// Check supernode delegation
				supernodeDelegation := cli.CustomQuery("query", "staking", "delegation", supernodeAccount, valAddr)
				t.Logf("Supernode delegation: %s", supernodeDelegation)

				// Parse and verify delegation amount
				delegationAmountStr := gjson.Get(supernodeDelegation, "delegation_response.balance.amount").String()
				delegationAmount, err := strconv.ParseInt(delegationAmountStr, 10, 64)
				require.NoError(t, err, "Failed to parse supernode delegation amount")

				// Verify that supernode delegation exists and has tokens
				require.Greater(t, delegationAmount, int64(0), "Supernode delegation amount should be greater than 0")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			minimumStake := tc.minimumStake
			if minimumStake == "" {
				minimumStake = "1000000"
			}

			// Initialize and reset chain
			sut.ResetChain(t)

			// 1. Set minimum supernode stake in genesis
			sut.ModifyGenesisJSON(t, func(genesis []byte) []byte {
				// Update the supernode module params to set minimum stake as a Coin
				coinJSON := `{"denom":"stake","amount":"` + minimumStake + `"}`
				state, err := sjson.SetRawBytes(genesis, "app_state.supernode.params.minimum_stake_for_sn", []byte(coinJSON))
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

			// Parse and verify delegation amount
			delegationAmountStr := gjson.Get(initialDelegation, "delegation_response.balance.amount").String()
			_, err := strconv.ParseInt(delegationAmountStr, 10, 64)
			require.NoError(t, err, "Failed to parse delegation amount")

			_, err = strconv.ParseInt(minimumStake, 10, 64)
			require.NoError(t, err, "Failed to parse minimum stake")

			// Run additional setup if provided
			if tc.additionalSetupFn != nil {
				tc.additionalSetupFn(t, cli, valAddr, supernodeAccount)
			}

			// Register supernode
			registerCmd := []string{
				"tx", "supernode", "register-supernode",
				valAddr,          // validator address
				"192.168.1.1",    // IP address
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

			// Run additional validation if provided
			if tc.additionalValidateFn != nil {
				tc.additionalValidateFn(t, cli, valAddr, supernodeAccount, accountAddr)
			}
		})
	}
}

func TestSupernodeRegistrationFailures(t *testing.T) {
	testCases := []struct {
		name              string
		minimumStake      string
		setupFn           func(t *testing.T, cli *LumeradCli) (string, string, string) // returns (valAddr, accountAddr, keyName)
		expectedError     string
		additionalSetupFn func(t *testing.T, cli *LumeradCli, valAddr string, accountAddr string, keyName string)
	}{
		{
			name:         "insufficient_self_stake",
			minimumStake: "100000000000", // Set very high minimum stake requirement
			setupFn: func(t *testing.T, cli *LumeradCli) (string, string, string) {
				valAddr := strings.TrimSpace(cli.Keys("keys", "show", "node0", "--bech", "val", "-a"))
				accountAddr := cli.GetKeyAddr("node0")
				return valAddr, accountAddr, "node0"
			},
			expectedError: "does not meet minimum stake requirement",
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
		{
			name:         "insufficient_self_stake_and_insufficient_supernode_delegation",
			minimumStake: "100000000", // Set high minimum stake requirement
			setupFn: func(t *testing.T, cli *LumeradCli) (string, string, string) {
				valAddr := strings.TrimSpace(cli.Keys("keys", "show", "node0", "--bech", "val", "-a"))
				accountAddr := cli.GetKeyAddr("node0")

				// Create a supernode account
				supernodeKeyName := "supernode_insufficient"
				cli.AddKey(supernodeKeyName)

				return valAddr, accountAddr, "node0"
			},
			additionalSetupFn: func(t *testing.T, cli *LumeradCli, valAddr string, accountAddr string, keyName string) {
				// Get supernode account address
				supernodeAccount := cli.GetKeyAddr("supernode_insufficient")

				// Fund the supernode account with insufficient amount
				cli.FundAddress(supernodeAccount, "10000000stake")

				// Delegate from supernode account to validator, but not enough to meet the minimum stake
				delegateCmd := []string{
					"tx", "staking", "delegate",
					valAddr,        // validator address
					"5000000stake", // delegation amount (not enough to meet minimum with self-delegation)
					"--from", "supernode_insufficient",
				}
				resp := cli.CustomCommand(delegateCmd...)
				RequireTxSuccess(t, resp)

				// Wait for delegation to be processed
				sut.AwaitNextBlock(t)
			},
			expectedError: "does not meet minimum stake requirement",
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
				// Create proper Coin JSON structure
				coinJSON := `{"denom":"stake","amount":"` + tc.minimumStake + `"}`
				state, err := sjson.SetRawBytes(genesis, "app_state.supernode.params.minimum_stake_for_sn", []byte(coinJSON))
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

			// Run additional setup if provided
			if tc.additionalSetupFn != nil {
				tc.additionalSetupFn(t, cli, valAddr, accountAddr, keyName)
			}

			// Attempt to register supernode
			t.Log("Attempting to register supernode")
			var supernodeAccount string
			if tc.name == "insufficient_self_stake_and_insufficient_supernode_delegation" {
				// Use the supernode_insufficient account for this test case
				supernodeAccount = cli.GetKeyAddr("supernode_insufficient")
			} else {
				supernodeAccount = accountAddr
			}
			registerResp := cli.CustomCommand(
				"tx", "supernode", "register-supernode",
				valAddr,          // validator address
				"192.168.1.1",    // IP address
				supernodeAccount, // supernode account
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
