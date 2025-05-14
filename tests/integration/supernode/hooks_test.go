package integration_test

import (
	"fmt"
	types2 "github.com/LumeraProtocol/lumera/x/supernode/v1/types"

	sdkmath "cosmossdk.io/math"

	_ "cosmossdk.io/log"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

func (suite *KeeperIntegrationSuite) TestAfterValidatorBondedHook() {
	tests := []struct {
		name          string
		setup         func()
		execute       func() error
		validate      func() error
		expectSuccess bool
	}{
		{
			name: "when validator is bonded and meet supernode requirements, it should be active",
			setup: func() {
				params := types2.Params{
					MinimumStakeForSn: sdk.NewCoin("ulume", sdkmath.NewInt(1000000)),
				}
				suite.keeper.SetParams(suite.ctx, params)

				supernode := types2.SuperNode{
					ValidatorAddress: sdk.ValAddress([]byte("validator1c")).String(),
					SupernodeAccount: sdk.AccAddress([]byte("validator1c")).String(),
					Version:          "1.0.0",
					States:           []*types2.SuperNodeStateRecord{{State: types2.SuperNodeStateActive}},
					PrevIpAddresses:  []*types2.IPAddressHistory{{Address: "192.168.1.1"}},
					P2PPort:          "26657",
				}
				suite.keeper.SetSuperNode(suite.ctx, supernode)

				validator := stakingtypes.Validator{
					OperatorAddress: sdk.ValAddress("validator1c").String(),
					Tokens:          sdkmath.NewInt(2000000),
					DelegatorShares: sdkmath.LegacyNewDec(2000000), // Ensure shares match self-delegation
				}
				suite.app.StakingKeeper.SetValidator(suite.ctx, validator)

				// Set self-delegation
				selfDelegation := stakingtypes.Delegation{
					DelegatorAddress: sdk.AccAddress(("validator1c")).String(),
					ValidatorAddress: sdk.ValAddress("validator1c").String(),
					Shares:           sdkmath.LegacyNewDec(2000000), // Self-delegation should match shares
				}
				suite.app.StakingKeeper.SetDelegation(suite.ctx, selfDelegation)
			},
			execute: func() error {
				return suite.keeper.Hooks().AfterValidatorBonded(suite.ctx, sdk.ConsAddress("cons2"), sdk.ValAddress("validator1c"))
			},
			validate: func() error {
				result, found := suite.keeper.QuerySuperNode(suite.ctx, sdk.ValAddress("validator1c"))
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
			name: "when the validator is bonded but jailed, it should disabled",
			setup: func() {
				params := types2.Params{
					MinimumStakeForSn: sdk.NewCoin("ulume", sdkmath.NewInt(1000000)),
				}
				suite.keeper.SetParams(suite.ctx, params)

				supernode := types2.SuperNode{
					ValidatorAddress: sdk.ValAddress([]byte("validator1j")).String(),
					SupernodeAccount: sdk.AccAddress([]byte("validator1j")).String(),
					Version:          "1.0.0",
					States:           []*types2.SuperNodeStateRecord{{State: types2.SuperNodeStateActive}},
					PrevIpAddresses:  []*types2.IPAddressHistory{{Address: "192.168.1.1"}},
					P2PPort:          "26657",
				}
				suite.keeper.SetSuperNode(suite.ctx, supernode)

				validator := stakingtypes.Validator{
					OperatorAddress: sdk.ValAddress("validator1j").String(),
					Tokens:          sdkmath.NewInt(2000000),
					DelegatorShares: sdkmath.LegacyNewDec(2000000), // Ensure shares match self-delegation
					Jailed:          true,
				}
				suite.app.StakingKeeper.SetValidator(suite.ctx, validator)

				// Set self-delegation
				selfDelegation := stakingtypes.Delegation{
					DelegatorAddress: sdk.AccAddress(("validator1j")).String(),
					ValidatorAddress: sdk.ValAddress("validator1j").String(),
					Shares:           sdkmath.LegacyNewDec(2000000), // Self-delegation should match shares
				}
				suite.app.StakingKeeper.SetDelegation(suite.ctx, selfDelegation)
			},
			execute: func() error {
				return suite.keeper.Hooks().AfterValidatorBonded(suite.ctx, sdk.ConsAddress("cons2"), sdk.ValAddress("validator1j"))
			},
			validate: func() error {
				result, found := suite.keeper.QuerySuperNode(suite.ctx, sdk.ValAddress("validator1j"))
				if !found {
					return fmt.Errorf("SuperNode not found")
				}
				if result.States[len(result.States)-1].State != types2.SuperNodeStateDisabled {
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
			name: "when the validator begins un-bonding and the stake falls below minimum but is not jailed, it should be disabled",
			setup: func() {
				params := types2.Params{
					MinimumStakeForSn: sdk.NewCoin("ulume", sdkmath.NewInt(1000000)),
				}
				suite.keeper.SetParams(suite.ctx, params)

				supernode := types2.SuperNode{
					ValidatorAddress: sdk.ValAddress([]byte("validator1bu")).String(),
					SupernodeAccount: sdk.AccAddress([]byte("validator1bu")).String(),
					Version:          "1.0.0",
					States:           []*types2.SuperNodeStateRecord{{State: types2.SuperNodeStateActive}},
					PrevIpAddresses:  []*types2.IPAddressHistory{{Address: "192.168.1.1"}},
					P2PPort:          "26657",
				}
				suite.keeper.SetSuperNode(suite.ctx, supernode)

				validator := stakingtypes.Validator{
					OperatorAddress: sdk.ValAddress("validator1bu").String(),
					Tokens:          sdkmath.NewInt(500000),
				}
				suite.app.StakingKeeper.SetValidator(suite.ctx, validator)
			},
			execute: func() error {
				return suite.keeper.Hooks().AfterValidatorBeginUnbonding(suite.ctx, sdk.ConsAddress("cons2"), sdk.ValAddress("validator1bu"))
			},
			validate: func() error {
				result, found := suite.keeper.QuerySuperNode(suite.ctx, sdk.ValAddress("validator1bu"))
				if !found {
					return fmt.Errorf("SuperNode not found")
				}
				if result.States[len(result.States)-1].State != types2.SuperNodeStateDisabled {
					return fmt.Errorf("expected SuperNode to be disabled")
				}
				return nil
			},
			expectSuccess: true,
		},
		{
			name: "when the validator begins un-bonding and stake does not fall below minimum but is jailed, it should be disabled",
			setup: func() {
				params := types2.Params{
					MinimumStakeForSn: sdk.NewCoin("ulume", sdkmath.NewInt(1000000)),
				}
				suite.keeper.SetParams(suite.ctx, params)

				supernode := types2.SuperNode{
					ValidatorAddress: sdk.ValAddress([]byte("validator1ju")).String(),
					SupernodeAccount: sdk.AccAddress([]byte("validator1ju")).String(),
					Version:          "1.0.0",
					States:           []*types2.SuperNodeStateRecord{{State: types2.SuperNodeStateActive}},
					PrevIpAddresses:  []*types2.IPAddressHistory{{Address: "192.168.1.1"}},
					P2PPort:          "26657",
				}
				suite.keeper.SetSuperNode(suite.ctx, supernode)

				validator := stakingtypes.Validator{
					OperatorAddress: sdk.ValAddress("validator1ju").String(),
					Tokens:          sdkmath.NewInt(2000000),
					Jailed:          true,
					DelegatorShares: sdkmath.LegacyNewDec(2000000),
				}
				suite.app.StakingKeeper.SetValidator(suite.ctx, validator)

				// Set self-delegation
				selfDelegation := stakingtypes.Delegation{
					DelegatorAddress: sdk.AccAddress(("validator1jua")).String(),
					ValidatorAddress: sdk.ValAddress("validator1jua").String(),
					Shares:           sdkmath.LegacyNewDec(2000000), // Self-delegation should match shares
				}
				suite.app.StakingKeeper.SetDelegation(suite.ctx, selfDelegation)
			},
			execute: func() error {
				return suite.keeper.Hooks().AfterValidatorBeginUnbonding(suite.ctx, sdk.ConsAddress("cons2"), sdk.ValAddress("validator1ju"))
			},
			validate: func() error {
				result, found := suite.keeper.QuerySuperNode(suite.ctx, sdk.ValAddress("validator1ju"))
				if !found {
					return fmt.Errorf("SuperNode not found")
				}
				if result.States[len(result.States)-1].State != types2.SuperNodeStateDisabled {
					return fmt.Errorf("expected SuperNode to be disabled")
				}
				return nil
			},
			expectSuccess: true,
		},
		{
			name: "when the validator begins un-bonding but stake does not fall below minimum and is not jailed, it should not be disabled",
			setup: func() {
				params := types2.Params{
					MinimumStakeForSn: sdk.NewCoin("ulume", sdkmath.NewInt(1000000)),
				}
				suite.keeper.SetParams(suite.ctx, params)

				supernode := types2.SuperNode{
					ValidatorAddress: sdk.ValAddress([]byte("validator1jua")).String(),
					SupernodeAccount: sdk.AccAddress([]byte("validator1jua")).String(),
					Version:          "1.0.0",
					States:           []*types2.SuperNodeStateRecord{{State: types2.SuperNodeStateActive}},
					PrevIpAddresses:  []*types2.IPAddressHistory{{Address: "192.168.1.1"}},
					P2PPort:          "26657",
				}
				suite.keeper.SetSuperNode(suite.ctx, supernode)

				validator := stakingtypes.Validator{
					OperatorAddress: sdk.ValAddress("validator1jua").String(),
					Tokens:          sdkmath.NewInt(2000000),
					DelegatorShares: sdkmath.LegacyNewDec(2000000),
				}
				suite.app.StakingKeeper.SetValidator(suite.ctx, validator)

				// Set self-delegation
				selfDelegation := stakingtypes.Delegation{
					DelegatorAddress: sdk.AccAddress(("validator1jua")).String(),
					ValidatorAddress: sdk.ValAddress("validator1jua").String(),
					Shares:           sdkmath.LegacyNewDec(2000000), // Self-delegation should match shares
				}
				suite.app.StakingKeeper.SetDelegation(suite.ctx, selfDelegation)
			},
			execute: func() error {
				return suite.keeper.Hooks().AfterValidatorBeginUnbonding(suite.ctx, sdk.ConsAddress("cons32"), sdk.ValAddress("validator1jua"))
			},
			validate: func() error {
				result, found := suite.keeper.QuerySuperNode(suite.ctx, sdk.ValAddress("validator1jua"))
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
				supernode := types2.SuperNode{
					ValidatorAddress: sdk.ValAddress("validator1r").String(),
					SupernodeAccount: sdk.AccAddress([]byte("validator1r")).String(),
					Version:          "1.0.0",
					States:           []*types2.SuperNodeStateRecord{{State: types2.SuperNodeStateActive}},
					PrevIpAddresses:  []*types2.IPAddressHistory{{Address: "192.168.1.1"}},
					P2PPort:          "26657",
				}
				suite.keeper.SetSuperNode(suite.ctx, supernode)
			},
			execute: func() error {
				return suite.keeper.Hooks().AfterValidatorRemoved(suite.ctx, sdk.ConsAddress("cons1"), sdk.ValAddress("validator1r"))
			},
			validate: func() error {
				result, found := suite.keeper.QuerySuperNode(suite.ctx, sdk.ValAddress("validator1r"))
				if !found {
					return fmt.Errorf("SuperNode not found")
				}
				if result.States[len(result.States)-1].State != types2.SuperNodeStateDisabled {
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
				supernode := types2.SuperNode{
					ValidatorAddress: sdk.ValAddress("validator3").String(),
					SupernodeAccount: sdk.AccAddress([]byte("validator3")).String(),
					Version:          "1.0.0",
					States:           []*types2.SuperNodeStateRecord{{State: types2.SuperNodeStateActive}},
					PrevIpAddresses:  []*types2.IPAddressHistory{{Address: "192.168.1.1"}},
					P2PPort:          "26657",
				}
				suite.keeper.SetSuperNode(suite.ctx, supernode)
				params := types2.Params{
					MinimumStakeForSn: sdk.NewCoin("ulume", sdkmath.NewInt(1000000)),
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
				return suite.keeper.Hooks().BeforeDelegationSharesModified(suite.ctx, sdk.AccAddress("validator3"), sdk.ValAddress("validator3"))
			},
			validate: func() error {
				result, found := suite.keeper.QuerySuperNode(suite.ctx, sdk.ValAddress("validator3"))
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
