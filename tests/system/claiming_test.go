package system_test

import (
	"os"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pastelnetwork/pastel/app"
	"github.com/pastelnetwork/pastel/tests/ibctesting"
	"github.com/pastelnetwork/pastel/x/claim/keeper"
	"github.com/pastelnetwork/pastel/x/claim/types"
	"github.com/stretchr/testify/require"
)

// valid test data
const (
	OldAddress     = "PtqHAEacynVd3V821NPhgxu9K4Ab6kAguHi"
	Pubkey         = "0309331fc3d23ca17d91eec40ee7711efcd56facf949d46cbfa6393d43f2747e90"
	NewAddress     = "pastel139k6camfq63u9gtc4pq8yjw4j7tmwmqeggr4p0"
	validSignature = "1f46b3a2129047a0d7a6bf91e2879e940ed3db06a2cafaaaabacc337141146f43e4932d357b435bbf2c48227f5c2f738df23a2ebc221dd11cb14ed4b83bd2a95c7"
	testAmount     = 1000000
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
	suite.ctx = sdk.WrapSDKContext(baseCtx)

	// Set up default parameters
	err := suite.app.ClaimKeeper.SetParams(chain.GetContext(), types.DefaultParams())
	require.NoError(t, err)

	return suite
}

func TestClaimProcess(t *testing.T) {
	suite := setupClaimSystemSuite(t)

	// Create a test account
	testAddr := sdk.MustAccAddressFromBech32(NewAddress)

	// Create and fund the account with the specific address
	acc := suite.app.AccountKeeper.NewAccountWithAddress(suite.sdkCtx, testAddr)
	suite.app.AccountKeeper.SetAccount(suite.sdkCtx, acc)

	testCases := []struct {
		name          string
		setup         func()
		msg           *types.MsgClaim
		expectError   bool
		expectedError error
	}{
		{
			name: "Successful claim",
			setup: func() {
				// Enable claims
				params := types.DefaultParams()
				params.EnableClaims = true
				err := suite.app.ClaimKeeper.SetParams(suite.sdkCtx, params)
				require.NoError(t, err)

				oldAddress := OldAddress
				claimRecord := types.ClaimRecord{
					OldAddress: oldAddress,
					Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultDenom, sdkmath.NewInt(testAmount))),
					Claimed:    false,
				}
				err = suite.app.ClaimKeeper.SetClaimRecord(suite.sdkCtx, claimRecord)
				require.NoError(t, err)

				err = suite.app.BankKeeper.MintCoins(suite.sdkCtx, types.ModuleName, claimRecord.Balance)
				require.NoError(t, err)
			},
			msg: &types.MsgClaim{
				OldAddress: OldAddress,
				NewAddress: testAddr.String(),
				PubKey:     Pubkey,
				Signature:  validSignature,
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

				oldAddress := OldAddress
				claimRecord := types.ClaimRecord{
					OldAddress: oldAddress,
					Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultDenom, sdkmath.NewInt(testAmount))),
					Claimed:    true,
					ClaimTime:  suite.sdkCtx.BlockTime().Unix(),
				}
				err = suite.app.ClaimKeeper.SetClaimRecord(suite.sdkCtx, claimRecord)
				require.NoError(t, err)
			},
			msg: &types.MsgClaim{
				OldAddress: OldAddress,
				NewAddress: testAddr.String(),
				PubKey:     Pubkey,
				Signature:  validSignature,
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
				OldAddress: "PtqHAEacynVd3V821NPhgxu9K4Ab6kAguHx",
				NewAddress: testAddr.String(),
				PubKey:     Pubkey,
				Signature:  validSignature,
			},
			expectError:   true,
			expectedError: types.ErrClaimNotFound,
		},
		{
			name: "Invalid pubkey",
			setup: func() {
				params := types.DefaultParams()
				params.EnableClaims = true
				err := suite.app.ClaimKeeper.SetParams(suite.sdkCtx, params)
				require.NoError(t, err)

				oldAddress := OldAddress
				claimRecord := types.ClaimRecord{
					OldAddress: oldAddress,
					Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultDenom, sdkmath.NewInt(testAmount))),
					Claimed:    false,
				}
				err = suite.app.ClaimKeeper.SetClaimRecord(suite.sdkCtx, claimRecord)
				require.NoError(t, err)
			},
			msg: &types.MsgClaim{
				OldAddress: OldAddress,
				NewAddress: testAddr.String(),
				PubKey:     "030933invalid1pubkey2here3please4ignore5this6one7thanks890",
				Signature:  validSignature,
			},
			expectError:   true,
			expectedError: types.ErrInvalidPubKey,
		},
		{
			name: "Invalid signature",
			setup: func() {
				params := types.DefaultParams()
				params.EnableClaims = true
				err := suite.app.ClaimKeeper.SetParams(suite.sdkCtx, params)
				require.NoError(t, err)

				oldAddress := OldAddress
				claimRecord := types.ClaimRecord{
					OldAddress: oldAddress,
					Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultDenom, sdkmath.NewInt(testAmount))),
					Claimed:    false,
				}
				err = suite.app.ClaimKeeper.SetClaimRecord(suite.sdkCtx, claimRecord)
				require.NoError(t, err)
			},
			msg: &types.MsgClaim{
				OldAddress: OldAddress,
				NewAddress: testAddr.String(),
				PubKey:     Pubkey,
				Signature:  "1f46b3a2129047a0d7a6bf91e2879e940ed3db06a2cafaaaabacc337141146f43e4932d357b435bbf2c48227f5c2f738df23a2ebc221dd11cb14ed4b83bd2a95c8",
			},
			expectError:   true,
			expectedError: types.ErrInvalidSignature,
		},
		{
			name: "Claims disabled",
			setup: func() {
				params := types.DefaultParams()
				params.EnableClaims = false
				err := suite.app.ClaimKeeper.SetParams(suite.sdkCtx, params)
				require.NoError(t, err)

				oldAddress := OldAddress
				claimRecord := types.ClaimRecord{
					OldAddress: oldAddress,
					Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultDenom, sdkmath.NewInt(testAmount))),
					Claimed:    false,
				}
				err = suite.app.ClaimKeeper.SetClaimRecord(suite.sdkCtx, claimRecord)
				require.NoError(t, err)
			},
			msg: &types.MsgClaim{
				OldAddress: OldAddress,
				NewAddress: testAddr.String(),
				PubKey:     Pubkey,
				Signature:  validSignature,
			},
			expectError:   true,
			expectedError: types.ErrClaimDisabled,
		},
		{
			name: "Too many claims in block",
			setup: func() {
				params := types.DefaultParams()
				params.EnableClaims = true
				err := suite.app.ClaimKeeper.SetParams(suite.sdkCtx, params)
				require.NoError(t, err)

				oldAddress := OldAddress
				claimRecord := types.ClaimRecord{
					OldAddress: oldAddress,
					Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultDenom, sdkmath.NewInt(testAmount))),
					Claimed:    false,
				}
				err = suite.app.ClaimKeeper.SetClaimRecord(suite.sdkCtx, claimRecord)
				require.NoError(t, err)

				// Set claims count to max
				for i := uint64(0); i < params.MaxClaimsPerBlock; i++ {
					suite.app.ClaimKeeper.IncrementBlockClaimCount(suite.sdkCtx)
				}
			},
			msg: &types.MsgClaim{
				OldAddress: OldAddress,
				NewAddress: testAddr.String(),
				PubKey:     Pubkey,
				Signature:  validSignature,
			},
			expectError:   true,
			expectedError: types.ErrTooManyClaims,
		},
		{
			name: "Claim period expired",
			setup: func() {
				params := types.DefaultParams()
				params.EnableClaims = true
				params.ClaimEndTime = suite.sdkCtx.BlockTime().Unix() - 1
				err := suite.app.ClaimKeeper.SetParams(suite.sdkCtx, params)
				require.NoError(t, err)

				oldAddress := OldAddress
				claimRecord := types.ClaimRecord{
					OldAddress: oldAddress,
					Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultDenom, sdkmath.NewInt(testAmount))),
					Claimed:    false,
				}
				err = suite.app.ClaimKeeper.SetClaimRecord(suite.sdkCtx, claimRecord)
				require.NoError(t, err)

				err = suite.app.BankKeeper.MintCoins(suite.sdkCtx, types.ModuleName, claimRecord.Balance)
				require.NoError(t, err)

				suite.sdkCtx = suite.sdkCtx.WithBlockHeight(suite.sdkCtx.BlockHeight() + 1).
					WithBlockTime(suite.sdkCtx.BlockTime().Add(time.Hour * 24 * 365))
			},
			msg: &types.MsgClaim{
				OldAddress: OldAddress,
				NewAddress: testAddr.String(),
				PubKey:     Pubkey,
				Signature:  validSignature,
			},
			expectError:   true,
			expectedError: types.ErrClaimPeriodExpired,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup()
			msgServer := keeper.NewMsgServerImpl(suite.app.ClaimKeeper)

			moduleAddr := suite.app.AccountKeeper.GetModuleAddress(types.ModuleName)
			initialModuleBalance := suite.app.BankKeeper.GetAllBalances(suite.sdkCtx, moduleAddr)
			initialUserBalance := suite.app.BankKeeper.GetAllBalances(suite.sdkCtx, testAddr)

			response, err := msgServer.Claim(suite.ctx, tc.msg)

			if tc.expectError {
				require.Error(t, err)
				require.ErrorIs(t, err, tc.expectedError)
				require.Nil(t, response)

				if tc.name != "Non-existent claim" {
					record, found, err := suite.app.ClaimKeeper.GetClaimRecord(suite.sdkCtx, tc.msg.OldAddress)
					require.NoError(t, err)
					require.True(t, found)
					if tc.name == "Already claimed" {
						require.True(t, record.Claimed)
					} else {
						require.False(t, record.Claimed)
					}
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, response)

				record, found, err := suite.app.ClaimKeeper.GetClaimRecord(suite.sdkCtx, tc.msg.OldAddress)
				require.NoError(t, err)
				require.True(t, found)
				require.True(t, record.Claimed)
				require.NotZero(t, record.ClaimTime)

				finalUserBalance := suite.app.BankKeeper.GetAllBalances(suite.sdkCtx, testAddr)
				finalModuleBalance := suite.app.BankKeeper.GetAllBalances(suite.sdkCtx, moduleAddr)

				expectedUserBalance := initialUserBalance.Add(record.Balance...)
				require.Equal(t, expectedUserBalance, finalUserBalance)

				expectedModuleBalance := initialModuleBalance.Sub(record.Balance...)
				require.Equal(t, expectedModuleBalance, finalModuleBalance)

				events := suite.sdkCtx.EventManager().Events()
				require.NotEmpty(t, events)

				var eventFound bool
				for _, event := range events {
					if event.Type == types.EventTypeClaimProcessed {
						eventFound = true
						var hasOldAddr, hasNewAddr bool
						for _, attr := range event.Attributes {
							switch string(attr.Key) {
							case types.AttributeKeyOldAddress:
								require.Equal(t, tc.msg.OldAddress, string(attr.Value))
								hasOldAddr = true
							case types.AttributeKeyNewAddress:
								require.Equal(t, tc.msg.NewAddress, string(attr.Value))
								hasNewAddr = true
							}
						}
						require.True(t, hasOldAddr, "missing old address attribute")
						require.True(t, hasNewAddr, "missing new address attribute")
					}
				}
				require.True(t, eventFound, "claim_processed event not found")
			}
		})
	}
}
