package integration_test

import (
	"context"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/cometbft/cometbft/crypto/secp256k1"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/pastelnetwork/pastel/app"
	"github.com/pastelnetwork/pastel/x/pastelid/keeper"
	"github.com/pastelnetwork/pastel/x/pastelid/types"
	"github.com/stretchr/testify/assert"
)

func TestCreatePastelId(t *testing.T) {

	type dependencies struct {
		app    *app.App
		ctx    context.Context
		sdkCtx sdk.Context

		keeper    keeper.Keeper
		msgServer types.MsgServer

		creator sdk.AccAddress
	}

	var deps dependencies
	deps.app = app.Setup(t)
	deps.ctx = context.Background()
	deps.sdkCtx = deps.app.BaseApp.NewContext(true)

	// Define the block header
	header := tmproto.Header{
		ChainID: "test-chain",
		Height:  1,
		Time:    time.Now(),
	}

	deps.sdkCtx = deps.sdkCtx.WithBlockHeader(header)

	deps.keeper = deps.app.PastelidKeeper
	deps.msgServer = keeper.NewMsgServerImpl(deps.keeper)

	moduleAcc := deps.app.AccountKeeper.GetModuleAccount(deps.sdkCtx, types.ModuleName)
	if moduleAcc == nil {
		moduleAcc = authtypes.NewEmptyModuleAccount(types.ModuleName, authtypes.Minter, authtypes.Burner)
		deps.app.AccountKeeper.SetModuleAccount(deps.sdkCtx, moduleAcc)
	}

	deps.keeper.SetParams(deps.sdkCtx, types.DefaultParams())

	privKey := secp256k1.GenPrivKey()
	deps.creator = sdk.AccAddress(privKey.PubKey().Address())

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
				err := deps.app.BankKeeper.SendCoins(deps.sdkCtx, deps.creator, keyPubAddr(), sdk.NewCoins())
				assert.NoError(t, err)
			},
			msg: &types.MsgCreatePastelId{
				Creator:   deps.creator.String(),
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
				err := deps.app.BankKeeper.MintCoins(deps.sdkCtx, types.ModuleName, initialBalance)
				assert.NoError(t, err)

				err = deps.app.BankKeeper.SendCoinsFromModuleToAccount(deps.sdkCtx, types.ModuleName, deps.creator, initialBalance)
				assert.NoError(t, err)
			},
			msg: &types.MsgCreatePastelId{
				Creator:   deps.creator.String(),
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
			name:  "Invalid Creator address",
			setup: func() {},
			msg: &types.MsgCreatePastelId{
				Creator:   "invalid-address",
				IdType:    "test-type",
				PastelId:  "test-pastelid",
				PqKey:     "test-pqkey",
				Signature: "test-signature",
				TimeStamp: "2022-01-01T00:00:00Z",
				Version:   1.0,
			},
			expectError:   true,
			expectedError: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "PastelID already exists",
			setup: func() {
				pastelidEntry := types.PastelidEntry{
					Address:   deps.creator.String(),
					IdType:    "test-type",
					PastelId:  "existing-pastelid",
					PqKey:     "test-pqkey",
					Signature: "test-signature",
					TimeStamp: "2022-01-01T00:00:00Z",
					Version:   1.0,
				}
				deps.keeper.SetPastelidEntry(deps.sdkCtx, pastelidEntry)
			},
			msg: &types.MsgCreatePastelId{
				Creator:   deps.creator.String(),
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

			goCtx := sdk.WrapSDKContext(deps.sdkCtx)
			response, err := deps.msgServer.CreatePastelId(goCtx, tc.msg)
			if tc.expectError {
				assert.ErrorIs(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, response)
			}
		})
	}
}

func keyPubAddr() sdk.AccAddress {
	key := secp256k1.GenPrivKey()
	pub := key.PubKey()
	addr := sdk.AccAddress(pub.Address())
	return addr
}
