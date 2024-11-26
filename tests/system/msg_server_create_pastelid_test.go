package system_test

import (
	"context"
	"os"
	"testing"

	sdkmath "cosmossdk.io/math"
	wasmvmtypes "github.com/CosmWasm/wasmvm/v2/types"
	"github.com/cometbft/cometbft/crypto/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pastelnetwork/pastel/app"
	"github.com/pastelnetwork/pastel/tests/ibctesting"
	"github.com/pastelnetwork/pastel/tests/system"
	"github.com/pastelnetwork/pastel/x/pastelid/keeper"
	"github.com/pastelnetwork/pastel/x/pastelid/types"
	"github.com/stretchr/testify/assert"
)

type SystemTestSuite struct {
	app    *app.App
	sdkCtx sdk.Context
	ctx    context.Context
}

func setupSystemSuite(t *testing.T) *SystemTestSuite {
	os.Setenv("SYSTEM_TESTS", "true")

	suite := &SystemTestSuite{}
	coord := ibctesting.NewCoordinator(t, 1) // One chain setup
	chain := coord.GetChain(ibctesting.GetChainID(1))

	contractAddr := system.InstantiateReflectContract(t, chain)
	chain.Fund(contractAddr, sdkmath.NewIntFromUint64(1_000_000_000))

	app := chain.App.(*app.App)
	suite.app = app

	suite.ctx = sdk.WrapSDKContext(chain.GetContext())
	suite.sdkCtx = sdk.UnwrapSDKContext(chain.GetContext())

	// Delegate a high amount to the contract
	delegateMsg := wasmvmtypes.CosmosMsg{
		Staking: &wasmvmtypes.StakingMsg{
			Delegate: &wasmvmtypes.DelegateMsg{
				Validator: sdk.ValAddress(chain.Vals.Validators[0].Address).String(),
				Amount: wasmvmtypes.Coin{
					Denom:  sdk.DefaultBondDenom,
					Amount: "1000000000",
				},
			},
		},
	}
	system.MustExecViaReflectContract(t, chain, contractAddr, delegateMsg)

	err := suite.app.PastelidKeeper.SetParams(chain.GetContext(), types.DefaultParams())
	assert.NoError(t, err)

	return suite
}

func TestCreatePastelId(t *testing.T) {
	suite := setupSystemSuite(t)

	privKey := secp256k1.GenPrivKey()
	creator := sdk.AccAddress(privKey.PubKey().Address())

	testCases := []struct {
		name          string
		setup         func()
		msg           *types.MsgCreatePastelId
		expectError   bool
		expectedError error
	}{
		{
			name: "Insufficient funds",
			setup: func() {
			},
			msg: &types.MsgCreatePastelId{
				Creator:   creator.String(),
				IdType:    "test-type",
				PastelId:  "new-pastelid",
				PqKey:     "test-pqkey",
				Signature: "test-signature",
				TimeStamp: "2022-01-01T00:00:00Z",
				Version:   1.0,
			},
			expectError:   true,
			expectedError: types.ErrInsufficientFunds,
		},
		{
			name: "Valid PastelId creation",
			setup: func() {
				initialBalance := sdk.NewCoins(sdk.NewCoin("upsl", sdkmath.NewInt(10_000_000_000)))
				err := suite.app.BankKeeper.MintCoins(suite.sdkCtx, types.ModuleName, initialBalance)
				assert.NoError(t, err)

				err = suite.app.BankKeeper.SendCoinsFromModuleToAccount(suite.sdkCtx, types.ModuleName, creator, initialBalance)
				assert.NoError(t, err)
			},
			msg: &types.MsgCreatePastelId{
				Creator:   creator.String(),
				IdType:    "test-type",
				PastelId:  "test-pastelid",
				PqKey:     "test-pqkey",
				Signature: "test-signature",
				TimeStamp: "2022-01-01T00:00:00Z",
				Version:   1.0,
			},
			expectError:   false,
			expectedError: nil,
		},
		{
			name: "PastelID already exists",
			setup: func() {
				entry := types.PastelidEntry{
					Address:   creator.String(),
					IdType:    "test-type",
					PastelId:  "existing-pastelid",
					PqKey:     "test-pqkey",
					Signature: "test-signature",
					TimeStamp: "2022-01-01T00:00:00Z",
					Version:   1.0,
				}
				suite.app.PastelidKeeper.SetPastelidEntry(suite.sdkCtx, entry)
			},
			msg: &types.MsgCreatePastelId{
				Creator:   creator.String(),
				IdType:    "test-type",
				PastelId:  "existing-pastelid",
				PqKey:     "test-pqkey",
				Signature: "test-signature",
				TimeStamp: "2022-01-01T00:00:00Z",
				Version:   1.0,
			},
			expectError:   true,
			expectedError: types.ErrPastelIDExists,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup()
			msgServer := keeper.NewMsgServerImpl(suite.app.PastelidKeeper)

			response, err := msgServer.CreatePastelId(suite.ctx, tc.msg)

			if tc.expectError {
				assert.ErrorIs(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, response)
			}
		})
	}
}
