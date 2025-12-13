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

		if !slices.Contains(act.SuperNodes, req.SuperNodeAddress) {
			// Skip actions not associated with the requested supernode
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

	return &types.QueryListActionsBySuperNodeResponse{
		Actions:    actions,
		Pagination: pageRes,
		Total:      pageRes.GetTotal(),
	}, nil
}
