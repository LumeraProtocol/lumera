package system_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
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
		msg    *sntypes.MsgStopSupernode
		setup  func(*SystemTestSuite)
		verify func(t *testing.T, suite *SystemTestSuite, resp *sntypes.MsgStopSupernodeResponse, err error)
	}{
		{
			name: "successful stop",
			msg: &sntypes.MsgStopSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: valAddrStr,
				Reason:           "maintenance",
			},
			setup: func(suite *SystemTestSuite) {
				// Create a supernode in active state
				sn := sntypes.SuperNode{
					ValidatorAddress: valAddrStr,
					SupernodeAccount: walletAddr.String(),
					Note:             "1.0.0",
					States: []*sntypes.SuperNodeStateRecord{
						{
							State:  sntypes.SuperNodeStateActive,
							Height: suite.sdkCtx.BlockHeight(),
						},
					},
					PrevIpAddresses: []*sntypes.IPAddressHistory{
						{
							Address: "192.168.0.2",
							Height:  suite.sdkCtx.BlockHeight(),
						},
					},
					P2PPort: "26657",
				}
				err := suite.app.SupernodeKeeper.SetSuperNode(suite.sdkCtx, sn)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, suite *SystemTestSuite, resp *sntypes.MsgStopSupernodeResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, resp)

				// Check final state
				val, convErr := sdk.ValAddressFromBech32(valAddrStr)
				require.NoError(t, convErr)
				sn, found := suite.app.SupernodeKeeper.QuerySuperNode(suite.sdkCtx, val)
				require.True(t, found)
				require.NotEmpty(t, sn.States)
				require.Equal(t, sntypes.SuperNodeStateStopped, sn.States[len(sn.States)-1].State)

				// Verify event was emitted
				events := suite.sdkCtx.EventManager().Events()
				var foundStopEvent bool
				for _, e := range events {
					if e.Type == sntypes.EventTypeSupernodeStopped {
						foundStopEvent = true
						var addrOK, heightOK, oldStateOK bool
						for _, attr := range e.Attributes {
							if string(attr.Key) == sntypes.AttributeKeyValidatorAddress {
								require.Equal(t, valAddrStr, string(attr.Value))
								addrOK = true
							}
							if string(attr.Key) == sntypes.AttributeKeyHeight {
								require.NotEmpty(t, string(attr.Value))
								heightOK = true
							}
							if string(attr.Key) == sntypes.AttributeKeyOldState {
								require.NotEmpty(t, string(attr.Value))
								oldStateOK = true
							}
						}
						require.True(t, addrOK && heightOK && oldStateOK)
					}
				}
				require.True(t, foundStopEvent, "supernode_stopped event not found")
			},
		},
		{
			name: "invalid validator address",
			msg: &sntypes.MsgStopSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: "invalid",
				Reason:           "maintenance",
			},
			setup: nil,
			verify: func(t *testing.T, suite *SystemTestSuite, resp *sntypes.MsgStopSupernodeResponse, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
				require.Nil(t, resp)
			},
		},
		{
			name: "supernode not found",
			msg: &sntypes.MsgStopSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: valAddrStr,
				Reason:           "node down",
			},
			setup: nil,
			verify: func(t *testing.T, suite *SystemTestSuite, resp *sntypes.MsgStopSupernodeResponse, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, sdkerrors.ErrNotFound)
				require.Nil(t, resp)
			},
		},
		{
			name: "unauthorized attempt",
			msg: &sntypes.MsgStopSupernode{
				Creator:          unauthAddr.String(),
				ValidatorAddress: valAddrStr,
				Reason:           "not your node",
			},
			setup: func(suite *SystemTestSuite) {
				// Create supernode belonging to walletAddr
				sn := sntypes.SuperNode{
					ValidatorAddress: valAddrStr,
					SupernodeAccount: walletAddr.String(),
					Note:             "1.0.0",
					States: []*sntypes.SuperNodeStateRecord{
						{
							State:  sntypes.SuperNodeStateActive,
							Height: suite.sdkCtx.BlockHeight(),
						},
					},
					PrevIpAddresses: []*sntypes.IPAddressHistory{
						{
							Address: "192.168.0.3",
							Height:  suite.sdkCtx.BlockHeight(),
						},
					},
					P2PPort: "26657",
				}
				err := suite.app.SupernodeKeeper.SetSuperNode(suite.sdkCtx, sn)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, suite *SystemTestSuite, resp *sntypes.MsgStopSupernodeResponse, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
				require.Nil(t, resp)
			},
		},
		{
			name: "already stopped supernode",
			msg: &sntypes.MsgStopSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: valAddrStr,
				Reason:           "maintenance",
			},
			setup: func(suite *SystemTestSuite) {
				sn := sntypes.SuperNode{
					ValidatorAddress: valAddrStr,
					SupernodeAccount: walletAddr.String(),
					Note:             "1.0.0",
					States: []*sntypes.SuperNodeStateRecord{
						{
							State:  sntypes.SuperNodeStateActive,
							Height: suite.sdkCtx.BlockHeight(),
						},
						{
							State:  sntypes.SuperNodeStateStopped,
							Height: suite.sdkCtx.BlockHeight() + 1,
						},
					},
					PrevIpAddresses: []*sntypes.IPAddressHistory{
						{
							Address: "192.168.0.4",
							Height:  suite.sdkCtx.BlockHeight(),
						},
					},
					P2PPort: "26657",
				}
				err := suite.app.SupernodeKeeper.SetSuperNode(suite.sdkCtx, sn)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, suite *SystemTestSuite, resp *sntypes.MsgStopSupernodeResponse, err error) {
				// Per your logic, you might disallow another stop if it's already stopped
				// So we expect an error
				require.Error(t, err)
				require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
				require.Nil(t, resp)
			},
		},
		{
			name: "disabled supernode",
			msg: &sntypes.MsgStopSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: valAddrStr,
				Reason:           "maintenance",
			},
			setup: func(suite *SystemTestSuite) {
				sn := sntypes.SuperNode{
					ValidatorAddress: valAddrStr,
					SupernodeAccount: walletAddr.String(),
					Note:             "1.0.0",
					States: []*sntypes.SuperNodeStateRecord{
						{
							State:  sntypes.SuperNodeStateActive,
							Height: suite.sdkCtx.BlockHeight(),
						},
						{
							State:  sntypes.SuperNodeStateDisabled,
							Height: suite.sdkCtx.BlockHeight() + 1,
						},
					},
					PrevIpAddresses: []*sntypes.IPAddressHistory{
						{
							Address: "192.168.0.5",
							Height:  suite.sdkCtx.BlockHeight(),
						},
					},
					P2PPort: "26657",
				}
				err := suite.app.SupernodeKeeper.SetSuperNode(suite.sdkCtx, sn)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, suite *SystemTestSuite, resp *sntypes.MsgStopSupernodeResponse, err error) {
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
