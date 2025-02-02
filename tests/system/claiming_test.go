package system_test

import (
	"context"
	"os"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/LumeraProtocol/lumera/app"
	"github.com/LumeraProtocol/lumera/tests/ibctesting"
	"github.com/LumeraProtocol/lumera/x/claim/keeper"
	"github.com/LumeraProtocol/lumera/x/claim/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	claimtestutils "github.com/LumeraProtocol/lumera/x/claim/testutils"
)

type SystemTestSuite struct {
	app    *app.App
	sdkCtx sdk.Context
	ctx    context.Context
}

func setupClaimSystemSuite(t *testing.T) *SystemTestSuite {
	os.Setenv("SYSTEM_TESTS", "true")
	t.Cleanup(func() {
		os.Unsetenv("SYSTEM_TESTS")
	})

	suite := &SystemTestSuite{}
	coord := ibctesting.NewCoordinator(t, 1) // One chain setup
	chain := coord.GetChain(ibctesting.GetChainID(1))

	app := chain.App.(*app.App)
	suite.app = app

	baseCtx := chain.GetContext()
	suite.sdkCtx = baseCtx
	suite.ctx = baseCtx

	// Set up default parameters
	err := suite.app.ClaimKeeper.SetParams(chain.GetContext(), types.DefaultParams())
	require.NoError(t, err)

	return suite
}

func TestClaimProcess(t *testing.T) {
	suite := setupClaimSystemSuite(t)

	// Generate test data
	testData, err := claimtestutils.GenerateClaimingTestData()
	t.Logf("\nTest Data Generated:"+
		"\n===================="+
		"\nOldAddress: %s"+
		"\nPubKey:     %s"+
		"\nNewAddress: %s"+
		"\nSignature:  %s"+
		"\n====================\n",
		testData.OldAddress,
		testData.PubKey,
		testData.NewAddress,
		testData.Signature)
	require.NoError(t, err)

	testAmount := int64(1000000)

	testCases := []struct {
		name          string
		setup         func()
		msg           *types.MsgClaim
		expectError   bool
		expectedError error
	}{
		{
			name: "Successful claim with non-existing new address",
			setup: func() {
				params := types.DefaultParams()
				params.EnableClaims = true
				err := suite.app.ClaimKeeper.SetParams(suite.sdkCtx, params)
				require.NoError(t, err)

				claimRecord := types.ClaimRecord{
					OldAddress: testData.OldAddress,
					Balance:    sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(testAmount))),
					Claimed:    false,
				}
				err = suite.app.ClaimKeeper.SetClaimRecord(suite.sdkCtx, claimRecord)
				require.NoError(t, err)

				err = suite.app.BankKeeper.MintCoins(suite.sdkCtx, types.ModuleName, claimRecord.Balance)
				require.NoError(t, err)
			},
			msg: &types.MsgClaim{
				OldAddress: testData.OldAddress,
				NewAddress: testData.NewAddress,
				PubKey:     testData.PubKey,
				Signature:  testData.Signature,
			},
			expectError: false,
		},
		{
			name: "Already claimed",
			setup: func() {
				params := types.DefaultParams()
				params.EnableClaims = true
				err := suite.app.ClaimKeeper.SetParams(suite.sdkCtx, params)
				require.NoError(t, err)

				claimRecord := types.ClaimRecord{
					OldAddress: testData.OldAddress,
					Balance:    sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(testAmount))),
					Claimed:    true,
					ClaimTime:  suite.sdkCtx.BlockTime().Unix(),
				}
				err = suite.app.ClaimKeeper.SetClaimRecord(suite.sdkCtx, claimRecord)
				require.NoError(t, err)
			},
			msg: &types.MsgClaim{
				OldAddress: testData.OldAddress,
				NewAddress: testData.NewAddress,
				PubKey:     testData.PubKey,
				Signature:  testData.Signature,
			},
			expectError:   true,
			expectedError: types.ErrClaimAlreadyClaimed,
		},
		{
			name: "Non-existent claim",
			setup: func() {
				params := types.DefaultParams()
				params.EnableClaims = true
				err := suite.app.ClaimKeeper.SetParams(suite.sdkCtx, params)
				require.NoError(t, err)
			},
			msg: &types.MsgClaim{
				OldAddress: "PtqHAEacynVd3V821NPhgxu9K4Ab6kAguHx", // Different address
				NewAddress: testData.NewAddress,
				PubKey:     testData.PubKey,
				Signature:  testData.Signature,
			},
			expectError:   true,
			expectedError: types.ErrClaimNotFound,
		},
		{
			name: "Claims disabled",
			setup: func() {
				params := types.DefaultParams()
				params.EnableClaims = false
				err := suite.app.ClaimKeeper.SetParams(suite.sdkCtx, params)
				require.NoError(t, err)

				claimRecord := types.ClaimRecord{
					OldAddress: testData.OldAddress,
					Balance:    sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(testAmount))),
					Claimed:    false,
				}
				err = suite.app.ClaimKeeper.SetClaimRecord(suite.sdkCtx, claimRecord)
				require.NoError(t, err)
			},
			msg: &types.MsgClaim{
				OldAddress: testData.OldAddress,
				NewAddress: testData.NewAddress,
				PubKey:     testData.PubKey,
				Signature:  testData.Signature,
			},
			expectError:   true,
			expectedError: types.ErrClaimDisabled,
		},
		{
			name: "Invalid signature",
			setup: func() {
				params := types.DefaultParams()
				params.EnableClaims = true
				err := suite.app.ClaimKeeper.SetParams(suite.sdkCtx, params)
				require.NoError(t, err)

				claimRecord := types.ClaimRecord{
					OldAddress: testData.OldAddress,
					Balance:    sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(testAmount))),
					Claimed:    false,
				}
				err = suite.app.ClaimKeeper.SetClaimRecord(suite.sdkCtx, claimRecord)
				require.NoError(t, err)
			},
			msg: &types.MsgClaim{
				OldAddress: testData.OldAddress,
				NewAddress: testData.NewAddress,
				PubKey:     testData.PubKey,
				Signature:  "invalid_signature",
			},
			expectError:   true,
			expectedError: types.ErrInvalidSignature,
		},
		{
			name: "Claim period expired",
			setup: func() {
				params := types.DefaultParams()
				params.EnableClaims = true
				params.ClaimEndTime = suite.sdkCtx.BlockTime().Unix() - 1
				err := suite.app.ClaimKeeper.SetParams(suite.sdkCtx, params)
				require.NoError(t, err)

				claimRecord := types.ClaimRecord{
					OldAddress: testData.OldAddress,
					Balance:    sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(testAmount))),
					Claimed:    false,
				}
				err = suite.app.ClaimKeeper.SetClaimRecord(suite.sdkCtx, claimRecord)
				require.NoError(t, err)
			},
			msg: &types.MsgClaim{
				OldAddress: testData.OldAddress,
				NewAddress: testData.NewAddress,
				PubKey:     testData.PubKey,
				Signature:  testData.Signature,
			},
			expectError:   true,
			expectedError: types.ErrClaimPeriodExpired,
		},
	}

	// Execute each test case
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up the test environment for this specific case
			tc.setup()
			msgServer := keeper.NewMsgServerImpl(suite.app.ClaimKeeper)

			// Record initial balances for validation
			moduleAddr := suite.app.AccountKeeper.GetModuleAddress(types.ModuleName)
			initialModuleBalance := suite.app.BankKeeper.GetAllBalances(suite.sdkCtx, moduleAddr)

			// Get destination address and its initial balance
			destAddr, err := sdk.AccAddressFromBech32(tc.msg.NewAddress)
			require.NoError(t, err)
			initialUserBalance := suite.app.BankKeeper.GetAllBalances(suite.sdkCtx, destAddr)

			// Execute the claim message
			response, err := msgServer.Claim(suite.sdkCtx, tc.msg)

			// Handle error cases
			if tc.expectError {
				require.Error(t, err)
				require.ErrorIs(t, err, tc.expectedError)
				require.Nil(t, response)
			} else {
				// Verify successful claim execution
				require.NoError(t, err)
				require.NotNil(t, response)

				// Verify claim record has been properly updated
				record, found, err := suite.app.ClaimKeeper.GetClaimRecord(suite.sdkCtx, tc.msg.OldAddress)
				require.NoError(t, err)
				require.True(t, found)
				require.True(t, record.Claimed)
				require.NotZero(t, record.ClaimTime)

				// Verify destination account exists (should be created if it didn't exist)
				acc := suite.app.AccountKeeper.GetAccount(suite.sdkCtx, destAddr)
				require.NotNil(t, acc)

				// Verify token balances after transfer
				finalUserBalance := suite.app.BankKeeper.GetAllBalances(suite.sdkCtx, destAddr)
				finalModuleBalance := suite.app.BankKeeper.GetAllBalances(suite.sdkCtx, moduleAddr)

				// Check user received the correct amount
				expectedUserBalance := initialUserBalance.Add(record.Balance...)
				require.Equal(t, expectedUserBalance, finalUserBalance)

				// Check module account balance decreased correctly
				expectedModuleBalance := initialModuleBalance.Sub(record.Balance...)
				require.Equal(t, expectedModuleBalance, finalModuleBalance)

				// Verify event emission
				events := suite.sdkCtx.EventManager().Events()
				require.NotEmpty(t, events)

				// Look for the claim processed event and verify its attributes
				var foundClaimEvent bool
				for _, event := range events {
					if event.Type == types.EventTypeClaimProcessed {
						foundClaimEvent = true
						hasOldAddr, hasNewAddr, hasClaimTime := false, false, false

						// Check all required attributes are present with correct values
						for _, attr := range event.Attributes {
							switch string(attr.Key) {
							case types.AttributeKeyOldAddress:
								require.Equal(t, tc.msg.OldAddress, string(attr.Value))
								hasOldAddr = true
							case types.AttributeKeyNewAddress:
								require.Equal(t, tc.msg.NewAddress, string(attr.Value))
								hasNewAddr = true
							case types.AttributeKeyClaimTime:
								require.NotEmpty(t, string(attr.Value))
								hasClaimTime = true
							}
						}

						// Ensure all required attributes were found
						require.True(t, hasOldAddr, "missing old address attribute")
						require.True(t, hasNewAddr, "missing new address attribute")
						require.True(t, hasClaimTime, "missing claim time attribute")
					}
				}
				require.True(t, foundClaimEvent, "claim_processed event not found")
			}
		})
	}
}
