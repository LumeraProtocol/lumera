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

	var actions []*types.Action
	useStateIndex := req.ActionState != types.ActionStateUnspecified

	var pageRes *query.PageResponse
	var err error

	if useStateIndex {
		// When filtering by state, use the state index to avoid full scans
		statePrefix := []byte(ActionByStatePrefix + types.ActionState(req.ActionState).String() + "/")
		indexStore := prefix.NewStore(storeAdapter, statePrefix)

		onResult := func(key, _ []byte, accumulate bool) (bool, error) {
			actionID := string(key)
			act, found := q.k.GetActionByID(ctx, actionID)
			if !found {
				// Stale index entry; skip without counting
				return false, nil
			}

			if req.ActionType != types.ActionTypeUnspecified && act.ActionType != actiontypes.ActionType(req.ActionType) {
				return false, nil
			}

			if accumulate {
				actions = append(actions, act)
			}

			return true, nil
		}

		pageRes, err = query.FilteredPaginate(indexStore, req.Pagination, onResult)
	} else {
		actionStore := prefix.NewStore(storeAdapter, []byte(ActionKeyPrefix))

		onResult := func(key, value []byte, accumulate bool) (bool, error) {
			var act actiontypes.Action
			if err := q.k.cdc.Unmarshal(value, &act); err != nil {
				return false, err
			}

			if req.ActionType != types.ActionTypeUnspecified && act.ActionType != actiontypes.ActionType(req.ActionType) {
				return false, nil
			}

			if accumulate {
				actions = append(actions, &act)
			}

			return true, nil
		}
		pageRes, err = query.FilteredPaginate(actionStore, req.Pagination, onResult)
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to paginate actions: %v", err)
	}

	return &types.QueryListActionsResponse{
		Actions:    actions,
		Pagination: pageRes,
		Total:      pageRes.GetTotal(),
	}, nil
}
