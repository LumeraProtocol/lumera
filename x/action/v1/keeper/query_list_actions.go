package keeper

import (
	"context"

	"github.com/LumeraProtocol/lumera/x/action/v1/types"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"

	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ListActions returns a list of actions, optionally filtered by type and state
func (q queryServer) ListActions(goCtx context.Context, req *types.QueryListActionsRequest) (*types.QueryListActionsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	store := q.k.storeService.OpenKVStore(ctx)
	storeAdapter := runtime.KVStoreAdapter(store)
	actionStore := prefix.NewStore(storeAdapter, []byte(ActionKeyPrefix))

	var actions []*types.Action

	onResult := func(key, value []byte, accumulate bool) (bool, error) {
		var act actiontypes.Action
		if err := q.k.cdc.Unmarshal(value, &act); err != nil {
			return false, err
		}

		if req.ActionState != types.ActionStateUnspecified && act.State != actiontypes.ActionState(req.ActionState) {
			return false, nil
		}

		if req.ActionType != types.ActionTypeUnspecified && act.ActionType != actiontypes.ActionType(req.ActionType) {
			return false, nil
		}

		if accumulate {
			actions = append(actions, &types.Action{
				Creator:        act.Creator,
				ActionID:       act.ActionID,
				ActionType:     types.ActionType(act.ActionType),
				Metadata:       act.Metadata,
				Price:          act.Price,
				ExpirationTime: act.ExpirationTime,
				State:          types.ActionState(act.State),
				BlockHeight:    act.BlockHeight,
				SuperNodes:     act.SuperNodes,
				FileSizeKbs:    act.FileSizeKbs,
			})
		}

		return true, nil
	}

	pageRes, err := query.FilteredPaginate(actionStore, req.Pagination, onResult)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to paginate actions: %v", err)
	}

	return &types.QueryListActionsResponse{
		Actions:    actions,
		Pagination: pageRes,
	}, nil
}
