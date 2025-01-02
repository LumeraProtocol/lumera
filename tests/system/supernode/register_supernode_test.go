package system_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/pastelnetwork/pastel/x/supernode/keeper"
	"github.com/pastelnetwork/pastel/x/supernode/types"
	"github.com/stretchr/testify/require"
)

func TestRegisterSupernode(t *testing.T) {
	// Create base wallet and validator
	walletPrivKey := secp256k1.GenPrivKey()
	walletAddr := sdk.AccAddress(walletPrivKey.PubKey().Address())
	valAddr := sdk.ValAddress(walletAddr)
	valAddrStr := valAddr.String()

	testCases := []struct {
		name        string
		msg         *types.MsgRegisterSupernode
		setup       func()
		verify      func(t *testing.T, supernode types.SuperNode)
		createTwice bool
	}{
		{
			name: "basic registration - same wallet and supernode account",
			msg: &types.MsgRegisterSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: valAddrStr,
				SupernodeAccount: walletAddr.String(),
				IpAddress:        "192.168.1.1",
				Version:          "1.0.0",
			},
			verify: func(t *testing.T, supernode types.SuperNode) {
				require.Equal(t, valAddrStr, supernode.ValidatorAddress)
				require.Equal(t, walletAddr.String(), supernode.SupernodeAccount)
				require.Equal(t, "1.0.0", supernode.Version)
				require.Equal(t, "192.168.1.1", supernode.PrevIpAddresses[0].Address)
				require.Equal(t, types.SuperNodeStateActive, supernode.States[0].State)
			},
		},
		{
			name: "registration with different supernode account",
			msg: &types.MsgRegisterSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: valAddrStr,
				SupernodeAccount: sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String(),
				IpAddress:        "192.168.1.2",
				Version:          "1.0.1",
			},
			verify: func(t *testing.T, supernode types.SuperNode) {
				require.Equal(t, valAddrStr, supernode.ValidatorAddress)
				require.NotEqual(t, walletAddr.String(), supernode.SupernodeAccount)
				require.Equal(t, "1.0.1", supernode.Version)
				require.Equal(t, "192.168.1.2", supernode.PrevIpAddresses[0].Address)
			},
		},
		{
			name: "attempt duplicate registration for same validator",
			msg: &types.MsgRegisterSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: valAddrStr,
				SupernodeAccount: walletAddr.String(),
				IpAddress:        "192.168.1.1",
				Version:          "1.0.0",
			},
			createTwice: true,
			verify: func(t *testing.T, supernode types.SuperNode) {
				require.Fail(t, "second registration should have failed")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create fresh suite for each test case
			testSuite := setupSupernodeSystemSuite(t)

			// Create and set up validator for this test case
			validator, err := stakingtypes.NewValidator(valAddrStr, walletPrivKey.PubKey(), stakingtypes.Description{})
			require.NoError(t, err)
			validator.Status = stakingtypes.Bonded
			validator.Tokens = sdkmath.NewInt(1000000)
			testSuite.app.StakingKeeper.SetValidator(testSuite.sdkCtx, validator)

			if tc.setup != nil {
				tc.setup()
			}

			msgServer := keeper.NewMsgServerImpl(testSuite.app.SupernodeKeeper)

			// For duplicate test, first create a supernode
			if tc.createTwice {
				response, err := msgServer.RegisterSupernode(testSuite.ctx, tc.msg)
				require.NoError(t, err)
				require.NotNil(t, response)
			}

			// Try to register supernode
			response, err := msgServer.RegisterSupernode(testSuite.ctx, tc.msg)

			if tc.createTwice {
				require.Error(t, err)
				require.Contains(t, err.Error(), "supernode already exists for validator")
				return
			}

			require.NoError(t, err)
			require.NotNil(t, response)

			// Verify supernode was registered
			supernode, found := testSuite.app.SupernodeKeeper.QuerySuperNode(testSuite.sdkCtx, valAddr)
			require.True(t, found)

			// Run custom verifications
			tc.verify(t, supernode)

			// Verify event emission
			events := testSuite.sdkCtx.EventManager().Events()
			var foundEvent bool
			for _, event := range events {
				if event.Type == types.EventTypeSupernodeRegistered {
					foundEvent = true
					for _, attr := range event.Attributes {
						switch string(attr.Key) {
						case types.AttributeKeyValidatorAddress:
							require.Equal(t, tc.msg.ValidatorAddress, string(attr.Value))
						case types.AttributeKeyIPAddress:
							require.Equal(t, tc.msg.IpAddress, string(attr.Value))
						case types.AttributeKeyVersion:
							require.Equal(t, tc.msg.Version, string(attr.Value))
						}
					}
				}
			}
			require.True(t, foundEvent, "supernode_registered event not found")
		})
	}
}
