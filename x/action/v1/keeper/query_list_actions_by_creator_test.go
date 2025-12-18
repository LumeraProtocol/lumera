package keeper_test

import (
	"testing"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	"github.com/LumeraProtocol/lumera/testutil/sample"
	"github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/action/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestKeeper_ListActionsByCreator(t *testing.T) {
	creator := sample.AccAddress()
	otherCreator := sample.AccAddress()
	price := sdk.NewInt64Coin("ulume", 100)

	action1 := types.Action{
		Creator:        creator,
		ActionID:       "1",
		ActionType:     types.ActionTypeSense,
		Metadata:       []byte("metadata1"),
		Price:          price.String(),
		ExpirationTime: 1234567890,
		State:          types.ActionStatePending,
		BlockHeight:    10,
		SuperNodes:     []string{"sn1"},
	}
	action2 := types.Action{
		Creator:        creator,
		ActionID:       "2",
		ActionType:     types.ActionTypeCascade,
		Metadata:       []byte("metadata2"),
		Price:          price.String(),
		ExpirationTime: 1234567891,
		State:          types.ActionStateApproved,
		BlockHeight:    11,
		SuperNodes:     []string{"sn2"},
	}
	actionOther := types.Action{
		Creator:        otherCreator,
		ActionID:       "3",
		ActionType:     types.ActionTypeCascade,
		Metadata:       []byte("metadata3"),
		Price:          price.String(),
		ExpirationTime: 1234567892,
		State:          types.ActionStateDone,
		BlockHeight:    12,
		SuperNodes:     []string{"sn3"},
	}

	testCases := []struct {
		name        string
		req         *types.QueryListActionsByCreatorRequest
		setupState  func(k keeper.Keeper, ctx sdk.Context)
		expectedErr error
		checkResult func(t *testing.T, resp *types.QueryListActionsByCreatorResponse)
	}{
		{
			name:        "invalid request (nil)",
			req:         nil,
			expectedErr: status.Error(codes.InvalidArgument, "creator address must be provided"),
		},
		{
			name: "creator empty",
			req: &types.QueryListActionsByCreatorRequest{
				Creator: "",
			},
			expectedErr: status.Error(codes.InvalidArgument, "creator address must be provided"),
			},
			{
				name: "invalid creator address format",
				req: &types.QueryListActionsByCreatorRequest{
					Creator: "invalid-address",
				},
				expectedErr: status.Error(codes.InvalidArgument, "invalid creator address"),
			},
			{
				name: "no actions for creator",
				req: &types.QueryListActionsByCreatorRequest{
					Creator: creator,
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				// store only actions for other creator
				require.NoError(t, k.SetAction(ctx, &actionOther))
			},
			checkResult: func(t *testing.T, resp *types.QueryListActionsByCreatorResponse) {
				require.NotNil(t, resp)
				require.Len(t, resp.Actions, 0)
			},
		},
		{
			name: "actions found for creator",
			req: &types.QueryListActionsByCreatorRequest{
				Creator: creator,
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				require.NoError(t, k.SetAction(ctx, &action1))
				require.NoError(t, k.SetAction(ctx, &action2))
				require.NoError(t, k.SetAction(ctx, &actionOther))
			},
			checkResult: func(t *testing.T, resp *types.QueryListActionsByCreatorResponse) {
				require.NotNil(t, resp)
				require.Len(t, resp.Actions, 2)
				require.Equal(t, action1.ActionID, resp.Actions[0].ActionID)
				require.Equal(t, action2.ActionID, resp.Actions[1].ActionID)
			},
		},
		{
			name: "pagination works correctly",
			req: &types.QueryListActionsByCreatorRequest{
				Creator: creator,
				Pagination: &query.PageRequest{
					Offset: 1,
					Limit:  1,
				},
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				require.NoError(t, k.SetAction(ctx, &action1))
				require.NoError(t, k.SetAction(ctx, &action2))
			},
			checkResult: func(t *testing.T, resp *types.QueryListActionsByCreatorResponse) {
				require.NotNil(t, resp)
				require.Len(t, resp.Actions, 1)
				require.Equal(t, action2.ActionID, resp.Actions[0].ActionID)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			k, ctx := keepertest.ActionKeeper(t, ctrl)
			q := keeper.NewQueryServerImpl(k)

			if tc.setupState != nil {
				tc.setupState(k, ctx)
			}

			resp, err := q.ListActionsByCreator(ctx, tc.req)

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
