package keeper_test

import (
	"testing"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	"github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/action/v1/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	gogoproto "github.com/cosmos/gogoproto/proto"
	"go.uber.org/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestQueryActionByMetadata(t *testing.T) {
	actionID1 := "12345"
	actionID2 := "67890"
	actionID3 := "67891"
	actionID4 := "67892"
	price := sdk.NewInt64Coin("stake", 100)

	senseMetadata1 := &types.SenseMetadata{
		CollectionId: "collection1",
		GroupId:      "group1",
		DataHash:     "hash1",
	}
	senseMetadataBytes1, err := gogoproto.Marshal(senseMetadata1)
	require.NoError(t, err)

	senseMetadata2 := &types.SenseMetadata{
		CollectionId: "collection2",
		GroupId:      "group2",
		DataHash:     "hash2",
	}
	senseMetadataBytes2, err := gogoproto.Marshal(senseMetadata2)
	require.NoError(t, err)

	cascadeMetadata3 := &types.CascadeMetadata{
		FileName: "file1",
		DataHash: "hash1",
	}
	cascadeMetadataBytes3, err := gogoproto.Marshal(cascadeMetadata3)
	require.NoError(t, err)

	senseMetadata4 := &types.SenseMetadata{
		CollectionId: "collection1",
		GroupId:      "group1",
		DataHash:     "hash1",
	}
	senseMetadataBytes4, err := gogoproto.Marshal(senseMetadata4)
	require.NoError(t, err)

	action1 := types.Action{
		Creator:        "creator1",
		ActionID:       actionID1,
		ActionType:     types.ActionTypeSense,
		Metadata:       senseMetadataBytes1,
		Price:          price.String(),
		ExpirationTime: 1234567890,
		State:          types.ActionStateProcessing,
		BlockHeight:    100,
		SuperNodes:     []string{"supernode-1", "supernode-2"},
		AppPubkey:      []byte{1, 2, 3},
	}
	action2 := types.Action{
		Creator:        "creator2",
		ActionID:       actionID2,
		ActionType:     types.ActionTypeSense,
		Metadata:       senseMetadataBytes2,
		Price:          price.String(),
		ExpirationTime: 1234567891,
		State:          types.ActionStateApproved,
		BlockHeight:    100,
		SuperNodes:     []string{"supernode-1", "supernode-2"},
	}
	action3 := types.Action{
		Creator:        "creator3",
		ActionID:       actionID3,
		ActionType:     types.ActionTypeCascade,
		Metadata:       cascadeMetadataBytes3,
		Price:          price.String(),
		ExpirationTime: 1234567892,
		State:          types.ActionStateApproved,
		BlockHeight:    100,
		SuperNodes:     []string{"supernode-3"},
	}
	action4 := types.Action{
		Creator:        "creator1",
		ActionID:       actionID4,
		ActionType:     types.ActionTypeSense,
		Metadata:       senseMetadataBytes4,
		Price:          price.String(),
		ExpirationTime: 1234567890,
		State:          types.ActionStateProcessing,
		BlockHeight:    100,
		SuperNodes:     []string{"supernode-1", "supernode-2"},
	}

	testCases := []struct {
		name        string
		req         *types.QueryActionByMetadataRequest
		setupState  func(k keeper.Keeper, ctx sdk.Context)
		expectedErr error
		checkResult func(t *testing.T, resp *types.QueryActionByMetadataResponse)
	}{
		{
			name:        "invalid request (nil request)",
			req:         nil,
			expectedErr: status.Error(codes.InvalidArgument, "invalid request"),
		},
		{
			name:        "missing required parameters (ActionType and MetadataQuery)",
			req:         &types.QueryActionByMetadataRequest{},
			expectedErr: status.Error(codes.InvalidArgument, "action type and metadata query required"),
		},
		{
			name: "invalid metadata query format",
			req: &types.QueryActionByMetadataRequest{
				ActionType:    types.ActionTypeSense,
				MetadataQuery: "collection_id",
			},
			expectedErr: status.Error(codes.InvalidArgument, "invalid metadata query format, expected 'field=value'"),
		},
		{
			name: "actions found for valid metadata query",
			req: &types.QueryActionByMetadataRequest{
				ActionType:    types.ActionTypeSense,
				MetadataQuery: "collection_id=collection1",
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				k.SetAction(ctx, &action1)
				k.SetAction(ctx, &action2)
				k.SetAction(ctx, &action3)
				k.SetAction(ctx, &action4)
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types.QueryActionByMetadataResponse) {
				require.NotNil(t, resp)
				require.Len(t, resp.Actions, 2)
				require.Equal(t, actionID1, resp.Actions[0].ActionID)
				require.Equal(t, action1.AppPubkey, resp.Actions[0].AppPubkey)
			},
		},
		{
			name: "no actions found for non-matching metadata query",
			req: &types.QueryActionByMetadataRequest{
				ActionType:    types.ActionTypeSense,
				MetadataQuery: "collection_id=collection3",
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				k.SetAction(ctx, &action1)
				k.SetAction(ctx, &action2)
				k.SetAction(ctx, &action3)
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types.QueryActionByMetadataResponse) {
				require.NotNil(t, resp)
				require.Len(t, resp.Actions, 0)
			},
		},
		{
			name: "pagination works correctly",
			req: &types.QueryActionByMetadataRequest{
				ActionType:    types.ActionTypeSense,
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
			checkResult: func(t *testing.T, resp *types.QueryActionByMetadataResponse) {
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

			k, ctx := keepertest.ActionKeeper(t, ctrl)
			q := keeper.NewQueryServerImpl(k)

			if tc.setupState != nil {
				tc.setupState(k, ctx)
			}

			resp, err := q.QueryActionByMetadata(ctx, tc.req)

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
