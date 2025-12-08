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

// ListActionsByCreator returns all actions created by a specific address.
func (q queryServer) ListActionsByCreator(
	goCtx context.Context,
	req *types.QueryListActionsByCreatorRequest,
) (*types.QueryListActionsByCreatorResponse, error) {
	if req == nil || req.Creator == "" {
		return nil, status.Error(codes.InvalidArgument, "creator address must be provided")
	}

	// Validate creator address format early to provide clear feedback
	if _, err := q.k.addressCodec.StringToBytes(req.Creator); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid creator address: %s", err)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	store := q.k.storeService.OpenKVStore(ctx)
	storeAdapter := runtime.KVStoreAdapter(store)

	// Index store: keys are action IDs for this creator, values are markers.
	indexPrefix := []byte(ActionByCreatorPrefix + req.Creator + "/")
	indexStore := prefix.NewStore(storeAdapter, indexPrefix)

	var actions []*types.Action

	onResult := func(key, _ []byte, accumulate bool) (bool, error) {
		actionID := string(key)

	act, found := q.k.GetActionByID(ctx, actionID)
	if !found {
		// If index is stale or corrupted, skip this entry but continue scanning
		return false, nil
	}

	if accumulate {
		actions = append(actions, act)
	}

	return true, nil
}

	pageRes, err := query.FilteredPaginate(indexStore, req.Pagination, onResult)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to paginate actions: %v", err)
	}

	return &types.QueryListActionsByCreatorResponse{
		Actions:    actions,
		Pagination: pageRes,
		Total:      pageRes.GetTotal(),
	}, nil
}
