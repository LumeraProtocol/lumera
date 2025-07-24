//go:build system_test

package system

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// Helper function to create a delayed vesting account
func createDelayedVestingAccount(t *testing.T, cli *LumeradCli, keyName string, amount string, delayMonths int) string {
	// Create the key first
	address := cli.AddKey(keyName)

	// Calculate end time (current time + delay)
	endTime := time.Now().AddDate(0, delayMonths, 0).Unix()

	// Create delayed vesting account
	createCmd := []string{
		"tx", "vesting", "create-vesting-account",
		address,                        // to_address
		amount + "stake",               // amount
		strconv.FormatInt(endTime, 10), // end_time
		"--delayed",                    // make it delayed vesting
		"--from", "node0",
	}

	resp := cli.CustomCommand(createCmd...)
	RequireTxSuccess(t, resp)

	// Wait for account creation to be processed
	sut.AwaitNextBlock(t)

	// Fund the account with some liquid tokens for transaction fees
	cli.FundAddress(address, "1000000stake")

	return address
}

// Helper function to create a permanently locked account
func createPermanentlyLockedAccount(t *testing.T, cli *LumeradCli, keyName string, amount string) string {
	// Create the key first
	address := cli.AddKey(keyName)

	// Create permanently locked account
	createCmd := []string{
		"tx", "vesting", "create-permanent-locked-account",
		address,          // to_address
		amount + "stake", // amount
		"--from", "node0",
	}

	resp := cli.CustomCommand(createCmd...)
	RequireTxSuccess(t, resp)

	// Wait for account creation to be processed
	sut.AwaitNextBlock(t)

	// Fund the account with some liquid tokens for transaction fees
	cli.FundAddress(address, "1000000stake")

	return address
}

// Helper function to verify vesting account type
func verifyVestingAccountType(t *testing.T, cli *LumeradCli, address string, expectedType string) {
	account := cli.GetAccount(address)
	require.NotNil(t, account, "account not found")

	actualType := gjson.Get(account, "account.type").String()
	require.Equal(t, expectedType, actualType, "account type mismatch")
}

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
		{
			name: "register_with_delayed_vesting_supernode_account",
			setupFn: func(t *testing.T, cli *LumeradCli) string {
				// Create delayed vesting account for validator operator
				return createDelayedVestingAccount(t, cli, "vesting_validator", "150000000", 12)
			},
			minimumStake: "100000000",
			additionalSetupFn: func(t *testing.T, cli *LumeradCli, valAddr string, supernodeAccount string) {
				// Delegate from the vesting account to the validator
				delegateCmd := []string{
					"tx", "staking", "delegate",
					valAddr,          // validator address
					"120000000stake", // delegation amount
					"--from", "vesting_validator",
				}
				resp := cli.CustomCommand(delegateCmd...)
				RequireTxSuccess(t, resp)
				sut.AwaitNextBlock(t)
			},
			additionalValidateFn: func(t *testing.T, cli *LumeradCli, valAddr string, supernodeAccount string, accountAddr string) {
				// Verify the supernode account is a delayed vesting account
				verifyVestingAccountType(t, cli, supernodeAccount, "/cosmos.vesting.v1beta1.DelayedVestingAccount")

				// Verify delegation exists from the vesting account
				delegation := cli.CustomQuery("query", "staking", "delegation", supernodeAccount, valAddr)
				delegationAmount := gjson.Get(delegation, "delegation_response.balance.amount").String()
				require.NotEmpty(t, delegationAmount, "Delegation should exist")

				// Parse and verify the delegation amount meets minimum requirement
				delegationAmountInt, err := strconv.ParseInt(delegationAmount, 10, 64)
				require.NoError(t, err, "Failed to parse delegation amount")
				require.GreaterOrEqual(t, delegationAmountInt, int64(100000000), "Delegation should meet minimum requirement")
			},
		},
		{
			name: "register_with_permanently_locked_supernode_account",
			setupFn: func(t *testing.T, cli *LumeradCli) string {
				// Create permanently locked account for validator operator
				return createPermanentlyLockedAccount(t, cli, "locked_validator", "200000000")
			},
			minimumStake: "100000000",
			additionalSetupFn: func(t *testing.T, cli *LumeradCli, valAddr string, supernodeAccount string) {
				// Delegate from the permanently locked account to the validator
				delegateCmd := []string{
					"tx", "staking", "delegate",
					valAddr,          // validator address
					"150000000stake", // delegation amount
					"--from", "locked_validator",
				}
				resp := cli.CustomCommand(delegateCmd...)
				RequireTxSuccess(t, resp)
				sut.AwaitNextBlock(t)
			},
			additionalValidateFn: func(t *testing.T, cli *LumeradCli, valAddr string, supernodeAccount string, accountAddr string) {
				// Verify the account is a permanently locked vesting account
				verifyVestingAccountType(t, cli, supernodeAccount, "/cosmos.vesting.v1beta1.PermanentLockedAccount")

				// Verify delegation exists and is sufficient
				delegation := cli.CustomQuery("query", "staking", "delegation", supernodeAccount, valAddr)
				delegationAmountStr := gjson.Get(delegation, "delegation_response.balance.amount").String()
				delegationAmount, err := strconv.ParseInt(delegationAmountStr, 10, 64)
				require.NoError(t, err, "Failed to parse delegation amount")
				require.GreaterOrEqual(t, delegationAmount, int64(100000000), "Delegation should meet minimum requirement")
			},
		},
		{
			name: "register_with_mixed_self_and_supernode_delegations",
			setupFn: func(t *testing.T, cli *LumeradCli) string {
				// Create delayed vesting account for supernode registration
				return createDelayedVestingAccount(t, cli, "mixed_supernode", "100000000", 3)
			},
			minimumStake: "150000000", // Neither self nor SN delegation alone will meet this
			additionalSetupFn: func(t *testing.T, cli *LumeradCli, valAddr string, supernodeAccount string) {
				// Fund validator operator account (node0) for additional self-delegation
				validatorAddr := cli.GetKeyAddr("node0")
				cli.FundAddress(validatorAddr, "100000000stake")

				// Add self-delegation from validator operator account
				delegateCmd1 := []string{
					"tx", "staking", "delegate",
					valAddr,         // validator address
					"70000000stake", // partial amount - self delegation
					"--from", "node0",
				}
				resp1 := cli.CustomCommand(delegateCmd1...)
				RequireTxSuccess(t, resp1)

				// Delegate from supernode vesting account
				delegateCmd2 := []string{
					"tx", "staking", "delegate",
					valAddr,         // validator address
					"90000000stake", // partial amount - together should exceed minimum
					"--from", "mixed_supernode",
				}
				resp2 := cli.CustomCommand(delegateCmd2...)
				RequireTxSuccess(t, resp2)
				sut.AwaitNextBlock(t)
			},
			additionalValidateFn: func(t *testing.T, cli *LumeradCli, valAddr string, supernodeAccount string, accountAddr string) {
				// Verify supernode account is vesting account
				verifyVestingAccountType(t, cli, supernodeAccount, "/cosmos.vesting.v1beta1.DelayedVestingAccount")

				// Verify self-delegation from validator operator account
				selfDelegation := cli.CustomQuery("query", "staking", "delegation", accountAddr, valAddr)
				selfAmount, _ := strconv.ParseInt(gjson.Get(selfDelegation, "delegation_response.balance.amount").String(), 10, 64)

				// Verify supernode delegation
				supernodeDelegation := cli.CustomQuery("query", "staking", "delegation", supernodeAccount, valAddr)
				supernodeAmount, _ := strconv.ParseInt(gjson.Get(supernodeDelegation, "delegation_response.balance.amount").String(), 10, 64)

				// Verify combined delegations meet the requirement
				totalDelegation := selfAmount + supernodeAmount
				require.GreaterOrEqual(t, totalDelegation, int64(150000000), "Combined self and supernode delegations should meet minimum requirement")
				require.Greater(t, selfAmount, int64(0), "Self delegation should be greater than 0")
				require.Greater(t, supernodeAmount, int64(0), "Supernode delegation should be greater than 0")
			},
		},
		{
			name: "register_with_self_delegation_only",
			setupFn: func(t *testing.T, cli *LumeradCli) string {
				return cli.GetKeyAddr("node0") // Use validator account as supernode account
			},
			minimumStake: "50000000", // Set minimum that can be met with additional self-delegation only
			additionalSetupFn: func(t *testing.T, cli *LumeradCli, valAddr string, supernodeAccount string) {
				// Fund validator operator account for additional self-delegation
				validatorAddr := cli.GetKeyAddr("node0")
				cli.FundAddress(validatorAddr, "100000000stake")

				// Add additional self-delegation to meet minimum requirement
				delegateCmd := []string{
					"tx", "staking", "delegate",
					valAddr,         // validator address
					"60000000stake", // enough to meet minimum with existing self-delegation
					"--from", "node0",
				}
				resp := cli.CustomCommand(delegateCmd...)
				RequireTxSuccess(t, resp)
				sut.AwaitNextBlock(t)
			},
			additionalValidateFn: func(t *testing.T, cli *LumeradCli, valAddr string, supernodeAccount string, accountAddr string) {
				// Verify only self-delegation exists and meets requirement
				selfDelegation := cli.CustomQuery("query", "staking", "delegation", accountAddr, valAddr)
				selfDelegationAmountStr := gjson.Get(selfDelegation, "delegation_response.balance.amount").String()
				selfDelegationAmount, err := strconv.ParseInt(selfDelegationAmountStr, 10, 64)
				require.NoError(t, err, "Failed to parse self delegation amount")
				require.GreaterOrEqual(t, selfDelegationAmount, int64(50000000), "Self delegation should meet minimum requirement")

				// Verify supernode account is same as validator account (no separate supernode delegation)
				require.Equal(t, accountAddr, supernodeAccount, "Supernode account should be same as validator account")
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
			name:         "insufficient_stake",
			minimumStake: "100000000000", // Set very high minimum stake requirement
			setupFn: func(t *testing.T, cli *LumeradCli) (string, string, string) {
				valAddr := strings.TrimSpace(cli.Keys("keys", "show", "node0", "--bech", "val", "-a"))
				accountAddr := cli.GetKeyAddr("node0")
				return valAddr, accountAddr, "node0"
			},
			expectedError: "does not meet minimum stake requirement",
		},
		{
			name:         "validator_not_found",
			minimumStake: "1000000",
			setupFn: func(t *testing.T, cli *LumeradCli) (string, string, string) {
				keyName := "non_validator"
				accountAddr := cli.AddKey(keyName)
				cli.FundAddress(accountAddr, "1000000stake")
				nonValAddr := strings.TrimSpace(cli.Keys("keys", "show", keyName, "--bech", "val", "-a"))
				return nonValAddr, accountAddr, keyName
			},
			expectedError: "validator not found",
		},
		{
			name:         "duplicate_registration",
			minimumStake: "1000000",
			setupFn: func(t *testing.T, cli *LumeradCli) (string, string, string) {
				valAddr := strings.TrimSpace(cli.Keys("keys", "show", "node0", "--bech", "val", "-a"))
				accountAddr := cli.GetKeyAddr("node0")
				return valAddr, accountAddr, "node0"
			},
			additionalSetupFn: func(t *testing.T, cli *LumeradCli, valAddr string, accountAddr string, keyName string) {
				// Register supernode first time
				registerCmd := []string{
					"tx", "supernode", "register-supernode",
					valAddr,
					"192.168.1.1",
					accountAddr,
					"--from", keyName,
				}
				resp := cli.CustomCommand(registerCmd...)
				RequireTxSuccess(t, resp)
				sut.AwaitNextBlock(t)
			},
			expectedError: "supernode already exists",
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
			
			// Use standard IP address
			ipAddress := "192.168.1.1"
			
			registerResp := cli.CustomCommand(
				"tx", "supernode", "register-supernode",
				valAddr,     // validator address
				ipAddress,   // IP address
				accountAddr, // supernode account
				"--from", keyName,
			)
			t.Logf("Registration response: %s", registerResp)

			// Verify transaction failed with correct error
			t.Log("Verifying transaction failure")
			RequireTxFailure(t, registerResp, tc.expectedError)

			// Verify no supernode was registered (except for duplicate_registration case)
			if tc.name != "duplicate_registration" {
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
			}
		})
	}
}
