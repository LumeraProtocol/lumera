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

func TestStartSupernode(t *testing.T) {
	// Base addresses used in each test
	walletPrivKey := secp256k1.GenPrivKey()
	walletAddr := sdk.AccAddress(walletPrivKey.PubKey().Address())
	valAddr := sdk.ValAddress(walletAddr)
	valAddrStr := valAddr.String()

	// Unauthorized address
	unauthPrivKey := secp256k1.GenPrivKey()
	unauthAddr := sdk.AccAddress(unauthPrivKey.PubKey().Address())

	// Common message constructor
	newStartSupernodeMsg := func(creator string, validator string) *types2.MsgStartSupernode {
		return &types2.MsgStartSupernode{
			Creator:          creator,
			ValidatorAddress: validator,
		}
	}

	testCases := []struct {
		name   string
		msg    *types2.MsgStartSupernode
		setup  func(*SystemTestSuite)
		verify func(t *testing.T, suite *SystemTestSuite, resp *types2.MsgStartSupernodeResponse, err error)
	}{
		{
			name: "successful start from disabled",
			msg:  newStartSupernodeMsg(walletAddr.String(), valAddrStr),
			setup: func(suite *SystemTestSuite) {
				// Create a supernode in disabled state
				disabledSN := types2.SuperNode{
					ValidatorAddress: valAddrStr,
					SupernodeAccount: walletAddr.String(),
					States: []*types2.SuperNodeStateRecord{
						{
							State:  types2.SuperNodeStateDisabled,
							Height: suite.sdkCtx.BlockHeight(),
						},
					},
					Version: "1.0.0",
					PrevIpAddresses: []*types2.IPAddressHistory{
						{
							Address: "127.0.0.1",
							Height:  suite.sdkCtx.BlockHeight(),
						},
					},
				}
				err := suite.app.SupernodeKeeper.SetSuperNode(suite.sdkCtx, disabledSN)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, suite *SystemTestSuite, resp *types2.MsgStartSupernodeResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, resp)

				// Check supernode final state: should be active
				valOp, err := sdk.ValAddressFromBech32(valAddrStr)
				require.NoError(t, err)
				sn, found := suite.app.SupernodeKeeper.QuerySuperNode(suite.sdkCtx, valOp)
				require.True(t, found)
				require.NotEmpty(t, sn.States)
				require.Equal(t, types2.SuperNodeStateActive, sn.States[len(sn.States)-1].State)

				// Check for event
				events := suite.sdkCtx.EventManager().Events()
				var foundStartEvent bool
				for _, evt := range events {
					if evt.Type == types2.EventTypeSupernodeStarted {
						foundStartEvent = true
						for _, attr := range evt.Attributes {
							if string(attr.Key) == types2.AttributeKeyValidatorAddress {
								require.Equal(t, valAddrStr, string(attr.Value))
							}
						}
					}
				}
				require.True(t, foundStartEvent, "supernode_started event not found")
			},
		},
		{
			name: "supernode not found",
			msg:  newStartSupernodeMsg(walletAddr.String(), valAddrStr),
			setup: func(suite *SystemTestSuite) {
				// Do not create any supernode
			},
			verify: func(t *testing.T, suite *SystemTestSuite, resp *types2.MsgStartSupernodeResponse, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, sdkerrors.ErrNotFound)
				require.Nil(t, resp)
			},
		},
		{
			name: "unauthorized start attempt",
			msg:  newStartSupernodeMsg(unauthAddr.String(), valAddrStr),
			setup: func(suite *SystemTestSuite) {
				// Create a disabled supernode that belongs to `walletAddr`
				disabledSN := types2.SuperNode{
					ValidatorAddress: valAddrStr,
					SupernodeAccount: walletAddr.String(),
					States: []*types2.SuperNodeStateRecord{
						{
							State:  types2.SuperNodeStateDisabled,
							Height: suite.sdkCtx.BlockHeight(),
						},
					},
					Version: "1.0.0",
					PrevIpAddresses: []*types2.IPAddressHistory{
						{
							Address: "127.0.0.1",
							Height:  suite.sdkCtx.BlockHeight(),
						},
					},
				}
				err := suite.app.SupernodeKeeper.SetSuperNode(suite.sdkCtx, disabledSN)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, suite *SystemTestSuite, resp *types2.MsgStartSupernodeResponse, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
				require.Nil(t, resp)
			},
		},
		{
			name: "already active supernode",
			msg:  newStartSupernodeMsg(walletAddr.String(), valAddrStr),
			setup: func(suite *SystemTestSuite) {
				// Create a supernode that is already active
				activeSN := types2.SuperNode{
					ValidatorAddress: valAddrStr,
					SupernodeAccount: walletAddr.String(),
					States: []*types2.SuperNodeStateRecord{
						{
							State:  types2.SuperNodeStateActive,
							Height: suite.sdkCtx.BlockHeight(),
						},
					},
					Version: "1.0.0",
					PrevIpAddresses: []*types2.IPAddressHistory{
						{
							Address: "127.0.0.1",
							Height:  suite.sdkCtx.BlockHeight(),
						},
					},
				}
				err := suite.app.SupernodeKeeper.SetSuperNode(suite.sdkCtx, activeSN)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, suite *SystemTestSuite, resp *types2.MsgStartSupernodeResponse, err error) {
				// Depending on your logic, this might be an error or no-op
				// Below we'll assume it's a no-op with no error
				require.Error(t, err)
				require.Nil(t, resp)

				// Validate it remains active
				valOp, vErr := sdk.ValAddressFromBech32(valAddrStr)
				require.NoError(t, vErr)
				sn, found := suite.app.SupernodeKeeper.QuerySuperNode(suite.sdkCtx, valOp)
				require.True(t, found)
				require.NotEmpty(t, sn.States)
				require.Equal(t, types2.SuperNodeStateActive, sn.States[len(sn.States)-1].State)
			},
		},
		{
			name: "invalid validator address",
			msg:  newStartSupernodeMsg(walletAddr.String(), "invalid-address"),
			setup: func(suite *SystemTestSuite) {
				// No setup needed
			},
			verify: func(t *testing.T, suite *SystemTestSuite, resp *types2.MsgStartSupernodeResponse, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
				require.Nil(t, resp)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a fresh suite for each subtest
			testSuite := setupSupernodeSystemSuite(t)

			// Create & set up validator in the staking store
			validator, err := stakingtypes.NewValidator(valAddrStr, walletPrivKey.PubKey(), stakingtypes.Description{})
			require.NoError(t, err)
			validator.Status = stakingtypes.Bonded
			validator.Tokens = sdkmath.NewInt(1000000)
			testSuite.app.StakingKeeper.SetValidator(testSuite.sdkCtx, validator)

			// Run custom setup for this test
			if tc.setup != nil {
				tc.setup(testSuite)
			}

			// Invoke the StartSupernode message
			msgServer := keeper.NewMsgServerImpl(testSuite.app.SupernodeKeeper)
			resp, err := msgServer.StartSupernode(testSuite.ctx, tc.msg)

			// Run verification
			tc.verify(t, testSuite, resp, err)
		})
	}
}
