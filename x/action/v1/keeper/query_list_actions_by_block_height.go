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
	var (
		actions []*types.Action
		pageRes *query.PageResponse
		err     error
	)

	actionStore := prefix.NewStore(storeAdapter, []byte(ActionKeyPrefix))

	onResult := func(key, value []byte, accumulate bool) (bool, error) {
		var act actiontypes.Action
		if err := q.k.cdc.Unmarshal(value, &act); err != nil {
			return false, err
		}

		if act.BlockHeight != req.BlockHeight {
			// Skip non-matching heights without counting toward pagination/total
			return false, nil
		}

		if accumulate {
			actCopy := act
			actions = append(actions, &actCopy)
		}

		return true, nil
	}

	pageRes, err = query.FilteredPaginate(actionStore, req.Pagination, onResult)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to paginate actions: %v", err)
	}

	return &types.QueryListActionsByBlockHeightResponse{
		Actions:    actions,
		Pagination: pageRes,
		Total:      pageRes.GetTotal(),
	}, nil
}
