package keeper_test

import (
	"fmt"
	keeper2 "github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/mocks"
	types2 "github.com/LumeraProtocol/lumera/x/supernode/v1/types"

	"context"
	"errors"
	"testing"

	"github.com/cosmos/cosmos-sdk/types/query"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestKeeper_SetAndQuerySuperNode(t *testing.T) {
	valAddr := sdk.ValAddress([]byte("validator"))
	anotherValAddr := sdk.ValAddress([]byte("another-validator"))

	supernode := types2.SuperNode{
		ValidatorAddress: valAddr.String(),
		SupernodeAccount: sdk.AccAddress(valAddr).String(),
		Version:          "1.0.0",
		PrevIpAddresses: []*types2.IPAddressHistory{
			{
				Address: "1022.145.1.1",
				Height:  1,
			},
		},
		States: []*types2.SuperNodeStateRecord{
			{
				State:  types2.SuperNodeStateActive,
				Height: 1,
			},
		},
	}

	testCases := []struct {
		name        string
		setupState  func(k keeper2.Keeper, ctx sdk.Context)
		run         func(k keeper2.Keeper, ctx sdk.Context) (interface{}, error)
		expectedErr error
		checkResult func(t *testing.T, k keeper2.Keeper, ctx sdk.Context, result interface{})
	}{
		{
			name: "set and query existing supernode",
			setupState: func(k keeper2.Keeper, ctx sdk.Context) {
				// No pre-state needed
			},
			run: func(k keeper2.Keeper, ctx sdk.Context) (interface{}, error) {
				if err := k.SetSuperNode(ctx, supernode); err != nil {
					return nil, err
				}
				got, found := k.QuerySuperNode(ctx, valAddr)
				if !found {
					return nil, fmt.Errorf("supernode not found after setting it")
				}
				return got, nil
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, k keeper2.Keeper, ctx sdk.Context, result interface{}) {
				got := result.(types2.SuperNode)
				require.Equal(t, supernode, got)
			},
		},
		{
			name: "query non-existent supernode",
			setupState: func(k keeper2.Keeper, ctx sdk.Context) {
				// No supernode set
			},
			run: func(k keeper2.Keeper, ctx sdk.Context) (interface{}, error) {
				got, found := k.QuerySuperNode(ctx, anotherValAddr)
				if found {
					return got, fmt.Errorf("found supernode that should not exist")
				}
				return nil, nil
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, k keeper2.Keeper, ctx sdk.Context, result interface{}) {
				// No result expected, just ensure no error and no supernode found
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			stakingKeeper := supernodemocks.NewMockStakingKeeper(ctrl)
			slashingKeeper := supernodemocks.NewMockSlashingKeeper(ctrl)
			bankKeeper := supernodemocks.NewMockBankKeeper(ctrl)

			k, ctx := setupKeeperForTest(t, stakingKeeper, slashingKeeper, bankKeeper)

			if tc.setupState != nil {
				tc.setupState(k, ctx)
			}

			result, err := tc.run(k, ctx)
			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
			}

			if tc.checkResult != nil {
				tc.checkResult(t, k, ctx, result)
			}
		})
	}
}

func TestKeeper_GetAllSuperNodes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// We'll add at least one state record so the SuperNode won't be skipped.
	valAddr1 := sdk.ValAddress([]byte("val1"))
	valAddr2 := sdk.ValAddress([]byte("val2"))
	accAddr := sdk.AccAddress([]byte("acc1")).String()

	sn1 := types2.SuperNode{
		SupernodeAccount: accAddr,
		ValidatorAddress: valAddr1.String(),
		Version:          "1.0.0",
		States: []*types2.SuperNodeStateRecord{
			{
				State:  types2.SuperNodeStateActive,
				Height: 1, // or any block height, e.g. 1
			},
		},
		PrevIpAddresses: []*types2.IPAddressHistory{
			{
				Address: "1022.145.1.1",
				Height:  1,
			},
		},
	}

	sn2 := types2.SuperNode{
		SupernodeAccount: accAddr,
		ValidatorAddress: valAddr2.String(),
		Version:          "2.0.0",
		States: []*types2.SuperNodeStateRecord{
			{
				State:  types2.SuperNodeStateActive,
				Height: 1,
			},
		},
		PrevIpAddresses: []*types2.IPAddressHistory{
			{
				Address: "1022.145.1.1",
				Height:  1,
			},
		},
	}

	testCases := []struct {
		name        string
		setupState  func(k keeper2.Keeper, ctx sdk.Context)
		run         func(k keeper2.Keeper, ctx sdk.Context) (interface{}, error)
		expectedErr error
		checkResult func(t *testing.T, k keeper2.Keeper, ctx sdk.Context, result interface{})
	}{
		{
			name: "no supernodes",
			setupState: func(k keeper2.Keeper, ctx sdk.Context) {
				// no setup => store is empty
			},
			run: func(k keeper2.Keeper, ctx sdk.Context) (interface{}, error) {
				return k.GetAllSuperNodes(ctx)
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, k keeper2.Keeper, ctx sdk.Context, result interface{}) {
				snList := result.([]types2.SuperNode)
				require.Empty(t, snList)
			},
		},
		{
			name: "multiple supernodes",
			setupState: func(k keeper2.Keeper, ctx sdk.Context) {
				require.NoError(t, k.SetSuperNode(ctx, sn1))
				require.NoError(t, k.SetSuperNode(ctx, sn2))
			},
			run: func(k keeper2.Keeper, ctx sdk.Context) (interface{}, error) {
				return k.GetAllSuperNodes(ctx)
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, k keeper2.Keeper, ctx sdk.Context, result interface{}) {
				snList := result.([]types2.SuperNode)
				require.Len(t, snList, 2)
				// Check both present
				foundVal1, foundVal2 := false, false
				for _, s := range snList {
					if s.ValidatorAddress == sn1.ValidatorAddress {
						foundVal1 = true
					}
					if s.ValidatorAddress == sn2.ValidatorAddress {
						foundVal2 = true
					}
				}
				require.True(t, foundVal1)
				require.True(t, foundVal2)
			},
		},
		{
			name: "filter by state - only active",
			setupState: func(k keeper2.Keeper, ctx sdk.Context) {
				sn2Updated := sn2
				sn2Updated.States = append(sn2Updated.States, &types2.SuperNodeStateRecord{
					State:  types2.SuperNodeStateDisabled,
					Height: 2, // so the last state is Disabled
				})

				require.NoError(t, k.SetSuperNode(ctx, sn1))
				require.NoError(t, k.SetSuperNode(ctx, sn2Updated))
			},
			run: func(k keeper2.Keeper, ctx sdk.Context) (interface{}, error) {
				// Only return active supernodes
				return k.GetAllSuperNodes(ctx, types2.SuperNodeStateActive)
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, k keeper2.Keeper, ctx sdk.Context, result interface{}) {
				snList := result.([]types2.SuperNode)
				require.Len(t, snList, 1)
				require.Equal(t, sn1.ValidatorAddress, snList[0].ValidatorAddress)
			},
		},
		{
			name: "filter by state - unspecified (no filter)",
			setupState: func(k keeper2.Keeper, ctx sdk.Context) {
				require.NoError(t, k.SetSuperNode(ctx, sn1))
				require.NoError(t, k.SetSuperNode(ctx, sn2))
			},
			run: func(k keeper2.Keeper, ctx sdk.Context) (interface{}, error) {
				// Unspecified means no filtering
				return k.GetAllSuperNodes(ctx, types2.SuperNodeStateUnspecified)
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, k keeper2.Keeper, ctx sdk.Context, result interface{}) {
				snList := result.([]types2.SuperNode)
				require.Len(t, snList, 2)
			},
		},
		{
			name: "filter by state - skip non-active",
			setupState: func(k keeper2.Keeper, ctx sdk.Context) {
				sn2Updated := sn2
				sn2Updated.States = append(sn2Updated.States, &types2.SuperNodeStateRecord{
					State:  types2.SuperNodeStateDisabled,
					Height: 2, // so the last state is Disabled
				})
				require.NoError(t, k.SetSuperNode(ctx, sn2Updated))
			},
			run: func(k keeper2.Keeper, ctx sdk.Context) (interface{}, error) {
				return k.GetAllSuperNodes(ctx, types2.SuperNodeStateActive)
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, k keeper2.Keeper, ctx sdk.Context, result interface{}) {
				snList := result.([]types2.SuperNode)
				require.Len(t, snList, 0)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			stakingKeeper := supernodemocks.NewMockStakingKeeper(ctrl)
			slashingKeeper := supernodemocks.NewMockSlashingKeeper(ctrl)
			bankKeeper := supernodemocks.NewMockBankKeeper(ctrl)

			k, ctx := setupKeeperForTest(t, stakingKeeper, slashingKeeper, bankKeeper)

			if tc.setupState != nil {
				tc.setupState(k, ctx)
			}

			result, err := tc.run(k, ctx)
			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
			}

			if tc.checkResult != nil {
				tc.checkResult(t, k, ctx, result)
			}
		})
	}
}

// We define a helper to create a supernode with an initial state record.
func makeSuperNodeWithOneState(valIndex int, state types2.SuperNodeState) types2.SuperNode {
	// Use valIndex to produce a stable unique address
	valAddr := sdk.ValAddress([]byte(fmt.Sprintf("val%d", valIndex)))
	accAddr := sdk.AccAddress([]byte(fmt.Sprintf("acc%d", valIndex)))
	sn := types2.SuperNode{
		ValidatorAddress: valAddr.String(),
		SupernodeAccount: accAddr.String(),
		Version:          "1.0.0",
		// Must have at least one record so we don't skip it
		States: []*types2.SuperNodeStateRecord{
			{
				State:  types2.SuperNodeStateActive, // e.g. Active, Stopped, etc.
				Height: 1,                           // arbitrary block for the "registration"
			},
			{
				State:  state, // e.g. Active, Stopped, etc.
				Height: 1,     // arbitrary block for the "registration"
			},
		},
		PrevIpAddresses: []*types2.IPAddressHistory{
			{
				Address: "1022.145.1.1",
				Height:  1,
			},
		},
	}
	return sn
}

func TestKeeper_GetSuperNodesPaginated(t *testing.T) {
	// We'll build a set of 5 supernodes. By default, let's make them all "Active" at Height=1.
	supernodeCount := 5
	supernodes := make([]types2.SuperNode, supernodeCount)
	for i := 0; i < supernodeCount; i++ {
		sn := makeSuperNodeWithOneState(i, types2.SuperNodeStateActive)
		supernodes[i] = sn
	}

	testCases := []struct {
		name         string
		setupState   func(k keeper2.Keeper, ctx sdk.Context)
		pagination   *query.PageRequest
		stateFilters []types2.SuperNodeState
		expectedErr  error
		checkResult  func(t *testing.T, k keeper2.Keeper, ctx sdk.Context, snRes []*types2.SuperNode, pageRes *query.PageResponse)
	}{
		{
			name: "empty store, no results",
			setupState: func(k keeper2.Keeper, ctx sdk.Context) {
				// no supernodes set => store is empty
			},
			pagination:   &query.PageRequest{Limit: 10},
			stateFilters: []types2.SuperNodeState{}, // no filter
			expectedErr:  nil,
			checkResult: func(t *testing.T, k keeper2.Keeper, ctx sdk.Context, snRes []*types2.SuperNode, pageRes *query.PageResponse) {
				require.Empty(t, snRes)
				require.Nil(t, pageRes.NextKey)
			},
		},
		{
			name: "less results than limit",
			setupState: func(k keeper2.Keeper, ctx sdk.Context) {
				// Set fewer supernodes than limit => only 2
				require.NoError(t, k.SetSuperNode(ctx, supernodes[0]))
				require.NoError(t, k.SetSuperNode(ctx, supernodes[1]))
			},
			pagination:   &query.PageRequest{Limit: 10}, // limit > total supernodes = 2
			stateFilters: []types2.SuperNodeState{},
			expectedErr:  nil,
			checkResult: func(t *testing.T, k keeper2.Keeper, ctx sdk.Context, snRes []*types2.SuperNode, pageRes *query.PageResponse) {
				require.Len(t, snRes, 2)
				require.Nil(t, pageRes.NextKey)
			},
		},
		{
			name: "exact match: limit equals number of supernodes",
			setupState: func(k keeper2.Keeper, ctx sdk.Context) {
				for _, sn := range supernodes {
					require.NoError(t, k.SetSuperNode(ctx, sn))
				}
			},
			pagination:   &query.PageRequest{Limit: uint64(supernodeCount)}, // exactly 5
			stateFilters: []types2.SuperNodeState{},
			expectedErr:  nil,
			checkResult: func(t *testing.T, k keeper2.Keeper, ctx sdk.Context, snRes []*types2.SuperNode, pageRes *query.PageResponse) {
				require.Len(t, snRes, supernodeCount)
				require.Nil(t, pageRes.NextKey)
			},
		},
		{
			name: "pagination with multiple pages",
			setupState: func(k keeper2.Keeper, ctx sdk.Context) {
				for _, sn := range supernodes {
					require.NoError(t, k.SetSuperNode(ctx, sn))
				}
			},
			pagination:   &query.PageRequest{Limit: 2},
			stateFilters: []types2.SuperNodeState{},
			expectedErr:  nil,
			checkResult: func(t *testing.T, k keeper2.Keeper, ctx sdk.Context, snRes []*types2.SuperNode, pageRes *query.PageResponse) {
				// first call => 2 results
				require.Len(t, snRes, 2)
				require.NotNil(t, pageRes.NextKey)

				// second page
				secondPageSn, secondPageRes, err := k.GetSuperNodesPaginated(ctx, &query.PageRequest{
					Key:   pageRes.NextKey,
					Limit: 2,
				})
				require.NoError(t, err)
				require.Len(t, secondPageSn, 2)
				require.NotNil(t, secondPageRes.NextKey)

				// third page => only 1 left
				thirdPageSn, thirdPageRes, err := k.GetSuperNodesPaginated(ctx, &query.PageRequest{
					Key:   secondPageRes.NextKey,
					Limit: 2,
				})
				require.NoError(t, err)
				require.Len(t, thirdPageSn, 1)
				require.Nil(t, thirdPageRes.NextKey)
			},
		},
		{
			name: "filter by state (only active)",
			setupState: func(k keeper2.Keeper, ctx sdk.Context) {
				// We'll set 3 as active, 2 as stopped
				// The first 3 are active, the last 2 become stopped
				for i := 0; i < supernodeCount; i++ {
					sn := supernodes[i]
					if i >= 3 {
						// override last state to be Stopped
						sn.States[len(sn.States)-1].State = types2.SuperNodeStateStopped
					} else {
						// keep them active
						sn.States[len(sn.States)-1].State = types2.SuperNodeStateActive
					}
					require.NoError(t, k.SetSuperNode(ctx, sn))
				}
			},
			pagination:   &query.PageRequest{Limit: 10},
			stateFilters: []types2.SuperNodeState{types2.SuperNodeStateActive},
			expectedErr:  nil,
			checkResult: func(t *testing.T, k keeper2.Keeper, ctx sdk.Context, snRes []*types2.SuperNode, pageRes *query.PageResponse) {
				// The first 3 are active, the last 2 are stopped => we expect 3
				require.Len(t, snRes, 3)
			},
		},
		{
			name: "unspecified in filter (no filtering)",
			setupState: func(k keeper2.Keeper, ctx sdk.Context) {
				for _, sn := range supernodes {
					require.NoError(t, k.SetSuperNode(ctx, sn))
				}
			},
			pagination:   &query.PageRequest{Limit: 10},
			stateFilters: []types2.SuperNodeState{types2.SuperNodeStateUnspecified},
			expectedErr:  nil,
			checkResult: func(t *testing.T, k keeper2.Keeper, ctx sdk.Context, snRes []*types2.SuperNode, pageRes *query.PageResponse) {
				require.Len(t, snRes, supernodeCount) // no filtering, so all 5
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			stakingKeeper := supernodemocks.NewMockStakingKeeper(ctrl)
			slashingKeeper := supernodemocks.NewMockSlashingKeeper(ctrl)
			bankKeeper := supernodemocks.NewMockBankKeeper(ctrl)

			k, ctx := setupKeeperForTest(t, stakingKeeper, slashingKeeper, bankKeeper)

			if tc.setupState != nil {
				tc.setupState(k, ctx)
			}

			snRes, pageRes, err := k.GetSuperNodesPaginated(ctx, tc.pagination, tc.stateFilters...)
			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
			}

			if tc.checkResult != nil {
				tc.checkResult(t, k, ctx, snRes, pageRes)
			}
		})
	}
}

func TestCheckValidatorSupernodeEligibility(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Example val address
	valOperatorAddr := sdk.ValAddress([]byte("valoper-test"))
	valAddrString := valOperatorAddr.String()

	// Test cases
	testCases := []struct {
		name                 string
		validator            *stakingtypes.Validator
		selfDelegationFound  bool
		selfDelegationShares sdkmath.LegacyDec

		expectErr bool
		errSubstr string
	}{
		{
			name: "validator unbonded, but no self-delegation => error",
			validator: &stakingtypes.Validator{
				OperatorAddress: valAddrString,
				Status:          stakingtypes.Unbonded,
			},
			selfDelegationFound: false,
			expectErr:           true,
			errSubstr:           "no self-delegation",
		},
		{
			name: "validator unbonded, self-delegation < min => error",
			validator: &stakingtypes.Validator{
				OperatorAddress: valAddrString,
				Status:          stakingtypes.Unbonded,
				DelegatorShares: sdkmath.LegacyNewDec(1000000),
				Tokens:          sdkmath.NewInt(1000000),
			},
			selfDelegationFound:  true,
			selfDelegationShares: sdkmath.LegacyNewDec(500000),
			expectErr:            true,
			errSubstr:            "does not meet minimum self stake",
		},
		{
			name: "validator unbonded, self-delegation >= min => no error",
			validator: &stakingtypes.Validator{
				OperatorAddress: valAddrString,
				Status:          stakingtypes.Unbonded,
				DelegatorShares: sdkmath.LegacyNewDec(1000000),
				Tokens:          sdkmath.NewInt(1000000),
			},
			selfDelegationFound:  true,
			selfDelegationShares: sdkmath.LegacyNewDec(1000000),
			expectErr:            false,
		},
		{
			name: "delegation share 0, shouldn't panic => error",
			validator: &stakingtypes.Validator{
				OperatorAddress: valAddrString,
				Status:          stakingtypes.Unbonded,
				Tokens:          sdkmath.NewInt(1000000),
				DelegatorShares: sdkmath.LegacyNewDec(0),
			},
			selfDelegationFound:  true,
			selfDelegationShares: sdkmath.LegacyNewDec(500000),
			expectErr:            true,
			errSubstr:            "no self-stake available",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Mock out the Delegation(...) call

			stakingKeeper := supernodemocks.NewMockStakingKeeper(ctrl)
			slashingKeeper := supernodemocks.NewMockSlashingKeeper(ctrl)
			bankKeeper := supernodemocks.NewMockBankKeeper(ctrl)

			stakingKeeper.EXPECT().
				Delegation(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) (stakingtypes.Delegation, error) {
					if tc.selfDelegationFound {
						return stakingtypes.Delegation{
							DelegatorAddress: delAddr.String(),
							ValidatorAddress: valAddr.String(),
							Shares:           tc.selfDelegationShares,
						}, nil
					}
					return stakingtypes.Delegation{}, errors.New("no self-delegation")
				}).
				MaxTimes(1)

			k, ctx := setupKeeperForTest(t, stakingKeeper, slashingKeeper, bankKeeper)
			msgServer := keeper2.NewMsgServerImpl(k)

			// Call your function
			err := msgServer.CheckValidatorSupernodeEligibility(ctx, tc.validator, valAddrString)
			if tc.expectErr {
				require.Error(t, err)
				if tc.errSubstr != "" {
					require.Contains(t, err.Error(), tc.errSubstr)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
