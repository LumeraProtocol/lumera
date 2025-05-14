package system_test

import (
	"context"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	types2 "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	"os"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/LumeraProtocol/lumera/app"
	"github.com/LumeraProtocol/lumera/tests/ibctesting"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

type SystemTestSuite struct {
	app    *app.App
	sdkCtx sdk.Context
	ctx    context.Context
}

func setupSupernodeSystemSuite(t *testing.T) *SystemTestSuite {
	os.Setenv("SYSTEM_TESTS", "true")
	t.Cleanup(func() {
		os.Unsetenv("SYSTEM_TESTS")
	})

	suite := &SystemTestSuite{}
	coord := ibctesting.NewCoordinator(t, 1) // One chain setup
	chain := coord.GetChain(ibctesting.GetChainID(1))

	app := chain.App.(*app.App)
	suite.app = app

	baseCtx := chain.GetContext()
	suite.sdkCtx = baseCtx
	suite.ctx = sdk.WrapSDKContext(baseCtx)

	// Set up validator for testing
	valPrivKey := secp256k1.GenPrivKey()
	valPubKey := valPrivKey.PubKey()
	valAddr := sdk.ValAddress(valPubKey.Address().Bytes())

	// Create and bond the validator
	validator, err := stakingtypes.NewValidator(valAddr.String(), valPubKey, stakingtypes.Description{})
	if err != nil {
		t.Fatalf("failed to create validator: %s", err)
	}
	validator.Status = stakingtypes.Bonded
	validator.Tokens = sdkmath.NewInt(1000000)
	suite.app.StakingKeeper.SetValidator(suite.sdkCtx, validator)

	// Store validator address in context for test cases
	suite.ctx = context.WithValue(suite.ctx, "validator_address", valAddr.Bytes())

	// Set up default parameters
	err = suite.app.SupernodeKeeper.SetParams(chain.GetContext(), types2.DefaultParams())
	require.NoError(t, err)

	return suite
}

func TestDeregisterSupernode(t *testing.T) {
	//
	// Create a base wallet/validator address to use in each sub-test.
	//
	walletPrivKey := secp256k1.GenPrivKey()
	walletAddr := sdk.AccAddress(walletPrivKey.PubKey().Address())
	valAddr := sdk.ValAddress(walletAddr)
	valAddrStr := valAddr.String()

	// An unauthorized address for negative tests
	unauthorizedPrivKey := secp256k1.GenPrivKey()
	unauthorizedAddr := sdk.AccAddress(unauthorizedPrivKey.PubKey().Address())

	testCases := []struct {
		name   string
		msg    *types2.MsgDeregisterSupernode
		setup  func(suite *SystemTestSuite)
		verify func(t *testing.T, suite *SystemTestSuite, response *types2.MsgDeregisterSupernodeResponse, err error)
	}{
		{
			name: "Successful deregistration",
			msg: &types2.MsgDeregisterSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: valAddrStr,
			},
			setup: func(suite *SystemTestSuite) {
				// Register a supernode first
				supernode := types2.SuperNode{
					ValidatorAddress: valAddrStr,
					SupernodeAccount: walletAddr.String(),
					States: []*types2.SuperNodeStateRecord{
						{
							State:  types2.SuperNodeStateActive,
							Height: suite.sdkCtx.BlockHeight(),
						},
					},
					Version: "1.0.0",
					Metrics: &types2.MetricsAggregate{
						Metrics:     make(map[string]float64),
						ReportCount: 0,
					},
					Evidence: []*types2.Evidence{},
					PrevIpAddresses: []*types2.IPAddressHistory{
						{
							Address: "127.0.0.1",
							Height:  suite.sdkCtx.BlockHeight(),
						},
					},
					P2PPort: "26657",
				}
				err := suite.app.SupernodeKeeper.SetSuperNode(suite.sdkCtx, supernode)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, suite *SystemTestSuite, response *types2.MsgDeregisterSupernodeResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, response)

				// Verify supernode state has been updated
				val, valErr := sdk.ValAddressFromBech32(valAddrStr)
				require.NoError(t, valErr)
				supernode, found := suite.app.SupernodeKeeper.QuerySuperNode(suite.sdkCtx, val)
				require.True(t, found)
				require.NotEmpty(t, supernode.States)
				require.Equal(t, types2.SuperNodeStateDisabled, supernode.States[len(supernode.States)-1].State)

				// Verify event emission
				events := suite.sdkCtx.EventManager().Events()
				require.NotEmpty(t, events)

				var foundDeregisterEvent bool
				for _, evt := range events {
					if evt.Type == types2.EventTypeSupernodeDeRegistered {
						foundDeregisterEvent = true
						for _, attr := range evt.Attributes {
							if string(attr.Key) == types2.AttributeKeyValidatorAddress {
								require.Equal(t, valAddrStr, string(attr.Value))
							}
						}
					}
				}
				require.True(t, foundDeregisterEvent, "supernode_deregistered event not found")
			},
		},
		{
			name: "Supernode not found",
			msg: &types2.MsgDeregisterSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: valAddrStr,
			},
			setup: func(suite *SystemTestSuite) {
				// Don't set up any supernode
			},
			verify: func(t *testing.T, suite *SystemTestSuite, response *types2.MsgDeregisterSupernodeResponse, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, sdkerrors.ErrNotFound)
				require.Nil(t, response)
			},
		},
		{
			name: "Unauthorized deregistration attempt",
			msg: &types2.MsgDeregisterSupernode{
				Creator:          unauthorizedAddr.String(),
				ValidatorAddress: valAddrStr,
			},
			setup: func(suite *SystemTestSuite) {
				// Register a supernode with the authorized (walletAddr) account
				supernode := types2.SuperNode{
					ValidatorAddress: valAddrStr,
					SupernodeAccount: walletAddr.String(),
					States: []*types2.SuperNodeStateRecord{
						{
							State:  types2.SuperNodeStateActive,
							Height: suite.sdkCtx.BlockHeight(),
						},
					},
					Version: "1.0.0",
					Metrics: &types2.MetricsAggregate{
						Metrics:     make(map[string]float64),
						ReportCount: 0,
					},
					Evidence: []*types2.Evidence{},
					PrevIpAddresses: []*types2.IPAddressHistory{
						{
							Address: "127.0.0.1",
							Height:  suite.sdkCtx.BlockHeight(),
						},
					},
					P2PPort: "26657",
				}
				err := suite.app.SupernodeKeeper.SetSuperNode(suite.sdkCtx, supernode)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, suite *SystemTestSuite, response *types2.MsgDeregisterSupernodeResponse, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
				require.Nil(t, response)
			},
		},
		{
			name: "Already disabled supernode",
			msg: &types2.MsgDeregisterSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: valAddrStr,
			},
			setup: func(suite *SystemTestSuite) {
				// Create a disabled supernode
				supernode := types2.SuperNode{
					ValidatorAddress: valAddrStr,
					SupernodeAccount: walletAddr.String(),
					States: []*types2.SuperNodeStateRecord{
						{
							State:  types2.SuperNodeStateDisabled,
							Height: suite.sdkCtx.BlockHeight(),
						},
					},
					Version: "1.0.0",
					Metrics: &types2.MetricsAggregate{
						Metrics:     make(map[string]float64),
						ReportCount: 0,
					},
					Evidence: []*types2.Evidence{},
					PrevIpAddresses: []*types2.IPAddressHistory{
						{
							Address: "127.0.0.1",
							Height:  suite.sdkCtx.BlockHeight(),
						},
					},
					P2PPort: "26657",
				}
				err := suite.app.SupernodeKeeper.SetSuperNode(suite.sdkCtx, supernode)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, suite *SystemTestSuite, response *types2.MsgDeregisterSupernodeResponse, err error) {
				// Deregistration on an already disabled supernode is allowed (no error).
				require.NoError(t, err)
				require.NotNil(t, response)

				val, valErr := sdk.ValAddressFromBech32(valAddrStr)
				require.NoError(t, valErr)
				supernode, found := suite.app.SupernodeKeeper.QuerySuperNode(suite.sdkCtx, val)
				require.True(t, found)
				require.NotEmpty(t, supernode.States)
				// Ensure it remains disabled
				require.Equal(t, types2.SuperNodeStateDisabled, supernode.States[len(supernode.States)-1].State)

				// Verify event emission
				events := suite.sdkCtx.EventManager().Events()
				var foundDeregisterEvent bool
				for _, evt := range events {
					if evt.Type == types2.EventTypeSupernodeDeRegistered {
						foundDeregisterEvent = true
						for _, attr := range evt.Attributes {
							if string(attr.Key) == types2.AttributeKeyValidatorAddress {
								require.Equal(t, valAddrStr, string(attr.Value))
							}
						}
					}
				}
				require.True(t, foundDeregisterEvent, "supernode_deregistered event not found")
			},
		},
		{
			name: "Invalid validator address",
			msg: &types2.MsgDeregisterSupernode{
				Creator:          walletAddr.String(),
				ValidatorAddress: "invalid-address",
			},
			setup: func(suite *SystemTestSuite) {
				// No setup needed
			},
			verify: func(t *testing.T, suite *SystemTestSuite, response *types2.MsgDeregisterSupernodeResponse, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
				require.Nil(t, response)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			//
			// Create a fresh test suite for each test case
			//
			testSuite := setupSupernodeSystemSuite(t)

			// Create and set up the validator
			validator, err := stakingtypes.NewValidator(valAddrStr, walletPrivKey.PubKey(), stakingtypes.Description{})
			require.NoError(t, err)
			validator.Status = stakingtypes.Bonded
			validator.Tokens = sdkmath.NewInt(1000000)
			testSuite.app.StakingKeeper.SetValidator(testSuite.sdkCtx, validator)

			// Run any test-specific setup
			if tc.setup != nil {
				tc.setup(testSuite)
			}

			// Execute the DeregisterSupernode message
			msgServer := keeper.NewMsgServerImpl(testSuite.app.SupernodeKeeper)
			response, err := msgServer.DeregisterSupernode(testSuite.ctx, tc.msg)

			// Run verification logic
			tc.verify(t, testSuite, response, err)
		})
	}
}
