package keeper_test

import (
	"testing"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	"github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/action/v1/types"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"

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
	price := sdk.NewInt64Coin("stake", 100)
	action1 := actiontypes.Action{
		Creator:        creatorAddr.String(),
		ActionID:       actionID1,
		ActionType:     actiontypes.ActionTypeSense,
		Metadata:       []byte("metadata1"),
		Price:          price.String(),
		ExpirationTime: 1234567890,
		State:          actiontypes.ActionStateProcessing,
		BlockHeight:    blockHeight,
		SuperNodes:     []string{"node1", "node2"},
	}
	action2 := actiontypes.Action{
		Creator:        creatorAddr.String(),
		ActionID:       actionID2,
		ActionType:     types.ActionTypeCascade,
		Metadata:       []byte("metadata2"),
		Price:          price.String(),
		ExpirationTime: 1234567891,
		State:          actiontypes.ActionStateDone,
		BlockHeight:    blockHeight,
		SuperNodes:     []string{"node3", "node4"},
	}
	action3 := actiontypes.Action{
		Creator:        creatorAddr.String(),
		ActionID:       "11111",
		ActionType:     actiontypes.ActionTypeSense,
		Metadata:       []byte("metadata3"),
		Price:          price.String(),
		ExpirationTime: 1234567892,
		State:          actiontypes.ActionStatePending,
		BlockHeight:    anotherBlockHeight,
		SuperNodes:     []string{"node5", "node6"},
	}

	testCases := []struct {
		name        string
		req         *types.QueryListActionsByBlockHeightRequest
		setupState  func(k keeper.Keeper, ctx sdk.Context)
		expectedErr error
		checkResult func(t *testing.T, resp *types.QueryListActionsByBlockHeightResponse)
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
			checkResult: func(t *testing.T, resp *types.QueryListActionsByBlockHeightResponse) {
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
			checkResult: func(t *testing.T, resp *types.QueryListActionsByBlockHeightResponse) {
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
			checkResult: func(t *testing.T, resp *types.QueryListActionsByBlockHeightResponse) {
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

			k, ctx := keepertest.ActionKeeper(t, ctrl)
			q := keeper.NewQueryServerImpl(k)

			if tc.setupState != nil {
				tc.setupState(k, ctx)
			}

			resp, err := q.ListActionsByBlockHeight(ctx, tc.req)

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
