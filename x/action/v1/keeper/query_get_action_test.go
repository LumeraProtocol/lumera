package keeper_test

import (
	"testing"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	"github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/action/v1/types"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"go.uber.org/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestKeeper_GetAction(t *testing.T) {
	actionID := "12345"
	invalidActionID := "67890"
	creatorAddr := sdk.AccAddress([]byte("creator"))
	price := sdk.NewInt64Coin("stake", 100)
	action := actiontypes.Action{
		Creator:        creatorAddr.String(),
		ActionID:       actionID,
		ActionType:     actiontypes.ActionTypeSense,
		Metadata:       []byte("metadata"),
		Price:          price.String(),
		ExpirationTime: 1234567890,
		State:          actiontypes.ActionStateProcessing,
		BlockHeight:    1,
		SuperNodes:     []string{"node1", "node2"},
		FileSizeKbs:    123,
		AppPubkey:      []byte{1, 2, 3},
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
			expectedErr: status.Errorf(codes.NotFound, "failed to get action by ID"),
		},
		{
			name: "action found",
			req: &types.QueryGetActionRequest{
				ActionID: actionID,
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				action.Price = price.String()
				k.SetAction(ctx, &action)
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types.QueryGetActionResponse) {
				require.NotNil(t, resp.Action)
				require.Equal(t, action.ActionID, resp.Action.ActionID)
				require.Equal(t, action.Creator, resp.Action.Creator)
				require.Equal(t, action.Price, resp.Action.Price)
				require.Equal(t, action.FileSizeKbs, resp.Action.FileSizeKbs)
				require.Equal(t, action.AppPubkey, resp.Action.AppPubkey)
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

			resp, err := q.GetAction(ctx, tc.req)

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
