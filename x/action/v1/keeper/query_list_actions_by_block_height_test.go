package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/action/v1/types"

	actionapi "github.com/LumeraProtocol/lumera/api/lumera/action"
	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestKeeper_ListActionsByBlockHeight(t *testing.T) {
	actionID1 := "12345"
	actionID2 := "67890"
	blockHeight := int64(100)
	anotherBlockHeight := int64(200)
	invalidBlockHeight := int64(-1)
	creatorAddr := sdk.AccAddress([]byte("creator"))
	price := "100stake"
	action1 := actionapi.Action{
		Creator:        creatorAddr.String(),
		ActionID:       actionID1,
		ActionType:     actionapi.ActionType_ACTION_TYPE_SENSE,
		Metadata:       []byte("metadata1"),
		Price:          price,
		ExpirationTime: 1234567890,
		State:          actionapi.ActionState_ACTION_STATE_PROCESSING,
		BlockHeight:    blockHeight,
		SuperNodes:     []string{"node1", "node2"},
	}
	action2 := actionapi.Action{
		Creator:        creatorAddr.String(),
		ActionID:       actionID2,
		ActionType:     actionapi.ActionType_ACTION_TYPE_CASCADE,
		Metadata:       []byte("metadata2"),
		Price:          price,
		ExpirationTime: 1234567891,
		State:          actionapi.ActionState_ACTION_STATE_DONE,
		BlockHeight:    blockHeight,
		SuperNodes:     []string{"node3", "node4"},
	}
	action3 := actionapi.Action{
		Creator:        creatorAddr.String(),
		ActionID:       "11111",
		ActionType:     actionapi.ActionType_ACTION_TYPE_SENSE,
		Metadata:       []byte("metadata3"),
		Price:          price,
		ExpirationTime: 1234567892,
		State:          actionapi.ActionState_ACTION_STATE_PENDING,
		BlockHeight:    anotherBlockHeight,
		SuperNodes:     []string{"node5", "node6"},
	}

	testCases := []struct {
		name        string
		req         *types.QueryListActionsByBlockHeightRequest
		setupState  func(k keeper.Keeper, ctx sdk.Context)
		expectedErr error
		checkResult func(t *testing.T, resp *types.QueryListActionsResponse)
	}{
		{
			name:        "invalid request (nil)",
			req:         nil,
			expectedErr: status.Error(codes.InvalidArgument, "invalid request"),
		},
		{
			name: "invalid block height",
			req: &types.QueryListActionsByBlockHeightRequest{
				BlockHeight: invalidBlockHeight,
			},
			expectedErr: status.Error(codes.InvalidArgument, "block height must be non-negative"),
		},
		{
			name: "actions not found for block height",
			req: &types.QueryListActionsByBlockHeightRequest{
				BlockHeight: 9999,
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types.QueryListActionsResponse) {
				require.NotNil(t, resp)
				require.Len(t, resp.Actions, 0)
			},
		},
		{
			name: "actions found for block height",
			req: &types.QueryListActionsByBlockHeightRequest{
				BlockHeight: blockHeight,
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				k.SetAction(ctx, &action1)
				k.SetAction(ctx, &action2)
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types.QueryListActionsResponse) {
				require.NotNil(t, resp)
				require.Len(t, resp.Actions, 2)
				require.Equal(t, actionID1, resp.Actions[0].ActionID)
				require.Equal(t, actionID2, resp.Actions[1].ActionID)
			},
		},
		{
			name: "incorrect block height filtering (should not return actions from different block)",
			req: &types.QueryListActionsByBlockHeightRequest{
				BlockHeight: blockHeight,
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

				for _, act := range resp.Actions {
					require.NotEqual(t, action3.ActionID, act.ActionID, "Action from different block height should not be included")
				}
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			k, ctx := keepertest.ActionKeeper(t)

			if tc.setupState != nil {
				tc.setupState(k, ctx)
			}

			resp, err := k.ListActionsByBlockHeight(ctx, tc.req)

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
