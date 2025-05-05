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

func TestStopSupernode(t *testing.T) {
	// Generate base keys & addresses
	walletPrivKey := secp256k1.GenPrivKey()
	walletAddr := sdk.AccAddress(walletPrivKey.PubKey().Address())
	valAddr := sdk.ValAddress(walletAddr)
	valAddrStr := valAddr.String()

	// An unauthorized address
	unauthPrivKey := secp256k1.GenPrivKey()
	unauthAddr := sdk.AccAddress(unauthPrivKey.PubKey().Address())

	testCases := []struct {
		name   string
		msg    *types2.MsgStopSupernode
		setup  func(*SystemTestSuite)
		verify func(t *testing.T, suite *SystemTestSuite, resp *types2.MsgStopSupernodeResponse, err error)
	}{
		{
			name: "successful stop",
			msg: &types2.MsgStopSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: valAddrStr,
				Reason:           "maintenance",
			},
			setup: func(suite *SystemTestSuite) {
				// Create a supernode in active state
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
							Address: "192.168.0.2",
							Height:  suite.sdkCtx.BlockHeight(),
						},
					},
				}
				err := suite.app.SupernodeKeeper.SetSuperNode(suite.sdkCtx, sn)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, suite *SystemTestSuite, resp *types2.MsgStopSupernodeResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, resp)

				// Check final state
				val, convErr := sdk.ValAddressFromBech32(valAddrStr)
				require.NoError(t, convErr)
				sn, found := suite.app.SupernodeKeeper.QuerySuperNode(suite.sdkCtx, val)
				require.True(t, found)
				require.NotEmpty(t, sn.States)
				require.Equal(t, types2.SuperNodeStateStopped, sn.States[len(sn.States)-1].State)

				// Verify event was emitted
				events := suite.sdkCtx.EventManager().Events()
				var foundStopEvent bool
				for _, e := range events {
					if e.Type == types2.EventTypeSupernodeStopped {
						foundStopEvent = true
						for _, attr := range e.Attributes {
							if string(attr.Key) == types2.AttributeKeyValidatorAddress {
								require.Equal(t, valAddrStr, string(attr.Value))
							}
						}
					}
				}
				require.True(t, foundStopEvent, "supernode_stopped event not found")
			},
		},
		{
			name: "invalid validator address",
			msg: &types2.MsgStopSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: "invalid",
				Reason:           "maintenance",
			},
			setup: nil,
			verify: func(t *testing.T, suite *SystemTestSuite, resp *types2.MsgStopSupernodeResponse, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
				require.Nil(t, resp)
			},
		},
		{
			name: "supernode not found",
			msg: &types2.MsgStopSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: valAddrStr,
				Reason:           "node down",
			},
			setup: nil,
			verify: func(t *testing.T, suite *SystemTestSuite, resp *types2.MsgStopSupernodeResponse, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, sdkerrors.ErrNotFound)
				require.Nil(t, resp)
			},
		},
		{
			name: "unauthorized attempt",
			msg: &types2.MsgStopSupernode{
				Creator:          unauthAddr.String(),
				ValidatorAddress: valAddrStr,
				Reason:           "not your node",
			},
			setup: func(suite *SystemTestSuite) {
				// Create supernode belonging to walletAddr
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
							Address: "192.168.0.3",
							Height:  suite.sdkCtx.BlockHeight(),
						},
					},
				}
				err := suite.app.SupernodeKeeper.SetSuperNode(suite.sdkCtx, sn)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, suite *SystemTestSuite, resp *types2.MsgStopSupernodeResponse, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
				require.Nil(t, resp)
			},
		},
		{
			name: "already stopped supernode",
			msg: &types2.MsgStopSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: valAddrStr,
				Reason:           "maintenance",
			},
			setup: func(suite *SystemTestSuite) {
				sn := types2.SuperNode{
					ValidatorAddress: valAddrStr,
					SupernodeAccount: walletAddr.String(),
					Version:          "1.0.0",
					States: []*types2.SuperNodeStateRecord{
						{
							State:  types2.SuperNodeStateActive,
							Height: suite.sdkCtx.BlockHeight(),
						},
						{
							State:  types2.SuperNodeStateStopped,
							Height: suite.sdkCtx.BlockHeight() + 1,
						},
					},
					PrevIpAddresses: []*types2.IPAddressHistory{
						{
							Address: "192.168.0.4",
							Height:  suite.sdkCtx.BlockHeight(),
						},
					},
				}
				err := suite.app.SupernodeKeeper.SetSuperNode(suite.sdkCtx, sn)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, suite *SystemTestSuite, resp *types2.MsgStopSupernodeResponse, err error) {
				// Per your logic, you might disallow another stop if it's already stopped
				// So we expect an error
				require.Error(t, err)
				require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
				require.Nil(t, resp)
			},
		},
		{
			name: "disabled supernode",
			msg: &types2.MsgStopSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: valAddrStr,
				Reason:           "maintenance",
			},
			setup: func(suite *SystemTestSuite) {
				sn := types2.SuperNode{
					ValidatorAddress: valAddrStr,
					SupernodeAccount: walletAddr.String(),
					Version:          "1.0.0",
					States: []*types2.SuperNodeStateRecord{
						{
							State:  types2.SuperNodeStateActive,
							Height: suite.sdkCtx.BlockHeight(),
						},
						{
							State:  types2.SuperNodeStateDisabled,
							Height: suite.sdkCtx.BlockHeight() + 1,
						},
					},
					PrevIpAddresses: []*types2.IPAddressHistory{
						{
							Address: "192.168.0.5",
							Height:  suite.sdkCtx.BlockHeight(),
						},
					},
				}
				err := suite.app.SupernodeKeeper.SetSuperNode(suite.sdkCtx, sn)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, suite *SystemTestSuite, resp *types2.MsgStopSupernodeResponse, err error) {
				// If your logic doesn't allow stopping a disabled SN, expect an error
				require.Error(t, err)
				require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
				require.Nil(t, resp)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Create a fresh system test suite
			testSuite := setupSupernodeSystemSuite(t)

			// Set up the validator in staking
			validator, err := stakingtypes.NewValidator(valAddrStr, walletPrivKey.PubKey(), stakingtypes.Description{})
			require.NoError(t, err)
			validator.Status = stakingtypes.Bonded
			validator.Tokens = sdkmath.NewInt(1000000)
			testSuite.app.StakingKeeper.SetValidator(testSuite.sdkCtx, validator)

			// Run test-specific setup
			if tc.setup != nil {
				tc.setup(testSuite)
			}

			// Call StopSupernode
			msgServer := keeper.NewMsgServerImpl(testSuite.app.SupernodeKeeper)
			resp, err := msgServer.StopSupernode(testSuite.ctx, tc.msg)

			// Verify the outcome
			tc.verify(t, testSuite, resp, err)
		})
	}
}
