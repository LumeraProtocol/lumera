package keeper_test

import (
	"fmt"
	"testing"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	"github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/action/v1/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"go.uber.org/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestKeeper_ListActions(t *testing.T) {
	actionID1 := "12345"
	actionID2 := "67890"
	actionID3 := "67891"
	price := sdk.NewInt64Coin("stake", 100)

	action1 := types.Action{
		Creator:        "creator1",
		ActionID:       actionID1,
		ActionType:     types.ActionTypeSense,
		Metadata:       []byte("metadata1"),
		Price:          price.String(),
		ExpirationTime: 1234567890,
		State:          types.ActionStateProcessing,
		BlockHeight:    100,
		SuperNodes:     []string{"supernode-1", "supernode-2"},
	}
	action2 := types.Action{
		Creator:        "creator2",
		ActionID:       actionID2,
		ActionType:     types.ActionTypeCascade,
		Metadata:       []byte("metadata2"),
		Price:          price.String(),
		ExpirationTime: 1234567891,
		State:          types.ActionStateApproved,
		BlockHeight:    100,
		SuperNodes:     []string{"supernode-1", "supernode-2"},
	}
	action3 := types.Action{
		Creator:        "creator3",
		ActionID:       actionID3,
		ActionType:     types.ActionTypeSense,
		Metadata:       []byte("metadata3"),
		Price:          price.String(),
		ExpirationTime: 1234567892,
		State:          types.ActionStateApproved,
		BlockHeight:    100,
		SuperNodes:     []string{"supernode-3"},
	}

	testCases := []struct {
		name        string
		req         *types.QueryListActionsRequest
		setupState  func(k keeper.Keeper, ctx sdk.Context)
		expectedErr error
		checkResult func(t *testing.T, resp *types.QueryListActionsResponse)
	}{
		{
			name:        "invalid request (nil request)",
			req:         nil,
			expectedErr: status.Error(codes.InvalidArgument, "invalid request"),
		},
		{
			name: "actions found for action-state filter",
			req: &types.QueryListActionsRequest{
				ActionState: types.ActionStateProcessing,
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				k.SetAction(ctx, &action1)
				k.SetAction(ctx, &action2)
				k.SetAction(ctx, &action3)
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types.QueryListActionsResponse) {
				require.NotNil(t, resp)
				require.Len(t, resp.Actions, 1)
				require.Equal(t, actionID1, resp.Actions[0].ActionID)
			},
		},
		{
			name: "actions found for action-type filter",
			req: &types.QueryListActionsRequest{
				ActionType: types.ActionTypeCascade,
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				k.SetAction(ctx, &action1)
				k.SetAction(ctx, &action2)
				k.SetAction(ctx, &action3)
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types.QueryListActionsResponse) {
				require.NotNil(t, resp)
				require.Len(t, resp.Actions, 1)
				require.Equal(t, actionID2, resp.Actions[0].ActionID)
			},
		},
		{
			name: "pagination works correctly",
			req: &types.QueryListActionsRequest{
				Pagination: &query.PageRequest{
					Offset: 1,
					Limit:  1,
				},
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				k.SetAction(ctx, &action1)
				k.SetAction(ctx, &action2)
				k.SetAction(ctx, &action3)
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types.QueryListActionsResponse) {
				require.NotNil(t, resp)
				require.Len(t, resp.Actions, 1)
				require.Equal(t, actionID2, resp.Actions[0].ActionID)
			},
		},
		{
			name: "pagination works with offset and limit",
			req: &types.QueryListActionsRequest{
				Pagination: &query.PageRequest{
					Offset: 0,
					Limit:  2,
				},
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				k.SetAction(ctx, &action1)
				k.SetAction(ctx, &action2)
				k.SetAction(ctx, &action3)
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types.QueryListActionsResponse) {
				require.NotNil(t, resp)
				require.Len(t, resp.Actions, 2)
				require.Equal(t, actionID1, resp.Actions[0].ActionID)
				require.Equal(t, actionID2, resp.Actions[1].ActionID)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			k, ctx := keepertest.ActionKeeper(t, ctrl)
			q := keeper.NewQueryServerImpl(k)

			if tc.setupState != nil {
				tc.setupState(k, ctx)
			}

			resp, err := q.ListActions(ctx, tc.req)

			if tc.expectedErr != nil {
				require.Error(t, err)
				st, _ := status.FromError(err)
				expectedStatus, _ := status.FromError(tc.expectedErr)
				require.Equal(t, expectedStatus.Code(), st.Code())
			} else {
				require.NoError(t, err)
				if tc.checkResult != nil {
					tc.checkResult(t, resp)
				}
			}
		})
	}
}

func TestKeeper_ListActions_ReversePaginationUsesNumericActionIDOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	k, ctx := keepertest.ActionKeeper(t, ctrl)
	q := keeper.NewQueryServerImpl(k)
	price := sdk.NewInt64Coin("stake", 100)

	// Reproduces the mainnet boundary where lexical ordering would place 99999 before 123611.
	actionLowLexical := types.Action{
		Creator:        "creator-low",
		ActionID:       "99999",
		ActionType:     types.ActionTypeCascade,
		Metadata:       []byte("metadata-low"),
		Price:          price.String(),
		ExpirationTime: 1234567891,
		State:          types.ActionStateApproved,
		BlockHeight:    100,
		SuperNodes:     []string{"supernode-1"},
	}
	actionHighNumeric := types.Action{
		Creator:        "creator-high",
		ActionID:       "123611",
		ActionType:     types.ActionTypeCascade,
		Metadata:       []byte("metadata-high"),
		Price:          price.String(),
		ExpirationTime: 1234567892,
		State:          types.ActionStateApproved,
		BlockHeight:    101,
		SuperNodes:     []string{"supernode-2"},
	}

	require.NoError(t, k.SetAction(ctx, &actionLowLexical))
	require.NoError(t, k.SetAction(ctx, &actionHighNumeric))

	resp, err := q.ListActions(ctx, &types.QueryListActionsRequest{
		Pagination: &query.PageRequest{
			Limit:   1,
			Reverse: true,
		},
	})
	require.NoError(t, err)
	require.Len(t, resp.Actions, 1)
	require.Equal(t, "123611", resp.Actions[0].ActionID)
}

func TestKeeper_ListActions_ReversePaginationCursorMaintainsNumericOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	k, ctx := keepertest.ActionKeeper(t, ctrl)
	q := keeper.NewQueryServerImpl(k)
	price := sdk.NewInt64Coin("stake", 100)

	ids := []string{"2", "10", "100"}
	for i, id := range ids {
		action := types.Action{
			Creator:        "creator-" + id,
			ActionID:       id,
			ActionType:     types.ActionTypeCascade,
			Metadata:       []byte("metadata-" + id),
			Price:          price.String(),
			ExpirationTime: int64(1234567890 + i),
			State:          types.ActionStateApproved,
			BlockHeight:    int64(100 + i),
			SuperNodes:     []string{"supernode-" + id},
		}
		require.NoError(t, k.SetAction(ctx, &action))
	}

	firstPage, err := q.ListActions(ctx, &types.QueryListActionsRequest{
		Pagination: &query.PageRequest{
			Limit:   1,
			Reverse: true,
		},
	})
	require.NoError(t, err)
	require.Len(t, firstPage.Actions, 1)
	require.Equal(t, "100", firstPage.Actions[0].ActionID)
	require.NotEmpty(t, firstPage.Pagination.NextKey)

	secondPage, err := q.ListActions(ctx, &types.QueryListActionsRequest{
		Pagination: &query.PageRequest{
			Key:     firstPage.Pagination.NextKey,
			Limit:   1,
			Reverse: true,
		},
	})
	require.NoError(t, err)
	require.Len(t, secondPage.Actions, 1)
	require.Equal(t, "10", secondPage.Actions[0].ActionID)
}

func TestKeeper_ListActions_ReversePaginationWithTypeFilterUsesNumericOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	k, ctx := keepertest.ActionKeeper(t, ctrl)
	q := keeper.NewQueryServerImpl(k)
	price := sdk.NewInt64Coin("stake", 100)

	actionLowLexical := types.Action{
		Creator:        "creator-low",
		ActionID:       "99999",
		ActionType:     types.ActionTypeCascade,
		Metadata:       []byte("metadata-low"),
		Price:          price.String(),
		ExpirationTime: 1234567891,
		State:          types.ActionStateApproved,
		BlockHeight:    100,
		SuperNodes:     []string{"supernode-1"},
	}
	actionHighNumeric := types.Action{
		Creator:        "creator-high",
		ActionID:       "123611",
		ActionType:     types.ActionTypeCascade,
		Metadata:       []byte("metadata-high"),
		Price:          price.String(),
		ExpirationTime: 1234567892,
		State:          types.ActionStateApproved,
		BlockHeight:    101,
		SuperNodes:     []string{"supernode-2"},
	}
	actionDifferentType := types.Action{
		Creator:        "creator-other-type",
		ActionID:       "999999",
		ActionType:     types.ActionTypeSense,
		Metadata:       []byte("metadata-other-type"),
		Price:          price.String(),
		ExpirationTime: 1234567893,
		State:          types.ActionStateApproved,
		BlockHeight:    102,
		SuperNodes:     []string{"supernode-3"},
	}

	require.NoError(t, k.SetAction(ctx, &actionLowLexical))
	require.NoError(t, k.SetAction(ctx, &actionHighNumeric))
	require.NoError(t, k.SetAction(ctx, &actionDifferentType))

	resp, err := q.ListActions(ctx, &types.QueryListActionsRequest{
		ActionType: types.ActionTypeCascade,
		Pagination: &query.PageRequest{
			Limit:   1,
			Reverse: true,
		},
	})
	require.NoError(t, err)
	require.Len(t, resp.Actions, 1)
	require.Equal(t, "123611", resp.Actions[0].ActionID)
}

func TestKeeper_ListActions_ReversePaginationInvalidCursorKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	k, ctx := keepertest.ActionKeeper(t, ctrl)
	q := keeper.NewQueryServerImpl(k)

	resp, err := q.ListActions(ctx, &types.QueryListActionsRequest{
		Pagination: &query.PageRequest{
			Key:     []byte("bad-key"),
			Reverse: true,
		},
	})
	require.Nil(t, resp)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

func TestKeeper_ListActions_ReversePaginationZeroLimitUsesDefaultLimit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	k, ctx := keepertest.ActionKeeper(t, ctrl)
	q := keeper.NewQueryServerImpl(k)
	price := sdk.NewInt64Coin("stake", 100)

	entries := query.DefaultLimit + 1
	for i := 1; i <= entries; i++ {
		id := fmt.Sprintf("%d", i)
		action := types.Action{
			Creator:        "creator-" + id,
			ActionID:       id,
			ActionType:     types.ActionTypeCascade,
			Metadata:       []byte("metadata-" + id),
			Price:          price.String(),
			ExpirationTime: int64(1234567890 + i),
			State:          types.ActionStateApproved,
			BlockHeight:    int64(200 + i),
			SuperNodes:     []string{"supernode-" + id},
		}
		require.NoError(t, k.SetAction(ctx, &action))
	}

	resp, err := q.ListActions(ctx, &types.QueryListActionsRequest{
		Pagination: &query.PageRequest{
			Limit:   0,
			Reverse: true,
		},
	})
	require.NoError(t, err)
	require.Len(t, resp.Actions, int(query.DefaultLimit))
	require.NotEmpty(t, resp.Pagination.NextKey)
}
