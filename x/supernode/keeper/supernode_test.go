package keeper_test

import (
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/golang/mock/gomock"
	"github.com/pastelnetwork/pastel/x/supernode/keeper"
	supernodemocks "github.com/pastelnetwork/pastel/x/supernode/mocks"
	"github.com/pastelnetwork/pastel/x/supernode/types"
	"github.com/stretchr/testify/require"
)

func TestKeeper_SetAndQuerySuperNode(t *testing.T) {
	valAddr := sdk.ValAddress([]byte("validator"))
	anotherValAddr := sdk.ValAddress([]byte("another-validator"))

	supernode := types.SuperNode{
		ValidatorAddress: valAddr.String(),
		Version:          "1.0.0",
	}

	testCases := []struct {
		name        string
		setupState  func(k keeper.Keeper, ctx sdk.Context)
		run         func(k keeper.Keeper, ctx sdk.Context) (interface{}, error)
		expectedErr error
		checkResult func(t *testing.T, k keeper.Keeper, ctx sdk.Context, result interface{})
	}{
		{
			name: "set and query existing supernode",
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				// No pre-state needed
			},
			run: func(k keeper.Keeper, ctx sdk.Context) (interface{}, error) {
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
			checkResult: func(t *testing.T, k keeper.Keeper, ctx sdk.Context, result interface{}) {
				got := result.(types.SuperNode)
				require.Equal(t, supernode, got)
			},
		},
		{
			name: "query non-existent supernode",
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				// No supernode set
			},
			run: func(k keeper.Keeper, ctx sdk.Context) (interface{}, error) {
				got, found := k.QuerySuperNode(ctx, anotherValAddr)
				if found {
					return got, fmt.Errorf("found supernode that should not exist")
				}
				return nil, nil
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, k keeper.Keeper, ctx sdk.Context, result interface{}) {
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
	valAddr1 := sdk.ValAddress([]byte("val1"))
	valAddr2 := sdk.ValAddress([]byte("val2"))

	sn1 := types.SuperNode{
		ValidatorAddress: valAddr1.String(),
		Version:          "1.0.0",
	}
	sn2 := types.SuperNode{
		ValidatorAddress: valAddr2.String(),
		Version:          "2.0.0",
	}

	testCases := []struct {
		name        string
		setupState  func(k keeper.Keeper, ctx sdk.Context)
		run         func(k keeper.Keeper, ctx sdk.Context) (interface{}, error)
		expectedErr error
		checkResult func(t *testing.T, k keeper.Keeper, ctx sdk.Context, result interface{})
	}{
		{
			name: "no supernodes",
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				// no setup
			},
			run: func(k keeper.Keeper, ctx sdk.Context) (interface{}, error) {
				return k.GetAllSuperNodes(ctx)
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, k keeper.Keeper, ctx sdk.Context, result interface{}) {
				snList := result.([]types.SuperNode)
				require.Empty(t, snList)
			},
		},
		{
			name: "multiple supernodes",
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				require.NoError(t, k.SetSuperNode(ctx, sn1))
				require.NoError(t, k.SetSuperNode(ctx, sn2))
			},
			run: func(k keeper.Keeper, ctx sdk.Context) (interface{}, error) {
				return k.GetAllSuperNodes(ctx)
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, k keeper.Keeper, ctx sdk.Context, result interface{}) {
				snList := result.([]types.SuperNode)
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
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				require.NoError(t, k.SetSuperNode(ctx, sn1))
				require.NoError(t, k.SetSuperNode(ctx, sn2))
			},
			run: func(k keeper.Keeper, ctx sdk.Context) (interface{}, error) {
				// Only return active supernodes
				return k.GetAllSuperNodes(ctx, types.SuperNodeStateActive)
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, k keeper.Keeper, ctx sdk.Context, result interface{}) {
				snList := result.([]types.SuperNode)
				require.Len(t, snList, 1)
				require.Equal(t, sn1.ValidatorAddress, snList[0].ValidatorAddress)
			},
		},
		{
			name: "filter by state - unspecified (no filter)",
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				require.NoError(t, k.SetSuperNode(ctx, sn1))
				require.NoError(t, k.SetSuperNode(ctx, sn2))
			},
			run: func(k keeper.Keeper, ctx sdk.Context) (interface{}, error) {
				// Unspecified means no filtering
				return k.GetAllSuperNodes(ctx, types.SuperNodeStateUnspecified)
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, k keeper.Keeper, ctx sdk.Context, result interface{}) {
				snList := result.([]types.SuperNode)
				require.Len(t, snList, 2)
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

func TestKeeper_GetSuperNodesPaginated(t *testing.T) {
	supernodeCount := 5
	supernodes := make([]types.SuperNode, supernodeCount)
	for i := 0; i < supernodeCount; i++ {
		sn := types.SuperNode{
			ValidatorAddress: sdk.ValAddress([]byte(fmt.Sprintf("val%d", i))).String(),
			Version:          "1.0.0",
		}
		supernodes[i] = sn
	}

	testCases := []struct {
		name        string
		setupState  func(k keeper.Keeper, ctx sdk.Context)
		pagination  *query.PageRequest
		stateFilter []types.SuperNodeState
		expectedErr error
		checkResult func(t *testing.T, k keeper.Keeper, ctx sdk.Context, snRes []*types.SuperNode, pageRes *query.PageResponse)
	}{
		{
			name: "empty store, no results",
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				// no supernodes set
			},
			pagination:  &query.PageRequest{Limit: 10},
			stateFilter: []types.SuperNodeState{}, // no filter
			expectedErr: nil,
			checkResult: func(t *testing.T, k keeper.Keeper, ctx sdk.Context, snRes []*types.SuperNode, pageRes *query.PageResponse) {
				require.Empty(t, snRes)
				require.Nil(t, pageRes.NextKey)
			},
		},
		{
			name: "less results than limit",
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				// Set fewer supernodes than limit
				require.NoError(t, k.SetSuperNode(ctx, supernodes[0]))
				require.NoError(t, k.SetSuperNode(ctx, supernodes[1]))
			},
			pagination:  &query.PageRequest{Limit: 10}, // limit > total supernodes = 2
			stateFilter: []types.SuperNodeState{},
			expectedErr: nil,
			checkResult: func(t *testing.T, k keeper.Keeper, ctx sdk.Context, snRes []*types.SuperNode, pageRes *query.PageResponse) {
				require.Len(t, snRes, 2)
				require.Nil(t, pageRes.NextKey)
			},
		},
		{
			name: "exact match: limit equals number of supernodes",
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				for _, sn := range supernodes {
					require.NoError(t, k.SetSuperNode(ctx, sn))
				}
			},
			pagination:  &query.PageRequest{Limit: uint64(supernodeCount)}, // exactly 5
			stateFilter: []types.SuperNodeState{},
			expectedErr: nil,
			checkResult: func(t *testing.T, k keeper.Keeper, ctx sdk.Context, snRes []*types.SuperNode, pageRes *query.PageResponse) {
				require.Len(t, snRes, supernodeCount)
				require.Nil(t, pageRes.NextKey)
			},
		},
		{
			name: "pagination with multiple pages",
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				for _, sn := range supernodes {
					require.NoError(t, k.SetSuperNode(ctx, sn))
				}
			},
			pagination:  &query.PageRequest{Limit: 2},
			stateFilter: []types.SuperNodeState{},
			expectedErr: nil,
			checkResult: func(t *testing.T, k keeper.Keeper, ctx sdk.Context, snRes []*types.SuperNode, pageRes *query.PageResponse) {
				require.Len(t, snRes, 2)
				require.NotNil(t, pageRes.NextKey)

				// Fetch second page
				secondPageSn, secondPageRes, err := k.GetSuperNodesPaginated(ctx, &query.PageRequest{
					Key:   pageRes.NextKey,
					Limit: 2,
				})
				require.NoError(t, err)
				require.Len(t, secondPageSn, 2)
				require.NotNil(t, secondPageRes.NextKey)

				// Third page
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
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				// Set 3 active, 2 stopped
				for i := 0; i < supernodeCount; i++ {
					sn := supernodes[i]
					if i >= 3 {
						//sn.State = types.SuperNodeStateStopped
					} else {
						//sn.State = types.SuperNodeStateActive
					}
					require.NoError(t, k.SetSuperNode(ctx, sn))
				}
			},
			pagination:  &query.PageRequest{Limit: 10},
			stateFilter: []types.SuperNodeState{types.SuperNodeStateActive},
			expectedErr: nil,
			checkResult: func(t *testing.T, k keeper.Keeper, ctx sdk.Context, snRes []*types.SuperNode, pageRes *query.PageResponse) {
				require.Len(t, snRes, 3)
				/*for _, sn := range snRes {
					//require.Equal(t, types.SuperNodeStateActive, sn.State)
				}*/
			},
		},
		{
			name: "unspecified in filter (no filtering)",
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				for _, sn := range supernodes {
					require.NoError(t, k.SetSuperNode(ctx, sn))
				}
			},
			pagination:  &query.PageRequest{Limit: 10},
			stateFilter: []types.SuperNodeState{types.SuperNodeStateUnspecified},
			expectedErr: nil,
			checkResult: func(t *testing.T, k keeper.Keeper, ctx sdk.Context, snRes []*types.SuperNode, pageRes *query.PageResponse) {
				require.Len(t, snRes, supernodeCount) // no filtering
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

			snRes, pageRes, err := k.GetSuperNodesPaginated(ctx, tc.pagination, tc.stateFilter...)
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
