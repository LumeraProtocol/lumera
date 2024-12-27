package integration_test

import (
	sdkmath "cosmossdk.io/math"
	"fmt"

	_ "cosmossdk.io/log"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/pastelnetwork/pastel/x/supernode/types"
	"github.com/stretchr/testify/require"
)

func (suite *KeeperIntegrationSuite) TestAfterValidatorCreatedHook() {
	tests := []struct {
		name          string
		setup         func()
		execute       func() error
		validate      func() error
		expectSuccess bool
	}{
		{
			name: "when validator is created and meet supernode requirements, it should be active",
			setup: func() {
				params := types.Params{
					MinimumStakeForSn: 1000000,
				}
				suite.keeper.SetParams(suite.ctx, params)

				supernode := types.SuperNode{
					ValidatorAddress: sdk.ValAddress([]byte("validator1")).String(),
					States:           []*types.SuperNodeStateRecord{{State: types.SuperNodeStateDisabled}},
				}
				suite.keeper.SetSuperNode(suite.ctx, supernode)

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
				return suite.keeper.GetHooks().Hooks().AfterValidatorCreated(suite.ctx, sdk.ValAddress("validator1"))
			},
			validate: func() error {
				result, found := suite.keeper.QuerySuperNode(suite.ctx, sdk.ValAddress("validator1"))
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

func (suite *KeeperIntegrationSuite) TestValidatorBeginUnbondingHook() {
	tests := []struct {
		name          string
		setup         func()
		execute       func() error
		validate      func() error
		expectSuccess bool
	}{

		{
			name: "when the validator begins un-bonding and is active, it should be disabled",
			setup: func() {
				params := types.Params{
					MinimumStakeForSn: 1000000,
				}
				suite.keeper.SetParams(suite.ctx, params)

				validator := stakingtypes.Validator{
					OperatorAddress: sdk.ValAddress("validator1").String(),
					Tokens:          sdkmath.NewInt(2000000),
				}
				suite.app.StakingKeeper.SetValidator(suite.ctx, validator)
			},
			execute: func() error {
				return suite.keeper.GetHooks().Hooks().AfterValidatorBeginUnbonding(suite.ctx, sdk.ConsAddress("cons2"), sdk.ValAddress("validator1"))
			},
			validate: func() error {
				result, found := suite.keeper.QuerySuperNode(suite.ctx, sdk.ValAddress("validator1"))
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

func (suite *KeeperIntegrationSuite) TestAfterValidatorRemovedHook() {
	tests := []struct {
		name          string
		setup         func()
		execute       func() error
		validate      func() error
		expectSuccess bool
	}{
		{
			name: "when the validator is removed, and is active, it should be disabled",
			setup: func() {
				supernode := types.SuperNode{
					ValidatorAddress: sdk.ValAddress("validator1").String(),
					States:           []*types.SuperNodeStateRecord{{State: types.SuperNodeStateActive}},
				}
				suite.keeper.SetSuperNode(suite.ctx, supernode)
			},
			execute: func() error {
				return suite.keeper.GetHooks().Hooks().AfterValidatorRemoved(suite.ctx, sdk.ConsAddress("cons1"), sdk.ValAddress("validator1"))
			},
			validate: func() error {
				result, found := suite.keeper.QuerySuperNode(suite.ctx, sdk.ValAddress("validator1"))
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

func (suite *KeeperIntegrationSuite) TestBeforeValidatorModifiedHook() {
	tests := []struct {
		name          string
		setup         func()
		execute       func() error
		validate      func() error
		expectSuccess bool
	}{
		{
			name: "before the validator is modified, if the validator do not meet supernode requirements, it should be disabled",
			setup: func() {
				supernode := types.SuperNode{
					ValidatorAddress: sdk.ValAddress("validator1").String(),
					States:           []*types.SuperNodeStateRecord{{State: types.SuperNodeStateDisabled}},
				}
				suite.keeper.SetSuperNode(suite.ctx, supernode)
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
				return suite.keeper.GetHooks().Hooks().BeforeValidatorModified(suite.ctx, sdk.ValAddress("validator1"))
			},
			validate: func() error {
				result, found := suite.keeper.QuerySuperNode(suite.ctx, sdk.ValAddress("validator1"))
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
		{
			name: "before the validator is modified, if the validator meet supernode requirements, it should be active",
			setup: func() {
				supernode := types.SuperNode{
					ValidatorAddress: sdk.ValAddress("validator1b").String(),
					States:           []*types.SuperNodeStateRecord{{State: types.SuperNodeStateDisabled}},
				}
				suite.keeper.SetSuperNode(suite.ctx, supernode)

				params := types.Params{
					MinimumStakeForSn: 1000000,
				}
				suite.keeper.SetParams(suite.ctx, params)

				validator := stakingtypes.Validator{
					OperatorAddress: sdk.ValAddress("validator1b").String(),
					Tokens:          sdkmath.NewInt(2000000),
					DelegatorShares: sdkmath.LegacyNewDec(2000000), // Ensure shares match self-delegation
				}
				suite.app.StakingKeeper.SetValidator(suite.ctx, validator)

				// Set self-delegation
				selfDelegation := stakingtypes.Delegation{
					DelegatorAddress: sdk.AccAddress(("validator1b")).String(),
					ValidatorAddress: sdk.ValAddress("validator1b").String(),
					Shares:           sdkmath.LegacyNewDec(2000000), // Self-delegation should match shares
				}
				suite.app.StakingKeeper.SetDelegation(suite.ctx, selfDelegation)
			},
			execute: func() error {
				return suite.keeper.GetHooks().Hooks().BeforeValidatorModified(suite.ctx, sdk.ValAddress("validator1b"))
			},
			validate: func() error {
				result, found := suite.keeper.QuerySuperNode(suite.ctx, sdk.ValAddress("validator1b"))
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

func (suite *KeeperIntegrationSuite) TestBeforeDelegationSharesModifiedHook() {
	tests := []struct {
		name          string
		setup         func()
		execute       func() error
		validate      func() error
		expectSuccess bool
	}{
		{
			name: "before the delegation shares are modified, if the validator meet supernode requirements, it should be active",
			setup: func() {
				supernode := types.SuperNode{
					ValidatorAddress: sdk.ValAddress("validator3").String(),
					States:           []*types.SuperNodeStateRecord{{State: types.SuperNodeStateDisabled}},
				}
				suite.keeper.SetSuperNode(suite.ctx, supernode)
				params := types.Params{
					MinimumStakeForSn: 1000000,
				}
				suite.keeper.SetParams(suite.ctx, params)

				validator := stakingtypes.Validator{
					OperatorAddress: sdk.ValAddress("validator3").String(),
					Tokens:          sdkmath.NewInt(1500000),
					DelegatorShares: sdkmath.LegacyNewDec(2000000), // Ensure shares match self-delegation
				}
				suite.app.StakingKeeper.SetValidator(suite.ctx, validator)

				// Set self-delegation
				selfDelegation := stakingtypes.Delegation{
					DelegatorAddress: sdk.AccAddress(("validator3")).String(),
					ValidatorAddress: sdk.ValAddress("validator3").String(),
					Shares:           sdkmath.LegacyNewDec(2000000), // Self-delegation should match shares
				}
				suite.app.StakingKeeper.SetDelegation(suite.ctx, selfDelegation)
			},
			execute: func() error {
				return suite.keeper.GetHooks().Hooks().BeforeDelegationSharesModified(suite.ctx, sdk.AccAddress("validator3"), sdk.ValAddress("validator3"))
			},
			validate: func() error {
				result, found := suite.keeper.QuerySuperNode(suite.ctx, sdk.ValAddress("validator3"))
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
