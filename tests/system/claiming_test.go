package system_test

import (
	"encoding/hex"
	"fmt"
	"os"
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pastelnetwork/pastel/app"
	"github.com/pastelnetwork/pastel/tests/ibctesting"
	"github.com/pastelnetwork/pastel/x/claim/keeper"
	"github.com/pastelnetwork/pastel/x/claim/keeper/crypto"
	cryptoutils "github.com/pastelnetwork/pastel/x/claim/keeper/crypto"
	"github.com/pastelnetwork/pastel/x/claim/types"
	"github.com/stretchr/testify/require"
)

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

type TestData struct {
	OldAddress string
	PubKey     string
	NewAddress string
	Signature  string
}

func generateTestData() (*TestData, error) {
	// Generate a new key pair
	privKeyObj, pubKeyObj := cryptoutils.GenerateKeyPair()

	// Get hex encoded public key
	pubKey := hex.EncodeToString(pubKeyObj.Key)

	// Generate old address from public key
	oldAddr, err := crypto.GetAddressFromPubKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate old address: %w", err)
	}

	// Generate a new cosmos address
	newAddr := sdk.AccAddress(privKeyObj.PubKey().Address()).String()

	// Construct message for signature (without hashing)
	message := oldAddr + "." + pubKey + "." + newAddr

	// Sign the message directly without hashing
	signature, err := crypto.SignMessage(privKeyObj, message)
	if err != nil {
		return nil, fmt.Errorf("failed to sign message: %w", err)
	}

	// Verify the signature to ensure it's valid
	valid, err := crypto.VerifySignature(pubKey, message, signature)
	if err != nil {
		return nil, fmt.Errorf("failed to verify generated signature: %w", err)
	}
	if !valid {
		return nil, fmt.Errorf("generated signature verification failed")
	}

	return &TestData{
		OldAddress: oldAddr,
		PubKey:     pubKey,
		NewAddress: newAddr,
		Signature:  signature,
	}, nil
}

func TestClaimProcess(t *testing.T) {
	suite := setupClaimSystemSuite(t)

	validFee := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(1000)))

	// Generate test data
	testData, err := generateTestData()
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

				// Add fee to context
				fee := sdk.NewCoins(validFee...) // Small fee
				suite.sdkCtx = suite.sdkCtx.WithValue(types.ClaimTxFee, fee)
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

				// Check user received the correct amount (minus fee)
				expectedUserBalance := initialUserBalance.Add(record.Balance...).Sub(validFee...)
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
