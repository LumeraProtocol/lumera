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
		msg    *sntypes.MsgUpdateSupernode
		setup  func(*SystemTestSuite)
		verify func(t *testing.T, suite *SystemTestSuite, resp *sntypes.MsgUpdateSupernodeResponse, err error)
	}{
		{
			name: "basic update - new ip, new version, new supernode account",
			msg: &sntypes.MsgUpdateSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: valAddrStr,
				IpAddress:        "10.0.0.2",
				Note:             "2.0.0",
				SupernodeAccount: sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String(),
			},
			setup: func(suite *SystemTestSuite) {
				// Register a supernode in some initial state
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
							Address: "192.168.1.1",
							Height:  suite.sdkCtx.BlockHeight(),
						},
					},
					P2PPort: "26657",
				}
				err := suite.app.SupernodeKeeper.SetSuperNode(suite.sdkCtx, sn)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, suite *SystemTestSuite, resp *sntypes.MsgUpdateSupernodeResponse, err error) {
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
				// Verify Note
				require.Equal(t, "2.0.0", sn.Note)
				// Verify new supernode account
				require.NotEqual(t, walletAddr.String(), sn.SupernodeAccount)

				// Verify event
				events := suite.sdkCtx.EventManager().Events()
				var foundUpdateEvent bool
				for _, e := range events {
					if e.Type == sntypes.EventTypeSupernodeUpdated {
						foundUpdateEvent = true
						var addrOK, fieldsOK, heightOK bool
						var oldAccOK, newAccOK, oldIPOK, newIPOK bool
						var fieldsUpdated string
						kv := map[string]string{}
						for _, attr := range e.Attributes {
						    kv[string(attr.Key)] = string(attr.Value)
						    if string(attr.Key) == sntypes.AttributeKeyValidatorAddress {
						        require.Equal(t, valAddrStr, string(attr.Value))
						        addrOK = true
						    }
						    if string(attr.Key) == sntypes.AttributeKeyFieldsUpdated {
						        fieldsUpdated = string(attr.Value)
						        fieldsOK = true
						    }
						    if string(attr.Key) == sntypes.AttributeKeyHeight {
						        require.NotEmpty(t, string(attr.Value))
						        heightOK = true
						    }
						    if string(attr.Key) == sntypes.AttributeKeyOldAccount {
						        require.Equal(t, walletAddr.String(), string(attr.Value))
						        oldAccOK = true
						    }
						    if string(attr.Key) == sntypes.AttributeKeyNewAccount {
						        require.NotEmpty(t, string(attr.Value))
						        newAccOK = true
						    }
						    if string(attr.Key) == sntypes.AttributeKeyOldIPAddress {
						        require.Equal(t, "192.168.1.1", string(attr.Value))
						        oldIPOK = true
						    }
						    if string(attr.Key) == sntypes.AttributeKeyIPAddress {
						        require.Equal(t, "10.0.0.2", string(attr.Value))
						        newIPOK = true
						    }
						}
						require.True(t, addrOK && fieldsOK && heightOK)
						require.Contains(t, fieldsUpdated, sntypes.AttributeKeyIPAddress)
						require.Contains(t, fieldsUpdated, sntypes.AttributeKeySupernodeAccount)
						require.Contains(t, fieldsUpdated, "note")
						require.True(t, oldAccOK && newAccOK && oldIPOK && newIPOK)
					}
				}
				require.True(t, foundUpdateEvent, "supernode_updated event not found")
			},
		},
		{
			name: "supernode not found",
			msg: &sntypes.MsgUpdateSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: valAddrStr,
				IpAddress:        "10.0.0.3",
			},
			setup: func(suite *SystemTestSuite) { /* do nothing */ },
			verify: func(t *testing.T, suite *SystemTestSuite, resp *sntypes.MsgUpdateSupernodeResponse, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, sdkerrors.ErrNotFound)
				require.Nil(t, resp)
			},
		},
		{
			name: "unauthorized update attempt",
			msg: &sntypes.MsgUpdateSupernode{
				Creator:          unauthAddr.String(),
				ValidatorAddress: valAddrStr,
				IpAddress:        "8.8.8.8",
			},
			setup: func(suite *SystemTestSuite) {
				// Create supernode owned by walletAddr
				sn := sntypes.SuperNode{
					ValidatorAddress: valAddrStr,
					SupernodeAccount: walletAddr.String(),
					Note:          "1.0.0",
					States: []*sntypes.SuperNodeStateRecord{
						{
							State:  sntypes.SuperNodeStateActive,
							Height: suite.sdkCtx.BlockHeight(),
						},
					},
					PrevIpAddresses: []*sntypes.IPAddressHistory{
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
			verify: func(t *testing.T, suite *SystemTestSuite, resp *sntypes.MsgUpdateSupernodeResponse, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
				require.Nil(t, resp)
			},
		},
		{
			name: "invalid validator address",
			msg: &sntypes.MsgUpdateSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: "invalid-addr",
			},
			setup: nil,
			verify: func(t *testing.T, suite *SystemTestSuite, resp *sntypes.MsgUpdateSupernodeResponse, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
				require.Nil(t, resp)
			},
		},
		{
			name: "update with no changes",
			msg: &sntypes.MsgUpdateSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: valAddrStr,
				// no changes to ip, version, or supernode account
			},
			setup: func(suite *SystemTestSuite) {
				// Existing supernode
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
							Address: "127.0.0.1",
							Height:  suite.sdkCtx.BlockHeight(),
						},
					},
					P2PPort: "26657",
				}
				err := suite.app.SupernodeKeeper.SetSuperNode(suite.sdkCtx, sn)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, suite *SystemTestSuite, resp *sntypes.MsgUpdateSupernodeResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, resp)

				// Verify nothing changed
				valOp, convErr := sdk.ValAddressFromBech32(valAddrStr)
				require.NoError(t, convErr)
				sn, found := suite.app.SupernodeKeeper.QuerySuperNode(suite.sdkCtx, valOp)
				require.True(t, found)

				// IP should remain the same, Note the same, etc.
				require.Equal(t, "1.0.0", sn.Note)
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

// Additional test case for P2P port update
func TestUpdateSupernode_P2PPort(t *testing.T) {
    // Base accounts
    walletPrivKey := secp256k1.GenPrivKey()
    walletAddr := sdk.AccAddress(walletPrivKey.PubKey().Address())
    valAddr := sdk.ValAddress(walletAddr)
    valAddrStr := valAddr.String()

    testSuite := setupSupernodeSystemSuite(t)
    // Create and set up validator in Staking
    validator, err := stakingtypes.NewValidator(valAddrStr, walletPrivKey.PubKey(), stakingtypes.Description{})
    require.NoError(t, err)
    validator.Status = stakingtypes.Bonded
    validator.Tokens = sdkmath.NewInt(1000000)
    testSuite.app.StakingKeeper.SetValidator(testSuite.sdkCtx, validator)

    // Set initial SN
    sn := sntypes.SuperNode{
        ValidatorAddress: valAddrStr,
        SupernodeAccount: walletAddr.String(),
        Note:             "1.0.0",
        States: []*sntypes.SuperNodeStateRecord{{State: sntypes.SuperNodeStateActive, Height: testSuite.sdkCtx.BlockHeight()}},
        PrevIpAddresses:  []*sntypes.IPAddressHistory{{Address: "127.0.0.1", Height: testSuite.sdkCtx.BlockHeight()}},
        P2PPort:          "26657",
    }
    err = testSuite.app.SupernodeKeeper.SetSuperNode(testSuite.sdkCtx, sn)
    require.NoError(t, err)

    // Update P2P port
    msg := &sntypes.MsgUpdateSupernode{
        Creator:          walletAddr.String(),
        ValidatorAddress: valAddrStr,
        P2PPort:          "26699",
    }
    msgServer := keeper.NewMsgServerImpl(testSuite.app.SupernodeKeeper)
    resp, err := msgServer.UpdateSupernode(testSuite.ctx, msg)
    require.NoError(t, err)
    require.NotNil(t, resp)

    // Verify event
    events := testSuite.sdkCtx.EventManager().Events()
    var foundUpdateEvent bool
    for _, e := range events {
        if e.Type == sntypes.EventTypeSupernodeUpdated {
            foundUpdateEvent = true
            kv := map[string]string{}
            for _, a := range e.Attributes {
                kv[string(a.Key)] = string(a.Value)
            }
            require.Equal(t, valAddrStr, kv[sntypes.AttributeKeyValidatorAddress])
            require.NotEmpty(t, kv[sntypes.AttributeKeyHeight])
            require.Contains(t, kv[sntypes.AttributeKeyFieldsUpdated], sntypes.AttributeKeyP2PPort)
            require.Equal(t, "26657", kv[sntypes.AttributeKeyOldP2PPort])
            require.Equal(t, "26699", kv[sntypes.AttributeKeyP2PPort])
        }
    }
    require.True(t, foundUpdateEvent, "supernode_updated event not found for P2P change")
}