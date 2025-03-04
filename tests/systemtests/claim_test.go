//go:build system_test

package system

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	claimtestutils "github.com/LumeraProtocol/lumera/x/claim/testutils"
	claimtypes "github.com/LumeraProtocol/lumera/x/claim/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

func TestClaimsSystem(t *testing.T) {
	testCases := []struct {
		name            string
		balanceToClaim  string
		setupFn         func(t *testing.T) (claimtestutils.TestData, string)
		modifyGenesis   func(genesis []byte) []byte
		expectSuccess   bool
		expectedError   string
		waitBeforeClaim bool
		claimAttempts   int // number of times to attempt the claim in the same block
	}{
		{
			name:           "successful_claim",
			balanceToClaim: "1000000",
			setupFn: func(t *testing.T) (claimtestutils.TestData, string) {
				testData, err := claimtestutils.GenerateClaimingTestData()
				require.NoError(t, err)
				return testData, testData.OldAddress
			},
			modifyGenesis: func(genesis []byte) []byte {
				state, err := sjson.SetRawBytes(genesis, "app_state.claim.total_claimable_amount", []byte("1000000"))
				require.NoError(t, err)
				return state
			},
			expectSuccess:   true,
			waitBeforeClaim: false,
			claimAttempts:   1,
		},
		{
			// we remove zero balances from csv file by default
			name:           "claim_with_zero_balance",
			balanceToClaim: "0",
			setupFn: func(t *testing.T) (claimtestutils.TestData, string) {
				testData, err := claimtestutils.GenerateClaimingTestData()
				require.NoError(t, err)
				return testData, testData.OldAddress
			},
			modifyGenesis: func(genesis []byte) []byte {
				state, err := sjson.SetRawBytes(genesis, "app_state.claim.total_claimable_amount", []byte("0"))
				require.NoError(t, err)
				return state
			},
			expectSuccess:   false,
			expectedError:   "claim not found",
			waitBeforeClaim: false,
			claimAttempts:   1,
		},
		{
			name:           "claims_disabled",
			balanceToClaim: "500000",
			setupFn: func(t *testing.T) (claimtestutils.TestData, string) {
				testData, err := claimtestutils.GenerateClaimingTestData()
				require.NoError(t, err)
				return testData, testData.OldAddress
			},
			modifyGenesis: func(genesis []byte) []byte {
				state := genesis
				var err error

				state, err = sjson.SetRawBytes(state, "app_state.claim.total_claimable_amount", []byte("500000"))
				require.NoError(t, err)

				state, err = sjson.SetRawBytes(state, "app_state.claim.params.enable_claims", []byte("false"))
				require.NoError(t, err)

				return state
			},
			expectSuccess: false,
			expectedError: "claim is disabled",
			claimAttempts: 1,
		},
		{
			name:           "claim_period_expired",
			balanceToClaim: "500000",
			setupFn: func(t *testing.T) (claimtestutils.TestData, string) {
				testData, err := claimtestutils.GenerateClaimingTestData()
				require.NoError(t, err)
				return testData, testData.OldAddress
			},
			modifyGenesis: func(genesis []byte) []byte {
				state := genesis
				var err error

				state, err = sjson.SetRawBytes(state, "app_state.claim.total_claimable_amount", []byte("500000"))
				require.NoError(t, err)

				state, err = sjson.SetRawBytes(state, "app_state.claim.params.enable_claims", []byte("true"))
				require.NoError(t, err)

				endTime := time.Now().Add(10 * time.Second).Unix()
				state, err = sjson.SetRawBytes(state, "app_state.claim.params.claim_end_time", []byte(strconv.FormatInt(endTime, 10)))
				require.NoError(t, err)

				return state
			},
			expectSuccess:   false,
			expectedError:   "claim is disabled",
			waitBeforeClaim: true,
			claimAttempts:   1,
		},
		{
			name:           "duplicate_claim",
			balanceToClaim: "500000",
			setupFn: func(t *testing.T) (claimtestutils.TestData, string) {
				testData, err := claimtestutils.GenerateClaimingTestData()
				require.NoError(t, err)
				return testData, testData.OldAddress
			},
			modifyGenesis: func(genesis []byte) []byte {
				state := genesis
				var err error

				// Set total claimable amount
				state, err = sjson.SetRawBytes(state, "app_state.claim.total_claimable_amount", []byte("500000"))
				require.NoError(t, err)

				// Set claims per block to 1
				state, err = sjson.SetRawBytes(state, "app_state.claim.params.max_claims_per_block", []byte("10"))
				require.NoError(t, err)

				// Enable claims
				state, err = sjson.SetRawBytes(state, "app_state.claim.params.enable_claims", []byte("true"))
				require.NoError(t, err)

				// Set reasonable claim end time
				endTime := time.Now().Add(1 * time.Hour).Unix()
				state, err = sjson.SetRawBytes(state, "app_state.claim.params.claim_end_time", []byte(strconv.FormatInt(endTime, 10)))
				require.NoError(t, err)

				return state
			},
			expectSuccess:   false,
			expectedError:   "claim already claimed",
			waitBeforeClaim: false,
			claimAttempts:   2, // Try to claim 2 times
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			sut.ResetChain(t)
			cli := NewLumeradCLI(t, sut, true)

			// Get test data and CSV address
			testData, csvAddress := tc.setupFn(t)

			// Apply custom genesis modifications
			sut.ModifyGenesisJSON(t, tc.modifyGenesis)

			// Create the CSV file in homedir
			homedir, err := os.UserHomeDir()
			require.NoError(t, err)
			csvPath := homedir + "/claims.csv"
			err = os.WriteFile(csvPath, []byte(csvAddress+","+tc.balanceToClaim+"\n"), 0644)
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = os.Remove(csvPath)
			})

			// Start the chain with modified genesis
			sut.StartChain(t)

			// Wait when needed
			if tc.waitBeforeClaim {
				t.Log("Waiting for claim period to expire...")
				time.Sleep(11 * time.Second)
			}

			var lastResp string
			// Register claim multiple times if specified
			for i := 0; i < tc.claimAttempts; i++ {
				registerCmd := []string{
					"tx", "claim", "claim",
					testData.OldAddress, // Old address
					testData.NewAddress, // New address
					testData.PubKey,     // PubKey
					testData.Signature,  // Signature
					"--from", "node0",
				}

				lastResp = cli.CustomCommand(registerCmd...)

				// For multiple attempts, only the first one should succeed
				if i == 0 && tc.expectSuccess {
					RequireTxSuccess(t, lastResp)
				}
			}

			// Validate the final response
			if tc.expectSuccess {
				RequireTxSuccess(t, lastResp)

				// Get txhash and query transaction
				txHash := gjson.Get(lastResp, "txhash").String()
				require.NotEmpty(t, txHash, "txhash not found in response")

				txResp := cli.CustomQuery("q", "tx", txHash)
				require.NotEmpty(t, txResp)

				// Verify claim_processed event
				events := gjson.Get(txResp, "events")
				require.True(t, events.Exists())

				var foundClaimEvent bool
				for _, event := range events.Array() {
					if event.Get("type").String() == "claim_processed" {
						foundClaimEvent = true
						attrs := event.Get("attributes").Array()
						for _, attr := range attrs {
							key := attr.Get("key").String()
							value := attr.Get("value").String()
							switch key {
							case "module":
								require.Equal(t, "claim", value)
							case "amount":
								require.Equal(t, tc.balanceToClaim+claimtypes.DefaultClaimsDenom, value)
							case "old_address":
								require.Equal(t, testData.OldAddress, value)
							case "new_address":
								require.Equal(t, testData.NewAddress, value)
							}
						}
					}
				}
				require.True(t, foundClaimEvent, "claim_processed event not found")

				// Check if the amount has been transferred to the new address
				bal := cli.QueryBalance(testData.NewAddress, claimtypes.DefaultClaimsDenom)
				expectedBal, err := strconv.ParseInt(tc.balanceToClaim, 10, 64)
				require.NoError(t, err)
				require.Equal(t, expectedBal, bal)

				// Verify the amount was deducted from the claims account
				claimsModuleAddr := authtypes.NewModuleAddress(claimtypes.ModuleName).String()
				claimsAccBal := cli.QueryBalance(claimsModuleAddr, claimtypes.DefaultClaimsDenom)
				expectedClaimsAccBal := cli.QueryTotalSupply(claimtypes.DefaultClaimsDenom) - expectedBal
				require.Equal(t, expectedClaimsAccBal, claimsAccBal, "claims account balance incorrect after claim")
			} else {
				RequireTxFailure(t, lastResp, tc.expectedError)
			}
		})
	}
}
