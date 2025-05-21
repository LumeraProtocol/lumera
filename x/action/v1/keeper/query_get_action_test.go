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

func TestKeeper_GetAction(t *testing.T) {
	actionID := "12345"
	invalidActionID := "67890"
	creatorAddr := sdk.AccAddress([]byte("creator"))
	price := "100stake"
	action := actionapi.Action{
		Creator:        creatorAddr.String(),
		ActionID:       actionID,
		ActionType:     actionapi.ActionType_ACTION_TYPE_SENSE,
		Metadata:       []byte("metadata"),
		Price:          price,
		ExpirationTime: 1234567890,
		State:          actionapi.ActionState_ACTION_STATE_PROCESSING,
		BlockHeight:    1,
		SuperNodes:     []string{"node1", "node2"},
	}

	testCases := []struct {
		name        string
		req         *types.QueryGetActionRequest
		setupState  func(k keeper.Keeper, ctx sdk.Context)
		expectedErr error
		checkResult func(t *testing.T, resp *types.QueryGetActionResponse)
	}{
		{
			name:        "invalid request (nil)",
			req:         nil,
			expectedErr: status.Error(codes.InvalidArgument, "invalid request"),
		},
		{
			name: "action not found",
			req: &types.QueryGetActionRequest{
				ActionID: invalidActionID,
			},
			expectedErr: status.Errorf(codes.Internal, "failed to get action by ID"),
		},
		{
			name: "invalid price format",
			req: &types.QueryGetActionRequest{
				ActionID: actionID,
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				action.Price = "invalid_price"
				k.SetAction(ctx, &action)
			},
			expectedErr: status.Errorf(codes.Internal, "invalid price"),
		},
		{
			name: "action found",
			req: &types.QueryGetActionRequest{
				ActionID: actionID,
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				action.Price = "100stake"
				k.SetAction(ctx, &action)
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types.QueryGetActionResponse) {
				require.NotNil(t, resp.Action)
				require.Equal(t, action.ActionID, resp.Action.ActionID)
				require.Equal(t, action.Creator, resp.Action.Creator)
				require.Equal(t, action.Price, resp.Action.Price.String())
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

			resp, err := k.GetAction(ctx, tc.req)

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
