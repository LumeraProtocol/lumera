package keeper

import (
	"context"
	"strconv"

	"github.com/LumeraProtocol/lumera/x/action/v1/types"

	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ListActionsByBlockHeight returns all actions created at a specific block height
func (q queryServer) ListActionsByBlockHeight(goCtx context.Context, req *types.QueryListActionsByBlockHeightRequest) (*types.QueryListActionsByBlockHeightResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if req.BlockHeight < 0 {
		return nil, status.Error(codes.InvalidArgument, "block height must be non-negative")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	store := q.k.storeService.OpenKVStore(ctx)
	storeAdapter := runtime.KVStoreAdapter(store)
	var actions []*types.Action
	var pageRes *query.PageResponse
	var err error

	// Use block height index for efficient lookup
	heightPrefix := []byte(ActionByBlockHeightPrefix + strconv.FormatInt(req.BlockHeight, 10) + "/")
	indexStore := prefix.NewStore(storeAdapter, heightPrefix)

	onResult := func(key, _ []byte, accumulate bool) (bool, error) {
		actionID := string(key)
		act, found := q.k.GetActionByID(ctx, actionID)
		if !found {
			// Stale index entry; skip
			return false, nil
		}

		// Sanity check to guard against any malformed index entries
		if act.BlockHeight != req.BlockHeight {
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

	return &types.QueryListActionsByBlockHeightResponse{
		Actions:    actions,
		Pagination: pageRes,
		Total:      pageRes.GetTotal(),
	}, nil
}
