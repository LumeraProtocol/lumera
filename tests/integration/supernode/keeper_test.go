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
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LumeraProtocol/lumera/app"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

type KeeperIntegrationSuite struct {
	suite.Suite

	app       *app.App
	ctx       sdk.Context
	keeper    keeper.Keeper
	queryServer sntypes.QueryServer
	authority sdk.AccAddress
	validator sdk.ValAddress
}

// SetupSuite initializes the integration test suite
func (suite *KeeperIntegrationSuite) SetupSuite() {
	os.Setenv("SYSTEM_TESTS", "true")

	suite.app = app.Setup(suite.T())
	suite.ctx = suite.app.BaseApp.NewContext(true)

	suite.authority = authtypes.NewModuleAddress(govtypes.ModuleName)
	storeService := runtime.NewKVStoreService(suite.app.GetKey(sntypes.StoreKey))

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
	suite.queryServer = keeper.NewQueryServerImpl(k)
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
				supernode := sntypes.SuperNode{
					ValidatorAddress: sdk.ValAddress([]byte("validator1e")).String(),
					SupernodeAccount: sdk.AccAddress([]byte("validator1e")).String(),
					Version:          "1.0.0",
					States:           []*sntypes.SuperNodeStateRecord{{State: sntypes.SuperNodeStateActive}},
					PrevIpAddresses:  []*sntypes.IPAddressHistory{{Address: "192.168.1.1"}},
					P2PPort:          "26657",
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
				if result.States[len(result.States)-1].State != sntypes.SuperNodeStateActive {
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
				supernode := sntypes.SuperNode{
					ValidatorAddress: sdk.ValAddress([]byte("validator1a")).String(),
					SupernodeAccount: sdk.AccAddress([]byte("validator1a")).String(),
					Version:          "1.0.0",
					States:           []*sntypes.SuperNodeStateRecord{{State: sntypes.SuperNodeStateActive}},
					PrevIpAddresses:  []*sntypes.IPAddressHistory{{Address: "192.168.1.1"}},
					P2PPort:          "26657",
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
					States:           []*sntypes.SuperNodeStateRecord{{State: sntypes.SuperNodeStateActive}},
					PrevIpAddresses:  []*sntypes.IPAddressHistory{{Address: "192.168.1.1"}},
					P2PPort:          "26657",
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
				if result.States[len(result.States)-1].State != sntypes.SuperNodeStateDisabled {
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
    minimumStakePrice := sdk.NewInt64Coin("stake", 1_000_000)
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
				params := sntypes.Params{
					MinimumStakeForSn: minimumStakePrice,
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
				params := sntypes.Params{
					MinimumStakeForSn: minimumStakePrice,
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
	supernode := sntypes.SuperNode{
		ValidatorAddress: sdk.ValAddress([]byte("validator1")).String(),
		SupernodeAccount: sdk.AccAddress([]byte("validator1")).String(),
		Version:          "1.0.0",
		States:           []*sntypes.SuperNodeStateRecord{{State: sntypes.SuperNodeStateActive}},
		PrevIpAddresses:  []*sntypes.IPAddressHistory{{Address: "192.168.1.1"}},
		P2PPort:          "26657",
	}

	require.NoError(suite.T(), suite.keeper.SetSuperNode(suite.ctx, supernode))

	result, found := suite.keeper.QuerySuperNode(suite.ctx, sdk.ValAddress("validator1"))
	require.True(suite.T(), found, "SuperNode should exist after being set")
	require.Equal(suite.T(), sntypes.SuperNodeStateActive, result.States[len(result.States)-1].State, "SuperNode state should match")
}

func (suite *KeeperIntegrationSuite) TestGetSuperNodeBySuperNodeAddress() {
	tests := []struct {
		name          string
		setup         func()
		execute       func() (*sntypes.QueryGetSuperNodeBySuperNodeAddressResponse, error)
		validate      func(response *sntypes.QueryGetSuperNodeBySuperNodeAddressResponse) error
		expectSuccess bool
	}{
		{
			name: "when supernode is found by address, it should return the supernode",
			setup: func() {
				suite.validator = sdk.ValAddress([]byte("validator1f"))
				suite.authority = sdk.AccAddress(suite.validator)
				supernode := sntypes.SuperNode{
					SupernodeAccount: suite.authority.String(),
					ValidatorAddress: suite.validator.String(),
					Version:          "1.0.0",
					States:           []*sntypes.SuperNodeStateRecord{{State: sntypes.SuperNodeStateActive}},
					PrevIpAddresses:  []*sntypes.IPAddressHistory{{Address: "192.168.1.1"}},
					P2PPort:          "26657",
				}
				require.NoError(suite.T(), suite.keeper.SetSuperNode(suite.ctx, supernode))
			},
			execute: func() (*types.QueryGetSuperNodeBySuperNodeAddressResponse, error) {
				req := &types.QueryGetSuperNodeBySuperNodeAddressRequest{
					SupernodeAddress: suite.authority.String(),
				}
				// Call the query method to get the supernode by address
				return suite.queryServer.GetSuperNodeBySuperNodeAddress(suite.ctx, req)
			},
			validate: func(response *sntypes.QueryGetSuperNodeBySuperNodeAddressResponse) error {
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
			execute: func() (*sntypes.QueryGetSuperNodeBySuperNodeAddressResponse, error) {
				req := &sntypes.QueryGetSuperNodeBySuperNodeAddressRequest{
					SupernodeAddress: "nonexistent-supernode",
				}
				return suite.queryServer.GetSuperNodeBySuperNodeAddress(suite.ctx, req)
			},
			validate: func(response *sntypes.QueryGetSuperNodeBySuperNodeAddressResponse) error {
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

func TestKeeperIntegrationSuite(t *testing.T) {
	suite.Run(t, new(KeeperIntegrationSuite))
}
