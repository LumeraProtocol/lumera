package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	types2 "github.com/LumeraProtocol/lumera/x/action/v1/types"

	"github.com/LumeraProtocol/lumera/api/lumera/action"
	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	v1beta1 "cosmossdk.io/api/cosmos/base/v1beta1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func TestQueryActionByMetadata(t *testing.T) {
	actionID1 := "12345"
	actionID2 := "67890"
	actionID3 := "67891"
	actionID4 := "67892"
	price := &v1beta1.Coin{Denom: "stake", Amount: "100"}

	senseMetadata1 := &action.SenseMetadata{
		CollectionId: "collection1",
		GroupId:      "group1",
		DataHash:     "hash1",
	}
	senseMetadataBytes1, err := proto.Marshal(senseMetadata1)
	require.NoError(t, err)

	senseMetadata2 := &action.SenseMetadata{
		CollectionId: "collection2",
		GroupId:      "group2",
		DataHash:     "hash2",
	}
	senseMetadataBytes2, err := proto.Marshal(senseMetadata2)
	require.NoError(t, err)

	cascadeMetadata3 := &action.CascadeMetadata{
		FileName: "file1",
		DataHash: "hash1",
	}
	cascadeMetadataBytes3, err := proto.Marshal(cascadeMetadata3)
	require.NoError(t, err)

	senseMetadata4 := &action.SenseMetadata{
		CollectionId: "collection1",
		GroupId:      "group1",
		DataHash:     "hash1",
	}
	senseMetadataBytes4, err := proto.Marshal(senseMetadata4)
	require.NoError(t, err)

	action1 := action.Action{
		Creator:        "creator1",
		ActionID:       actionID1,
		ActionType:     action.ActionType_ACTION_TYPE_SENSE,
		Metadata:       senseMetadataBytes1,
		Price:          price,
		ExpirationTime: 1234567890,
		State:          action.ActionState_ACTION_STATE_PROCESSING,
		BlockHeight:    100,
		SuperNodes:     []string{"supernode-1", "supernode-2"},
	}
	action2 := action.Action{
		Creator:        "creator2",
		ActionID:       actionID2,
		ActionType:     action.ActionType_ACTION_TYPE_SENSE,
		Metadata:       senseMetadataBytes2,
		Price:          price,
		ExpirationTime: 1234567891,
		State:          action.ActionState_ACTION_STATE_APPROVED,
		BlockHeight:    100,
		SuperNodes:     []string{"supernode-1", "supernode-2"},
	}
	action3 := action.Action{
		Creator:        "creator3",
		ActionID:       actionID3,
		ActionType:     action.ActionType_ACTION_TYPE_CASCADE,
		Metadata:       cascadeMetadataBytes3,
		Price:          price,
		ExpirationTime: 1234567892,
		State:          action.ActionState_ACTION_STATE_APPROVED,
		BlockHeight:    100,
		SuperNodes:     []string{"supernode-3"},
	}
	action4 := action.Action{
		Creator:        "creator1",
		ActionID:       actionID4,
		ActionType:     action.ActionType_ACTION_TYPE_SENSE,
		Metadata:       senseMetadataBytes4,
		Price:          price,
		ExpirationTime: 1234567890,
		State:          action.ActionState_ACTION_STATE_PROCESSING,
		BlockHeight:    100,
		SuperNodes:     []string{"supernode-1", "supernode-2"},
	}

	testCases := []struct {
		name        string
		req         *types2.QueryActionByMetadataRequest
		setupState  func(k keeper.Keeper, ctx sdk.Context)
		expectedErr error
		checkResult func(t *testing.T, resp *types2.QueryListActionsResponse)
	}{
		{
			name:        "invalid request (nil request)",
			req:         nil,
			expectedErr: status.Error(codes.InvalidArgument, "invalid request"),
		},
		{
			name:        "missing required parameters (ActionType and MetadataQuery)",
			req:         &types2.QueryActionByMetadataRequest{},
			expectedErr: status.Error(codes.InvalidArgument, "action type and metadata query required"),
		},
		{
			name: "invalid metadata query format",
			req: &types2.QueryActionByMetadataRequest{
				ActionType:    types2.ActionTypeSense,
				MetadataQuery: "collection_id",
			},
			expectedErr: status.Error(codes.InvalidArgument, "invalid metadata query format, expected 'field=value'"),
		},
		{
			name: "actions found for valid metadata query",
			req: &types2.QueryActionByMetadataRequest{
				ActionType:    types2.ActionTypeSense,
				MetadataQuery: "collection_id=collection1",
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				k.SetAction(ctx, &action1)
				k.SetAction(ctx, &action2)
				k.SetAction(ctx, &action3)
				k.SetAction(ctx, &action4)
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types2.QueryListActionsResponse) {
				require.NotNil(t, resp)
				require.Len(t, resp.Actions, 2)
				require.Equal(t, actionID1, resp.Actions[0].ActionID)
			},
		},
		{
			name: "no actions found for non-matching metadata query",
			req: &types2.QueryActionByMetadataRequest{
				ActionType:    types2.ActionTypeSense,
				MetadataQuery: "collection_id=collection3",
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				k.SetAction(ctx, &action1)
				k.SetAction(ctx, &action2)
				k.SetAction(ctx, &action3)
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types2.QueryListActionsResponse) {
				require.NotNil(t, resp)
				require.Len(t, resp.Actions, 0)
			},
		},
		{
			name: "pagination works correctly",
			req: &types2.QueryActionByMetadataRequest{
				ActionType:    types2.ActionTypeSense,
				MetadataQuery: "collection_id=collection1",
				Pagination: &query.PageRequest{
					Offset: 1,
					Limit:  2,
				},
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				k.SetAction(ctx, &action1)
				k.SetAction(ctx, &action2)
				k.SetAction(ctx, &action3)
				k.SetAction(ctx, &action4)
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types2.QueryListActionsResponse) {
				require.NotNil(t, resp)
				require.Len(t, resp.Actions, 1)
				require.Equal(t, actionID4, resp.Actions[0].ActionID)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			k, ctx := keepertest.ActionKeeper(t)

			if tc.setupState != nil {
				tc.setupState(k, ctx)
			}

			resp, err := k.QueryActionByMetadata(ctx, tc.req)

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
