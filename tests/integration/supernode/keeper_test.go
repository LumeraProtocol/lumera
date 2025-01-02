package integration_test

import (
	"fmt"
	"os"
	"testing"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/pastelnetwork/pastel/app"
	"github.com/pastelnetwork/pastel/x/supernode/keeper"
	"github.com/pastelnetwork/pastel/x/supernode/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type KeeperIntegrationSuite struct {
	suite.Suite
	app       *app.App
	ctx       sdk.Context
	keeper    keeper.Keeper
	authority sdk.AccAddress
}

// SetupSuite initializes the integration test suite
func (suite *KeeperIntegrationSuite) SetupSuite() {
	os.Setenv("SYSTEM_TESTS", "true")

	suite.app = app.Setup(suite.T())
	suite.ctx = suite.app.BaseApp.NewContext(true)

	suite.authority = authtypes.NewModuleAddress(govtypes.ModuleName)
	storeService := runtime.NewKVStoreService(suite.app.GetKey(types.StoreKey))

	k := keeper.NewKeeper(
		suite.app.AppCodec(),
		storeService,
		suite.app.Logger(),
		suite.authority.String(),
		suite.app.BankKeeper,
		suite.app.StakingKeeper,
		suite.app.SlashingKeeper,
	)
	hooks := keeper.NewSupernodeHooks(k)
	k.AddStakingHooks(types.NewStakingHooksWrapper(hooks))

	suite.keeper = k
}

// TearDownSuite cleans up after the test suite
func (suite *KeeperIntegrationSuite) TearDownSuite() {
	suite.app = nil
}

func (suite *KeeperIntegrationSuite) TestEnableSuperNode() {
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
				supernode := types.SuperNode{
					ValidatorAddress: sdk.ValAddress([]byte("validator1e")).String(),
					SupernodeAccount: sdk.AccAddress([]byte("validator1e")).String(),
					Version:          "1.0.0",
					States:           []*types.SuperNodeStateRecord{{State: types.SuperNodeStateActive}},
					PrevIpAddresses:  []*types.IPAddressHistory{{Address: "192.168.1.1"}},
				}
				err := suite.keeper.SetSuperNode(suite.ctx, supernode)
				require.NoError(suite.T(), err)
			},
			execute: func() error {
				return suite.keeper.EnableSuperNode(suite.ctx, sdk.ValAddress("validator1e"))
			},
			validate: func() error {
				result, found := suite.keeper.QuerySuperNode(suite.ctx, sdk.ValAddress("validator1e"))
				if !found {
					return fmt.Errorf("SuperNode not found")
				}
				if result.States[len(result.States)-1].State != types.SuperNodeStateActive {
					return fmt.Errorf("expected SuperNode to be active")
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
				supernode := types.SuperNode{
					ValidatorAddress: sdk.ValAddress([]byte("validator1a")).String(),
					SupernodeAccount: sdk.AccAddress([]byte("validator1a")).String(),
					Version:          "1.0.0",
					States:           []*types.SuperNodeStateRecord{{State: types.SuperNodeStateActive}},
					PrevIpAddresses:  []*types.IPAddressHistory{{Address: "192.168.1.1"}},
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

func (suite *KeeperIntegrationSuite) TestDisableSuperNode() {
	tests := []struct {
		name          string
		setup         func()
		execute       func() error
		validate      func() error
		expectSuccess bool
	}{
		{
			name: "when supernode is successfully disabled, it should be disabled",
			setup: func() {
				supernode := types.SuperNode{
					ValidatorAddress: sdk.ValAddress([]byte("validator1d")).String(),
					SupernodeAccount: sdk.AccAddress([]byte("validator1d")).String(),
					Version:          "1.0.0",
					States:           []*types.SuperNodeStateRecord{{State: types.SuperNodeStateActive}},
					PrevIpAddresses:  []*types.IPAddressHistory{{Address: "192.168.1.1"}},
				}
				suite.keeper.SetSuperNode(suite.ctx, supernode)
			},
			execute: func() error {
				return suite.keeper.DisableSuperNode(suite.ctx, sdk.ValAddress("validator1d"))
			},
			validate: func() error {
				result, found := suite.keeper.QuerySuperNode(suite.ctx, sdk.ValAddress("validator1d"))
				if !found {
					return fmt.Errorf("SuperNode not found")
				}
				if result.States[len(result.States)-1].State != types.SuperNodeStateDisabled {
					return fmt.Errorf("expected SuperNode to be disabled")
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
				params := types.Params{
					MinimumStakeForSn: 1000000,
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
				meets := suite.keeper.MeetsSuperNodeRequirements(suite.ctx, sdk.ValAddress("validator1"))
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
				params := types.Params{
					MinimumStakeForSn: 1000000,
				}
				suite.keeper.SetParams(suite.ctx, params)

				validator := stakingtypes.Validator{
					OperatorAddress: sdk.ValAddress("validator1").String(),
					Tokens:          sdkmath.NewInt(500000),
				}
				suite.app.StakingKeeper.SetValidator(suite.ctx, validator)
			},
			execute: func() error {
				meets := suite.keeper.MeetsSuperNodeRequirements(suite.ctx, sdk.ValAddress("validator1"))
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
	supernode := types.SuperNode{
		ValidatorAddress: sdk.ValAddress([]byte("validator1")).String(),
		SupernodeAccount: sdk.AccAddress([]byte("validator1")).String(),
		Version:          "1.0.0",
		States:           []*types.SuperNodeStateRecord{{State: types.SuperNodeStateActive}},
		PrevIpAddresses:  []*types.IPAddressHistory{{Address: "192.168.1.1"}},
	}

	require.NoError(suite.T(), suite.keeper.SetSuperNode(suite.ctx, supernode))

	result, found := suite.keeper.QuerySuperNode(suite.ctx, sdk.ValAddress("validator1"))
	require.True(suite.T(), found, "SuperNode should exist after being set")
	require.Equal(suite.T(), types.SuperNodeStateActive, result.States[len(result.States)-1].State, "SuperNode state should match")
}

func TestKeeperIntegrationSuite(t *testing.T) {
	suite.Run(t, new(KeeperIntegrationSuite))
}
