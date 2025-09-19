package integration_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	types2 "github.com/LumeraProtocol/lumera/x/supernode/v1/types"

	sdkmath "cosmossdk.io/math"
	"github.com/LumeraProtocol/lumera/app"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type KeeperIntegrationSuite struct {
	suite.Suite
	app       *app.App
	ctx       sdk.Context
	keeper    keeper.Keeper
	authority sdk.AccAddress
	validator sdk.ValAddress
}

// SetupSuite initializes the integration test suite
func (suite *KeeperIntegrationSuite) SetupSuite() {
	os.Setenv("SYSTEM_TESTS", "true")

	suite.app = app.Setup(suite.T())
	suite.ctx = suite.app.BaseApp.NewContext(true)

	suite.authority = authtypes.NewModuleAddress(govtypes.ModuleName)
	storeService := runtime.NewKVStoreService(suite.app.GetKey(types2.StoreKey))

	k := keeper.NewKeeper(
		suite.app.AppCodec(),
		storeService,
		suite.app.Logger(),
		suite.authority.String(),
		suite.app.BankKeeper,
		suite.app.StakingKeeper,
		suite.app.SlashingKeeper,
	)
	suite.keeper = k
}

// TearDownSuite cleans up after the test suite
func (suite *KeeperIntegrationSuite) TearDownSuite() {
	suite.app = nil
}

func (suite *KeeperIntegrationSuite) TestSetSuperNodeActive() {
	tests := []struct {
		name          string
		setup         func()
		execute       func() error
		validate      func() error
		expectSuccess bool
	}{
		{
			name: "when supernode state is successfully enabled, it should be active",
			setup: func() {
				supernode := types2.SuperNode{
					ValidatorAddress:      sdk.ValAddress([]byte("validator1e")).String(),
					SupernodeAccount:      sdk.AccAddress([]byte("validator1e")).String(),
					Note:                  "1.0.0",
					States:                []*types2.SuperNodeStateRecord{{State: types2.SuperNodeStateActive}},
					PrevIpAddresses:       []*types2.IPAddressHistory{{Address: "192.168.1.1"}},
					PrevSupernodeAccounts: []*types2.SupernodeAccountHistory{{Account: sdk.AccAddress([]byte("validator1e")).String()}},
					P2PPort:               "26657",
				}
				err := suite.keeper.SetSuperNode(suite.ctx, supernode)
				require.NoError(suite.T(), err)
			},
			execute: func() error {
				return suite.keeper.SetSuperNodeActive(suite.ctx, sdk.ValAddress("validator1e"))
			},
			validate: func() error {
				result, found := suite.keeper.QuerySuperNode(suite.ctx, sdk.ValAddress("validator1e"))
				if !found {
					return fmt.Errorf("SuperNode not found")
				}
				if result.States[len(result.States)-1].State != types2.SuperNodeStateActive {
					return fmt.Errorf("expected SuperNode to be active")
				}
				return nil
			},
			expectSuccess: true,
		},
		{
			name: "when supernode is stopped, it should become active",
			setup: func() {
				supernode := types2.SuperNode{
					ValidatorAddress:      sdk.ValAddress([]byte("validator1f")).String(),
					SupernodeAccount:      sdk.AccAddress([]byte("validator1f")).String(),
					Note:                  "1.0.0",
					States:                []*types2.SuperNodeStateRecord{{State: types2.SuperNodeStateStopped}},
					PrevIpAddresses:       []*types2.IPAddressHistory{{Address: "192.168.1.1"}},
					PrevSupernodeAccounts: []*types2.SupernodeAccountHistory{{Account: sdk.AccAddress([]byte("validator1f")).String()}},
					P2PPort:               "26657",
				}
				err := suite.keeper.SetSuperNode(suite.ctx, supernode)
				require.NoError(suite.T(), err)
			},
			execute: func() error {
				return suite.keeper.SetSuperNodeActive(suite.ctx, sdk.ValAddress("validator1f"))
			},
			validate: func() error {
				result, found := suite.keeper.QuerySuperNode(suite.ctx, sdk.ValAddress("validator1f"))
				if !found {
					return fmt.Errorf("SuperNode not found")
				}
				if len(result.States) != 2 {
					return fmt.Errorf("expected 2 state records, got %d", len(result.States))
				}
				if result.States[len(result.States)-1].State != types2.SuperNodeStateActive {
					return fmt.Errorf("expected SuperNode to be active")
				}
				return nil
			},
			expectSuccess: true,
		},
		{
			name: "when supernode is disabled, it should not become active",
			setup: func() {
				supernode := types2.SuperNode{
					ValidatorAddress:      sdk.ValAddress([]byte("validator1g")).String(),
					SupernodeAccount:      sdk.AccAddress([]byte("validator1g")).String(),
					Note:                  "1.0.0",
					States:                []*types2.SuperNodeStateRecord{{State: types2.SuperNodeStateDisabled}},
					PrevIpAddresses:       []*types2.IPAddressHistory{{Address: "192.168.1.1"}},
					PrevSupernodeAccounts: []*types2.SupernodeAccountHistory{{Account: sdk.AccAddress([]byte("validator1g")).String()}},
					P2PPort:               "26657",
				}
				err := suite.keeper.SetSuperNode(suite.ctx, supernode)
				require.NoError(suite.T(), err)
			},
			execute: func() error {
				return suite.keeper.SetSuperNodeActive(suite.ctx, sdk.ValAddress("validator1g"))
			},
			validate: func() error {
				result, found := suite.keeper.QuerySuperNode(suite.ctx, sdk.ValAddress("validator1g"))
				if !found {
					return fmt.Errorf("SuperNode not found")
				}
				// Should still have only 1 state record (disabled state unchanged)
				if len(result.States) != 1 {
					return fmt.Errorf("expected 1 state record, got %d", len(result.States))
				}
				if result.States[0].State != types2.SuperNodeStateDisabled {
					return fmt.Errorf("expected SuperNode to remain disabled")
				}
				return nil
			},
			expectSuccess: true, // SetSuperNodeActive silently ignores disabled supernodes
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			tc.setup()
			err := tc.execute()
			if tc.expectSuccess {
				require.NoError(suite.T(), err)
				if tc.validate != nil {
					require.NoError(suite.T(), tc.validate())
				}
			} else {
				require.Error(suite.T(), err)
			}
		})
	}
}

func (suite *KeeperIntegrationSuite) TestIsSupernodeActive() {
	tests := []struct {
		name          string
		setup         func()
		execute       func() error
		validate      func() error
		expectSuccess bool
	}{
		{
			name: "when supernode is in active state, should return true",
			setup: func() {
				supernode := types2.SuperNode{
					ValidatorAddress:      sdk.ValAddress([]byte("validator1a")).String(),
					SupernodeAccount:      sdk.AccAddress([]byte("validator1a")).String(),
					Note:                  "1.0.0",
					States:                []*types2.SuperNodeStateRecord{{State: types2.SuperNodeStateActive}},
					PrevIpAddresses:       []*types2.IPAddressHistory{{Address: "192.168.1.1"}},
					PrevSupernodeAccounts: []*types2.SupernodeAccountHistory{{Account: sdk.AccAddress([]byte("validator1a")).String()}},
					P2PPort:               "26657",
				}
				suite.keeper.SetSuperNode(suite.ctx, supernode)
			},
			execute: func() error {
				active := suite.keeper.IsSuperNodeActive(suite.ctx, sdk.ValAddress("validator1a"))
				if !active {
					return fmt.Errorf("expected SuperNode to be active")
				}
				return nil
			},
			validate:      func() error { return nil },
			expectSuccess: true,
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			tc.setup()
			err := tc.execute()
			if tc.expectSuccess {
				require.NoError(suite.T(), err)
				if tc.validate != nil {
					require.NoError(suite.T(), tc.validate())
				}
			} else {
				require.Error(suite.T(), err)
			}
		})
	}
}

func (suite *KeeperIntegrationSuite) TestSetSuperNodeStopped() {
	tests := []struct {
		name          string
		setup         func()
		execute       func() error
		validate      func() error
		expectSuccess bool
	}{
		{
			name: "when supernode is successfully stopped, it should be stopped",
			setup: func() {
				supernode := types2.SuperNode{
					ValidatorAddress:      sdk.ValAddress([]byte("validator1d")).String(),
					SupernodeAccount:      sdk.AccAddress([]byte("validator1d")).String(),
					Note:                  "1.0.0",
					States:                []*types2.SuperNodeStateRecord{{State: types2.SuperNodeStateActive}},
					PrevIpAddresses:       []*types2.IPAddressHistory{{Address: "192.168.1.1"}},
					PrevSupernodeAccounts: []*types2.SupernodeAccountHistory{{Account: sdk.AccAddress([]byte("validator1d")).String()}},
					P2PPort:               "26657",
				}
				suite.keeper.SetSuperNode(suite.ctx, supernode)
			},
			execute: func() error {
				return suite.keeper.SetSuperNodeStopped(suite.ctx, sdk.ValAddress("validator1d"))
			},
			validate: func() error {
				result, found := suite.keeper.QuerySuperNode(suite.ctx, sdk.ValAddress("validator1d"))
				if !found {
					return fmt.Errorf("SuperNode not found")
				}
				if result.States[len(result.States)-1].State != types2.SuperNodeStateStopped {
					return fmt.Errorf("expected SuperNode to be stopped")
				}
				return nil
			},
			expectSuccess: true,
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			tc.setup()
			err := tc.execute()
			if tc.expectSuccess {
				require.NoError(suite.T(), err)
				if tc.validate != nil {
					require.NoError(suite.T(), tc.validate())
				}
			} else {
				require.Error(suite.T(), err)
			}
		})
	}
}

func (suite *KeeperIntegrationSuite) TestMeetSupernodeRequirements() {
	tests := []struct {
		name          string
		setup         func()
		execute       func() error
		validate      func() error
		expectSuccess bool
	}{
		{
			name: "when supernode meets requirements, it should return true",
			setup: func() {
				params := types2.Params{
					MinimumStakeForSn: sdk.NewCoin("stake", sdkmath.NewInt(1000000)),
				}
				suite.keeper.SetParams(suite.ctx, params)

				validator := stakingtypes.Validator{
					OperatorAddress: sdk.ValAddress("validator1").String(),
					Tokens:          sdkmath.NewInt(2000000),
					DelegatorShares: sdkmath.LegacyNewDec(2000000), // Ensure shares match self-delegation
				}
				suite.app.StakingKeeper.SetValidator(suite.ctx, validator)

				// Set self-delegation
				selfDelegation := stakingtypes.Delegation{
					DelegatorAddress: sdk.AccAddress(("validator1")).String(),
					ValidatorAddress: sdk.ValAddress("validator1").String(),
					Shares:           sdkmath.LegacyNewDec(2000000), // Self-delegation should match shares
				}
				suite.app.StakingKeeper.SetDelegation(suite.ctx, selfDelegation)
			},
			execute: func() error {
				meets := suite.keeper.IsEligibleAndNotJailedValidator(suite.ctx, sdk.ValAddress("validator1"))
				if !meets {
					return fmt.Errorf("expected validator to meet SuperNode requirements")
				}
				return nil
			},
			validate:      func() error { return nil },
			expectSuccess: true,
		},
		{
			name: "when the stake is below minimum, should return false",
			setup: func() {
				params := types2.Params{
					MinimumStakeForSn: sdk.NewCoin("stake", sdkmath.NewInt(1000000)),
				}
				suite.keeper.SetParams(suite.ctx, params)

				validator := stakingtypes.Validator{
					OperatorAddress: sdk.ValAddress("validator1").String(),
					Tokens:          sdkmath.NewInt(500000),
				}
				suite.app.StakingKeeper.SetValidator(suite.ctx, validator)
			},
			execute: func() error {
				meets := suite.keeper.IsEligibleAndNotJailedValidator(suite.ctx, sdk.ValAddress("validator1"))
				if meets {
					return fmt.Errorf("expected validator not to meet SuperNode requirements")
				}
				return nil
			},
			validate:      func() error { return nil },
			expectSuccess: true,
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			tc.setup()
			err := tc.execute()
			if tc.expectSuccess {
				require.NoError(suite.T(), err)
				if tc.validate != nil {
					require.NoError(suite.T(), tc.validate())
				}
			} else {
				require.Error(suite.T(), err)
			}
		})
	}
}

func (suite *KeeperIntegrationSuite) TestSetSuperNodeAndQuerySupernode() {
	supernode := types2.SuperNode{
		ValidatorAddress: sdk.ValAddress([]byte("validator1")).String(),
		SupernodeAccount: sdk.AccAddress([]byte("validator1")).String(),
		Note:             "1.0.0",
		States:           []*types2.SuperNodeStateRecord{{State: types2.SuperNodeStateActive}},
		PrevIpAddresses:  []*types2.IPAddressHistory{{Address: "192.168.1.1"}},
		P2PPort:          "26657",
	}

	require.NoError(suite.T(), suite.keeper.SetSuperNode(suite.ctx, supernode))

	result, found := suite.keeper.QuerySuperNode(suite.ctx, sdk.ValAddress("validator1"))
	require.True(suite.T(), found, "SuperNode should exist after being set")
	require.Equal(suite.T(), types2.SuperNodeStateActive, result.States[len(result.States)-1].State, "SuperNode state should match")
}

func (suite *KeeperIntegrationSuite) TestGetSuperNodeBySuperNodeAddress() {
	tests := []struct {
		name          string
		setup         func()
		execute       func() (*types2.QueryGetSuperNodeBySuperNodeAddressResponse, error)
		validate      func(response *types2.QueryGetSuperNodeBySuperNodeAddressResponse) error
		expectSuccess bool
	}{
		{
			name: "when supernode is found by address, it should return the supernode",
			setup: func() {
				suite.validator = sdk.ValAddress([]byte("validator1f"))
				suite.authority = sdk.AccAddress(suite.validator)
				supernode := types2.SuperNode{
					SupernodeAccount:      suite.authority.String(),
					ValidatorAddress:      suite.validator.String(),
					Note:                  "1.0.0",
					States:                []*types2.SuperNodeStateRecord{{State: types2.SuperNodeStateActive}},
					PrevIpAddresses:       []*types2.IPAddressHistory{{Address: "192.168.1.1"}},
					PrevSupernodeAccounts: []*types2.SupernodeAccountHistory{{Account: sdk.AccAddress([]byte("validator1")).String()}},
					P2PPort:               "26657",
				}
				require.NoError(suite.T(), suite.keeper.SetSuperNode(suite.ctx, supernode))
			},
			execute: func() (*types2.QueryGetSuperNodeBySuperNodeAddressResponse, error) {
				req := &types2.QueryGetSuperNodeBySuperNodeAddressRequest{
					SupernodeAddress: suite.authority.String(),
				}
				return suite.keeper.GetSuperNodeBySuperNodeAddress(suite.ctx, req)
			},
			validate: func(response *types2.QueryGetSuperNodeBySuperNodeAddressResponse) error {
				if response.Supernode == nil {
					return fmt.Errorf("supernode should not be nil")
				}
				if response.Supernode.SupernodeAccount != suite.authority.String() {
					return fmt.Errorf("expected supernode account '%v', got: %v", suite.authority.String(), response.Supernode.SupernodeAccount)
				}
				return nil
			},
			expectSuccess: true,
		},
		{
			name: "when supernode is not found by address, it should return an error",
			setup: func() {
				// No setup needed, as no supernode will be added for this test.
			},
			execute: func() (*types2.QueryGetSuperNodeBySuperNodeAddressResponse, error) {
				req := &types2.QueryGetSuperNodeBySuperNodeAddressRequest{
					SupernodeAddress: "nonexistent-supernode",
				}
				return suite.keeper.GetSuperNodeBySuperNodeAddress(suite.ctx, req)
			},
			validate: func(response *types2.QueryGetSuperNodeBySuperNodeAddressResponse) error {
				if response != nil {
					return fmt.Errorf("expected nil response, got: %v", response)
				}
				return nil
			},
			expectSuccess: false,
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			tc.setup()
			resp, err := tc.execute()
			if tc.expectSuccess {
				require.NoError(suite.T(), err)
				if tc.validate != nil {
					require.NoError(suite.T(), tc.validate(resp))
				}
			} else {
				require.Error(suite.T(), err)
			}
		})
	}
}

func (suite *KeeperIntegrationSuite) TestSupernodeReRegistration() {
	// Test re-registration of a disabled supernode
	pk := secp256k1.GenPrivKey()
	accAddr := sdk.AccAddress(pk.PubKey().Address())
	valAddr := sdk.ValAddress(pk.PubKey().Address())

	// Step 1: Create and register a supernode
	supernode := types2.SuperNode{
		ValidatorAddress:      valAddr.String(),
		SupernodeAccount:      accAddr.String(),
		Note:                  "1.0.0",
		States:                []*types2.SuperNodeStateRecord{{State: types2.SuperNodeStateActive, Height: 100}},
		PrevIpAddresses:       []*types2.IPAddressHistory{{Address: "192.168.1.1", Height: 100}},
		PrevSupernodeAccounts: []*types2.SupernodeAccountHistory{{Account: accAddr.String(), Height: 100}},
		P2PPort:               "26657",
	}
	err := suite.keeper.SetSuperNode(suite.ctx, supernode)
	require.NoError(suite.T(), err)

	// Step 2: Disable the supernode (deregister)
	supernode.States = append(supernode.States, &types2.SuperNodeStateRecord{
		State:  types2.SuperNodeStateDisabled,
		Height: 200,
	})
	err = suite.keeper.SetSuperNode(suite.ctx, supernode)
	require.NoError(suite.T(), err)

	// Step 3: Simulate re-registration (only state should change)
	// Create validator for eligibility check
	validator := stakingtypes.Validator{
		OperatorAddress: valAddr.String(),
		Tokens:          sdkmath.NewInt(2000000),
		Status:          stakingtypes.Bonded,
	}
	suite.app.StakingKeeper.SetValidator(suite.ctx, validator)

	// Step 4: Re-registration changes state to active but doesn't update other fields
	supernode.States = append(supernode.States, &types2.SuperNodeStateRecord{
		State:  types2.SuperNodeStateActive,
		Height: 300,
	})
	err = suite.keeper.SetSuperNode(suite.ctx, supernode)
	require.NoError(suite.T(), err)

	// Verify the final state
	result, found := suite.keeper.QuerySuperNode(suite.ctx, valAddr)
	require.True(suite.T(), found)
	require.Len(suite.T(), result.States, 3) // Active -> Disabled -> Active
	require.Equal(suite.T(), types2.SuperNodeStateActive, result.States[2].State)
	require.Equal(suite.T(), types2.SuperNodeStateDisabled, result.States[1].State)

	// Verify other fields remain unchanged
	require.Len(suite.T(), result.PrevIpAddresses, 1) // No new IP history
	require.Equal(suite.T(), "192.168.1.1", result.PrevIpAddresses[0].Address)
	require.Len(suite.T(), result.PrevSupernodeAccounts, 1) // No new account history
	require.Equal(suite.T(), accAddr.String(), result.PrevSupernodeAccounts[0].Account)
}

func TestKeeperIntegrationSuite(t *testing.T) {
	suite.Run(t, new(KeeperIntegrationSuite))
}
