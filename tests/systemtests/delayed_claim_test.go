//go:build system_test

package system

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	claimtestutils "github.com/LumeraProtocol/lumera/x/claim/testutils"
	claimtypes "github.com/LumeraProtocol/lumera/x/claim/types"
)

func TestDelayedClaimsSystem(t *testing.T) {
	testCases := []struct {
		name            string
		balanceToClaim  uint64
		setupFn         func(t *testing.T, cli *LumeradCli) (claimtestutils.TestData, string)
		modifyGenesis   func(genesis []byte) []byte
		expectSuccess   bool
		expectedError   string
		waitBeforeClaim bool
		claimAttempts   int // number of times to attempt the claim in the same block
		tier            string
		from            string
	}{
		{
			name:           "successful_claim",
			balanceToClaim: 1_000_000,
			setupFn: func(t *testing.T, cli *LumeradCli) (claimtestutils.TestData, string) {
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
			tier:            "1",
			from:            "node0",
		},
		{
			name:           "successful_claim_from_same_address",
			balanceToClaim: 1_000_000,
			setupFn: func(t *testing.T, cli *LumeradCli) (claimtestutils.TestData, string) {
				return claimtestutils.TestData{}, ""
			},
			modifyGenesis: func(genesis []byte) []byte {
				state, err := sjson.SetRawBytes(genesis, "app_state.claim.total_claimable_amount", []byte("1000000"))
				require.NoError(t, err)
				return state
			},
			expectSuccess:   true,
			waitBeforeClaim: false,
			claimAttempts:   1,
			tier:            "1",
			from:            "test_1",
		},
		{
			// we remove zero balances from csv file by default
			name:           "claim_with_zero_balance",
			balanceToClaim: 0,
			setupFn: func(t *testing.T, cli *LumeradCli) (claimtestutils.TestData, string) {
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
			tier:            "1",
			from:            "node0",
		},
		{
			name:           "claims_disabled",
			balanceToClaim: 500_000,
			setupFn: func(t *testing.T, cli *LumeradCli) (claimtestutils.TestData, string) {
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
			tier:          "1",
			from:          "node0",
		},
		{
			name:           "claim_period_expired",
			balanceToClaim: 500_000,
			setupFn: func(t *testing.T, cli *LumeradCli) (claimtestutils.TestData, string) {
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
			tier:            "1",
			from:            "node0",
		},
		{
			name:           "duplicate_claim",
			balanceToClaim: 500_000,
			setupFn: func(t *testing.T, cli *LumeradCli) (claimtestutils.TestData, string) {
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
			tier:            "1",
			from:            "node0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			sut.ResetChain(t)
			cli := NewLumeradCLI(t, sut, true)

			// Get test data and CSV address
			testData, csvAddress := tc.setupFn(t, cli)

			// Apply custom genesis modifications
			sut.ModifyGenesisJSON(t, tc.modifyGenesis)

			var pastelAccount claimtestutils.PastelAccount
			if tc.name == "successful_claim_from_same_address" {
				var err error
				pastelAccount, err = claimtestutils.GeneratePastelAddress()
				require.NoError(t, err)
				csvAddress = pastelAccount.Address
			}

			// generate CSV file with claim data
			claimsPath, err := claimtestutils.GenerateClaimsCSVFile([]claimtestutils.ClaimCSVRecord{
				{
					OldAddress: csvAddress, 
					Amount: tc.balanceToClaim,
				},
			})
			require.NoError(t, err)

			t.Cleanup(func() {
				claimtestutils.CleanupClaimsCSVFile(claimsPath)
			})

			// Start the chain with modified genesis
			sut.StartChain(t, fmt.Sprintf("--%s=%s", claimtypes.FlagClaimsPath, claimsPath))

			// Wait when needed
			if tc.waitBeforeClaim {
				t.Log("Waiting for claim period to expire...")
				time.Sleep(11 * time.Second)
			}

			if tc.name == "successful_claim_from_same_address" {
				address := cli.AddKey("test_1")
				cli.FundAddress(address, "1stake")
				testData, err = claimtestutils.GenerateClaimingTestData2(pastelAccount, address)
				require.NoError(t, err)

				// Verify account exist and it is Base account
				baseAccount := cli.GetAccount(testData.NewAddress)
				require.NotNil(t, baseAccount, "account not found")
				baseAccountType := gjson.Get(baseAccount, "account.type").String()
				require.Equal(t, "/cosmos.auth.v1beta1.BaseAccount", baseAccountType, "account type mismatch")
			}

			var lastResp string
			// Register claim multiple times if specified
			for i := 0; i < tc.claimAttempts; i++ {
				registerCmd := []string{
					"tx", "claim", "delayed-claim",
					testData.OldAddress, // Old address
					testData.NewAddress, // New address
					testData.PubKey,     // PubKey
					testData.Signature,  // Signature
					tc.tier,             // Tier
					"--from", tc.from,   // From address
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

				// Verify delayed_claim_processed event and transfer from module
				events := gjson.Get(txResp, "events")
				require.True(t, events.Exists())

				foundClaimEvent := false
				foundModuleTransfer := false

				var claimTime, delayedTime int64
				for _, event := range events.Array() {
					eventType := event.Get("type").String()
					attrs := event.Get("attributes").Array()

					// Check delayed_claim_processed event
					if eventType == "delayed_claim_processed" {
						foundClaimEvent = true
						for _, attr := range attrs {
							key := attr.Get("key").String()
							value := attr.Get("value").String()
							switch key {
							case "module":
								require.Equal(t, "claim", value)
							case "amount":
								require.Equal(t, strconv.FormatUint(tc.balanceToClaim, 10) + claimtypes.DefaultClaimsDenom, value)
							case "old_address":
								require.Equal(t, testData.OldAddress, value)
							case "new_address":
								require.Equal(t, testData.NewAddress, value)
							case "delayed_end_time":
								delayedTime = attr.Get("value").Int()
							case "claim_time":
								claimTime = attr.Get("value").Int()
							}
						}
						require.Greater(t, delayedTime, claimTime, "delayed_end_time should be greater than claim_time")
						// delayedTime = claimTime + 6 month * tc.tier
						//tier, _ := strconv.ParseInt(tc.tier, 10, 64)
						//endTime := time.Unix(claimTime, 0).AddDate(0, int(tier*6), 0).Unix() - 3600
						//require.Equal(t, endTime, delayedTime, "delayed_end_time should be equal to claim_time + 6 month * tc.tier")
					}

					// Check for transfer from module to recipient
					if eventType == "transfer" {
						// Only interested in msg_index=0 which relates to the claim operation
						msgIndexAttr := false
						for _, attr := range attrs {
							if attr.Get("key").String() == "msg_index" && attr.Get("value").String() == "0" {
								msgIndexAttr = true
								break
							}
						}

						if msgIndexAttr {
							recipient := ""
							amount := ""

							for _, attr := range attrs {
								if attr.Get("key").String() == "recipient" {
									recipient = attr.Get("value").String()
								} else if attr.Get("key").String() == "amount" {
									amount = attr.Get("value").String()
								}
							}

							if recipient == testData.NewAddress &&
								amount == (strconv.FormatUint(tc.balanceToClaim, 10) + claimtypes.DefaultClaimsDenom) {
								foundModuleTransfer = true
							}
						}
					}
				}

				require.True(t, foundClaimEvent, "claim_processed event not found")
				require.True(t, foundModuleTransfer, "module transfer to recipient not found")

				// Verify balance after claim
				balance := cli.QueryBalance(testData.NewAddress, claimtypes.DefaultClaimsDenom)
				require.Equal(t, tc.balanceToClaim, balance)

				// Verify vested account creation
				vestingAccount := cli.GetAccount(testData.NewAddress)
				require.NotNil(t, vestingAccount, "account not found")
				// Check if the account is a vesting account
				vestingAccountType := gjson.Get(vestingAccount, "account.type").String()
				require.Equal(t, "/cosmos.vesting.v1beta1.DelayedVestingAccount", vestingAccountType, "account type mismatch")
				// Check if the account has a vesting schedule
				vestingSchedule := gjson.Get(vestingAccount, "account.value.base_vesting_account.end_time").Int()
				require.Greater(t, vestingSchedule, int64(0), "vesting schedule not found")
				// check end_time is equal to delayed_end_time
				require.Equal(t, delayedTime, vestingSchedule, "vesting schedule mismatch")
				// Check if the account has a balance
				vesting_balance := gjson.Get(vestingAccount, "account.value.base_vesting_account.original_vesting").Array()
				require.NotEmpty(t, vesting_balance, "account balance not found")
				// check amount is equal to balanceToClaim
				amount := gjson.Get(vestingAccount, "account.value.base_vesting_account.original_vesting.0.amount").String()
				require.Equal(t, tc.balanceToClaim, amount, "account balance mismatch")
			} else {
				RequireTxFailure(t, lastResp, tc.expectedError)
			}
		})
	}
}
