package system_test

import (
	"github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	types2 "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

func TestUpdateSupernode(t *testing.T) {
	// Base accounts
	walletPrivKey := secp256k1.GenPrivKey()
	walletAddr := sdk.AccAddress(walletPrivKey.PubKey().Address())
	valAddr := sdk.ValAddress(walletAddr)
	valAddrStr := valAddr.String()

	// Unauthorized address
	unauthPrivKey := secp256k1.GenPrivKey()
	unauthAddr := sdk.AccAddress(unauthPrivKey.PubKey().Address())

	testCases := []struct {
		name   string
		msg    *types2.MsgUpdateSupernode
		setup  func(*SystemTestSuite)
		verify func(t *testing.T, suite *SystemTestSuite, resp *types2.MsgUpdateSupernodeResponse, err error)
	}{
		{
			name: "basic update - new ip, new version, new supernode account",
			msg: &types2.MsgUpdateSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: valAddrStr,
				IpAddress:        "10.0.0.2",
				Version:          "2.0.0",
				SupernodeAccount: sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String(),
			},
			setup: func(suite *SystemTestSuite) {
				// Register a supernode in some initial state
				sn := types2.SuperNode{
					ValidatorAddress: valAddrStr,
					SupernodeAccount: walletAddr.String(),
					Version:          "1.0.0",
					States: []*types2.SuperNodeStateRecord{
						{
							State:  types2.SuperNodeStateActive,
							Height: suite.sdkCtx.BlockHeight(),
						},
					},
					PrevIpAddresses: []*types2.IPAddressHistory{
						{
							Address: "192.168.1.1",
							Height:  suite.sdkCtx.BlockHeight(),
						},
					},
					P2PPort: "26657",
				}
				err := suite.app.SupernodeKeeper.SetSuperNode(suite.sdkCtx, sn)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, suite *SystemTestSuite, resp *types2.MsgUpdateSupernodeResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, resp)

				// Check updated fields
				valOp, vErr := sdk.ValAddressFromBech32(valAddrStr)
				require.NoError(t, vErr)
				sn, found := suite.app.SupernodeKeeper.QuerySuperNode(suite.sdkCtx, valOp)
				require.True(t, found)
				// Verify IP was appended
				require.NotEmpty(t, sn.PrevIpAddresses)
				require.Equal(t, "10.0.0.2", sn.PrevIpAddresses[len(sn.PrevIpAddresses)-1].Address)
				// Verify version
				require.Equal(t, "2.0.0", sn.Version)
				// Verify new supernode account
				require.NotEqual(t, walletAddr.String(), sn.SupernodeAccount)

				// Verify event
				events := suite.sdkCtx.EventManager().Events()
				var foundUpdateEvent bool
				for _, e := range events {
					if e.Type == types2.EventTypeSupernodeUpdated {
						foundUpdateEvent = true
						for _, attr := range e.Attributes {
							if string(attr.Key) == types2.AttributeKeyValidatorAddress {
								require.Equal(t, valAddrStr, string(attr.Value))
							}
							if string(attr.Key) == types2.AttributeKeyVersion {
								require.Equal(t, "2.0.0", string(attr.Value))
							}
						}
					}
				}
				require.True(t, foundUpdateEvent, "supernode_updated event not found")
			},
		},
		{
			name: "supernode not found",
			msg: &types2.MsgUpdateSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: valAddrStr,
				IpAddress:        "10.0.0.3",
			},
			setup: func(suite *SystemTestSuite) { /* do nothing */ },
			verify: func(t *testing.T, suite *SystemTestSuite, resp *types2.MsgUpdateSupernodeResponse, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, sdkerrors.ErrNotFound)
				require.Nil(t, resp)
			},
		},
		{
			name: "unauthorized update attempt",
			msg: &types2.MsgUpdateSupernode{
				Creator:          unauthAddr.String(),
				ValidatorAddress: valAddrStr,
				IpAddress:        "8.8.8.8",
			},
			setup: func(suite *SystemTestSuite) {
				// Create supernode owned by walletAddr
				sn := types2.SuperNode{
					ValidatorAddress: valAddrStr,
					SupernodeAccount: walletAddr.String(),
					Version:          "1.0.0",
					States: []*types2.SuperNodeStateRecord{
						{
							State:  types2.SuperNodeStateActive,
							Height: suite.sdkCtx.BlockHeight(),
						},
					},
					PrevIpAddresses: []*types2.IPAddressHistory{
						{
							Address: "127.0.0.2",
							Height:  suite.sdkCtx.BlockHeight(),
						},
					},
					P2PPort: "26657",
				}
				err := suite.app.SupernodeKeeper.SetSuperNode(suite.sdkCtx, sn)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, suite *SystemTestSuite, resp *types2.MsgUpdateSupernodeResponse, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
				require.Nil(t, resp)
			},
		},
		{
			name: "invalid validator address",
			msg: &types2.MsgUpdateSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: "invalid-addr",
			},
			setup: nil,
			verify: func(t *testing.T, suite *SystemTestSuite, resp *types2.MsgUpdateSupernodeResponse, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
				require.Nil(t, resp)
			},
		},
		{
			name: "update with no changes",
			msg: &types2.MsgUpdateSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: valAddrStr,
				// no changes to ip, version, or supernode account
			},
			setup: func(suite *SystemTestSuite) {
				// Existing supernode
				sn := types2.SuperNode{
					ValidatorAddress: valAddrStr,
					SupernodeAccount: walletAddr.String(),
					Version:          "1.0.0",
					States: []*types2.SuperNodeStateRecord{
						{
							State:  types2.SuperNodeStateActive,
							Height: suite.sdkCtx.BlockHeight(),
						},
					},
					PrevIpAddresses: []*types2.IPAddressHistory{
						{
							Address: "127.0.0.1",
							Height:  suite.sdkCtx.BlockHeight(),
						},
					},
					P2PPort: "26657",
				}
				err := suite.app.SupernodeKeeper.SetSuperNode(suite.sdkCtx, sn)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, suite *SystemTestSuite, resp *types2.MsgUpdateSupernodeResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, resp)

				// Verify nothing changed
				valOp, convErr := sdk.ValAddressFromBech32(valAddrStr)
				require.NoError(t, convErr)
				sn, found := suite.app.SupernodeKeeper.QuerySuperNode(suite.sdkCtx, valOp)
				require.True(t, found)

				// IP should remain the same, version the same, etc.
				require.Equal(t, "1.0.0", sn.Version)
				require.Equal(t, walletAddr.String(), sn.SupernodeAccount)
				require.NotEmpty(t, sn.PrevIpAddresses)
				require.Equal(t, "127.0.0.1", sn.PrevIpAddresses[len(sn.PrevIpAddresses)-1].Address)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create fresh suite for each test
			testSuite := setupSupernodeSystemSuite(t)

			// Create and set up validator in Staking
			validator, err := stakingtypes.NewValidator(valAddrStr, walletPrivKey.PubKey(), stakingtypes.Description{})
			require.NoError(t, err)
			validator.Status = stakingtypes.Bonded
			validator.Tokens = sdkmath.NewInt(1000000)
			testSuite.app.StakingKeeper.SetValidator(testSuite.sdkCtx, validator)

			// Perform any test-specific setup
			if tc.setup != nil {
				tc.setup(testSuite)
			}

			// Invoke the UpdateSupernode message
			msgServer := keeper.NewMsgServerImpl(testSuite.app.SupernodeKeeper)
			resp, err := msgServer.UpdateSupernode(testSuite.ctx, tc.msg)

			// Verification
			tc.verify(t, testSuite, resp, err)
		})
	}
}
