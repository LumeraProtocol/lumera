package system_test

import (
	"context"
	sdkmath "cosmossdk.io/math"
	"testing"
	"time"

	"github.com/cometbft/cometbft/crypto/secp256k1"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pastelnetwork/pasteld/app"
	"github.com/pastelnetwork/pasteld/x/pastelid/keeper"
	"github.com/pastelnetwork/pasteld/x/pastelid/types"
	"github.com/stretchr/testify/assert"
)

type SystemTestSuite struct {
	app    *app.App
	sdkCtx sdk.Context
	ctx    context.Context
}

func (suite *SystemTestSuite) SetupSuite() {

}

func setupSystemSuite(t *testing.T) *SystemTestSuite {
	suite := &SystemTestSuite{}
	suite.app = app.Setup(t)

	suite.sdkCtx = suite.app.BaseApp.NewContext(true).WithBlockHeader(tmproto.Header{
		ChainID: "test-chain",
		Height:  1,
		Time:    time.Now(),
	})

	suite.ctx = sdk.WrapSDKContext(suite.sdkCtx)

	err := suite.app.PastelidKeeper.SetParams(suite.sdkCtx, types.DefaultParams())
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
