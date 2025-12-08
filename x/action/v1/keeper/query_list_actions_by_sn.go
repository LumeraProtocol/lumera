package keeper

import (
	"context"

	"github.com/LumeraProtocol/lumera/x/action/v1/types"

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
	var actions []*types.Action
	var pageRes *query.PageResponse
	var err error

	// Use supernode index for efficient lookup
	indexPrefix := []byte(ActionBySuperNodePrefix + req.SuperNodeAddress + "/")
	indexStore := prefix.NewStore(storeAdapter, indexPrefix)

	onResult := func(key, _ []byte, accumulate bool) (bool, error) {
		actionID := string(key)
		act, found := q.k.GetActionByID(ctx, actionID)
		if !found {
			// Stale index entry; skip without counting
			return false, nil
		}

		if accumulate {
			actions = append(actions, act)
		}

		return true, nil
	}

	pageRes, err = query.FilteredPaginate(indexStore, req.Pagination, onResult)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to paginate actions: %v", err)
	}

	return &types.QueryListActionsBySuperNodeResponse{
		Actions:    actions,
		Pagination: pageRes,
		Total:      pageRes.GetTotal(),
	}, nil
}
