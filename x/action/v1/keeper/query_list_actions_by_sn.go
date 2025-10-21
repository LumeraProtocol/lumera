package keeper

import (
	"context"
	"slices"

	"github.com/LumeraProtocol/lumera/x/action/v1/types"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"

	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ListActionsBySuperNode returns all actions associated with a specific supernode
func (q queryServer) ListActionsBySuperNode(goCtx context.Context, req *types.QueryListActionsBySuperNodeRequest) (*types.QueryListActionsBySuperNodeResponse, error) {
	if req == nil || req.SuperNodeAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "supernode address must be provided")
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

		if slices.Contains(act.SuperNodes, req.SuperNodeAddress) && accumulate {
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
			})
		}

		return true, nil
	}

	pageRes, err := query.FilteredPaginate(actionStore, req.Pagination, onResult)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to paginate actions: %v", err)
	}

	return &types.QueryListActionsBySuperNodeResponse{
		Actions:    actions,
		Pagination: pageRes,
	}, nil
}
