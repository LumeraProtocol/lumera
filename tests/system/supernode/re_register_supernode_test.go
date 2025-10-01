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

func TestReRegisterSupernode(t *testing.T) {
	// Base accounts
	walletPrivKey := secp256k1.GenPrivKey()
	walletAddr := sdk.AccAddress(walletPrivKey.PubKey().Address())
	valAddr := sdk.ValAddress(walletAddr)
	valAddrStr := valAddr.String()

	testCases := []struct {
		name   string
		msg    *sntypes.MsgRegisterSupernode
		setup  func(*SystemTestSuite)
		verify func(t *testing.T, suite *SystemTestSuite, resp *sntypes.MsgRegisterSupernodeResponse, err error)
	}{
		{
			name: "successful re-registration of disabled supernode",
			msg: &sntypes.MsgRegisterSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: valAddrStr,
				IpAddress:        "10.0.0.99",                                                        // Different from original - should be ignored
				SupernodeAccount: sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String(), // Different - should be ignored
				P2PPort:          "9999",                                                             // Different - should be ignored
			},
			setup: func(suite *SystemTestSuite) {
				// Create a disabled supernode with original parameters
				originalSupernode := sntypes.SuperNode{
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
							Address: "192.168.1.100",
							Height:  suite.sdkCtx.BlockHeight(),
						},
					},
					PrevSupernodeAccounts: []*sntypes.SupernodeAccountHistory{
						{
							Account: walletAddr.String(),
							Height:  suite.sdkCtx.BlockHeight(),
						},
					},
					P2PPort: "26657",
				}
				err := suite.app.SupernodeKeeper.SetSuperNode(suite.sdkCtx, originalSupernode)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, suite *SystemTestSuite, resp *sntypes.MsgRegisterSupernodeResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, resp)

				// Verify supernode is now active
				valOp, vErr := sdk.ValAddressFromBech32(valAddrStr)
				require.NoError(t, vErr)
				sn, found := suite.app.SupernodeKeeper.QuerySuperNode(suite.sdkCtx, valOp)
				require.True(t, found)

				// Verify state progression: Active → Disabled → Active
				require.Len(t, sn.States, 3)
				require.Equal(t, sntypes.SuperNodeStateActive, sn.States[0].State)
				require.Equal(t, sntypes.SuperNodeStateDisabled, sn.States[1].State)
				require.Equal(t, sntypes.SuperNodeStateActive, sn.States[2].State)

				// Verify ALL original parameters were preserved during re-registration
				require.Equal(t, "192.168.1.100", sn.PrevIpAddresses[len(sn.PrevIpAddresses)-1].Address)
				require.Equal(t, walletAddr.String(), sn.SupernodeAccount)
				require.Equal(t, "26657", sn.P2PPort)
				require.Equal(t, "1.0.0", sn.Note)

				// Verify no new history entries were added
				require.Len(t, sn.PrevIpAddresses, 1)
				require.Len(t, sn.PrevSupernodeAccounts, 1)

				// Verify re-registration event was emitted
				events := suite.sdkCtx.EventManager().Events()
				var foundEvent bool
				for _, e := range events {
					if e.Type == sntypes.EventTypeSupernodeRegistered {
						foundEvent = true
						for _, attr := range e.Attributes {
							if string(attr.Key) == sntypes.AttributeKeyReRegistered {
								require.Equal(t, "true", string(attr.Value))
							}
							if string(attr.Key) == sntypes.AttributeKeyOldState {
								require.Equal(t, sntypes.SuperNodeStateDisabled.String(), string(attr.Value))
							}
						}
					}
				}
				require.True(t, foundEvent, "re-registration event not found")
			},
		},
		{
			name: "cannot re-register STOPPED supernode",
			msg: &sntypes.MsgRegisterSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: valAddrStr,
				IpAddress:        "192.168.1.1",
				SupernodeAccount: walletAddr.String(),
				P2PPort:          "26657",
			},
			setup: func(suite *SystemTestSuite) {
				// Create a stopped supernode
				stoppedSupernode := sntypes.SuperNode{
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
							Address: "192.168.1.1",
							Height:  suite.sdkCtx.BlockHeight(),
						},
					},
					PrevSupernodeAccounts: []*sntypes.SupernodeAccountHistory{
						{
							Account: walletAddr.String(),
							Height:  suite.sdkCtx.BlockHeight(),
						},
					},
					P2PPort: "26657",
				}
				err := suite.app.SupernodeKeeper.SetSuperNode(suite.sdkCtx, stoppedSupernode)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, suite *SystemTestSuite, resp *sntypes.MsgRegisterSupernodeResponse, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
				require.Nil(t, resp)

				// Verify supernode state remains unchanged
				valOp, vErr := sdk.ValAddressFromBech32(valAddrStr)
				require.NoError(t, vErr)
				sn, found := suite.app.SupernodeKeeper.QuerySuperNode(suite.sdkCtx, valOp)
				require.True(t, found)
				require.Equal(t, sntypes.SuperNodeStateStopped, sn.States[len(sn.States)-1].State)
			},
		},
		{
			name: "cannot re-register PENALIZED supernode",
			msg: &sntypes.MsgRegisterSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: valAddrStr,
				IpAddress:        "192.168.1.1",
				SupernodeAccount: walletAddr.String(),
				P2PPort:          "26657",
			},
			setup: func(suite *SystemTestSuite) {
				// Create a penalized supernode
				penalizedSupernode := sntypes.SuperNode{
					ValidatorAddress: valAddrStr,
					SupernodeAccount: walletAddr.String(),
					Note:             "1.0.0",
					States: []*sntypes.SuperNodeStateRecord{
						{
							State:  sntypes.SuperNodeStateActive,
							Height: suite.sdkCtx.BlockHeight(),
						},
						{
							State:  sntypes.SuperNodeStatePenalized,
							Height: suite.sdkCtx.BlockHeight() + 1,
						},
					},
					PrevIpAddresses: []*sntypes.IPAddressHistory{
						{
							Address: "192.168.1.1",
							Height:  suite.sdkCtx.BlockHeight(),
						},
					},
					PrevSupernodeAccounts: []*sntypes.SupernodeAccountHistory{
						{
							Account: walletAddr.String(),
							Height:  suite.sdkCtx.BlockHeight(),
						},
					},
					P2PPort: "26657",
				}
				err := suite.app.SupernodeKeeper.SetSuperNode(suite.sdkCtx, penalizedSupernode)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, suite *SystemTestSuite, resp *sntypes.MsgRegisterSupernodeResponse, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
				require.Nil(t, resp)

				// Verify supernode state remains unchanged
				valOp, vErr := sdk.ValAddressFromBech32(valAddrStr)
				require.NoError(t, vErr)
				sn, found := suite.app.SupernodeKeeper.QuerySuperNode(suite.sdkCtx, valOp)
				require.True(t, found)
				require.Equal(t, sntypes.SuperNodeStatePenalized, sn.States[len(sn.States)-1].State)
			},
		},
		{
			name: "multiple consecutive re-registrations",
			msg: &sntypes.MsgRegisterSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: valAddrStr,
				IpAddress:        "192.168.1.1",
				SupernodeAccount: walletAddr.String(),
				P2PPort:          "26657",
			},
			setup: func(suite *SystemTestSuite) {
				// Create a supernode that has been re-registered multiple times
				multipleSupernode := sntypes.SuperNode{
					ValidatorAddress: valAddrStr,
					SupernodeAccount: walletAddr.String(),
					Note:             "1.0.0",
					States: []*sntypes.SuperNodeStateRecord{
						{
							State:  sntypes.SuperNodeStateActive,
							Height: 100,
						},
						{
							State:  sntypes.SuperNodeStateDisabled,
							Height: 200,
						},
						{
							State:  sntypes.SuperNodeStateActive,
							Height: 300,
						},
						{
							State:  sntypes.SuperNodeStateDisabled,
							Height: 400,
						},
					},
					PrevIpAddresses: []*sntypes.IPAddressHistory{
						{
							Address: "192.168.1.1",
							Height:  100,
						},
					},
					PrevSupernodeAccounts: []*sntypes.SupernodeAccountHistory{
						{
							Account: walletAddr.String(),
							Height:  100,
						},
					},
					P2PPort: "26657",
				}
				err := suite.app.SupernodeKeeper.SetSuperNode(suite.sdkCtx, multipleSupernode)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, suite *SystemTestSuite, resp *sntypes.MsgRegisterSupernodeResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, resp)

				// Verify supernode is now active again
				valOp, vErr := sdk.ValAddressFromBech32(valAddrStr)
				require.NoError(t, vErr)
				sn, found := suite.app.SupernodeKeeper.QuerySuperNode(suite.sdkCtx, valOp)
				require.True(t, found)

				// Verify state progression: Active → Disabled → Active → Disabled → Active
				require.Len(t, sn.States, 5)
				require.Equal(t, sntypes.SuperNodeStateActive, sn.States[4].State) // Latest state should be active
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create fresh suite for each test
			testSuite := setupSupernodeSystemSuite(t)

			// Create and set up validator in Staking with sufficient self-stake
			validator, err := stakingtypes.NewValidator(valAddrStr, walletPrivKey.PubKey(), stakingtypes.Description{})
			require.NoError(t, err)
			validator.Status = stakingtypes.Bonded
			validator.Tokens = sdkmath.NewInt(2000000)
			validator.DelegatorShares = sdkmath.LegacyNewDec(2000000)
			testSuite.app.StakingKeeper.SetValidator(testSuite.sdkCtx, validator)

			// Create self-delegation for the validator
			delegation := stakingtypes.NewDelegation(walletAddr.String(), valAddrStr, sdkmath.LegacyNewDec(1000000))
			testSuite.app.StakingKeeper.SetDelegation(testSuite.sdkCtx, delegation)

			// Perform any test-specific setup
			if tc.setup != nil {
				tc.setup(testSuite)
			}

			// Invoke the RegisterSupernode message
			msgServer := keeper.NewMsgServerImpl(testSuite.app.SupernodeKeeper)
			resp, err := msgServer.RegisterSupernode(testSuite.ctx, tc.msg)

			// Verification
			tc.verify(t, testSuite, resp, err)
		})
	}
}
