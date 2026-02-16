package keeper

import (
	"context"
	"sort"
	"strconv"

	"github.com/LumeraProtocol/lumera/x/action/v1/types"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"

	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func shouldUseNumericReverseOrdering(pageReq *query.PageRequest) bool {
	return pageReq != nil && pageReq.Reverse && len(pageReq.Key) == 0
}

func parseNumericActionID(actionID string) (uint64, bool) {
	parsed, err := strconv.ParseUint(actionID, 10, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func sortActionsByNumericID(actions []*types.Action) {
	sort.SliceStable(actions, func(i, j int) bool {
		leftNumericID, leftIsNumeric := parseNumericActionID(actions[i].ActionID)
		rightNumericID, rightIsNumeric := parseNumericActionID(actions[j].ActionID)

		switch {
		case leftIsNumeric && rightIsNumeric:
			if leftNumericID == rightNumericID {
				return actions[i].ActionID < actions[j].ActionID
			}
			return leftNumericID < rightNumericID
		case leftIsNumeric != rightIsNumeric:
			return leftIsNumeric
		default:
			return actions[i].ActionID < actions[j].ActionID
		}
	})
}

func paginateActionSlice(actions []*types.Action, pageReq *query.PageRequest) ([]*types.Action, *query.PageResponse) {
	if pageReq == nil {
		return actions, &query.PageResponse{}
	}

	total := uint64(len(actions))
	offset := pageReq.Offset
	if offset > total {
		offset = total
	}

	limit := pageReq.Limit
	if limit == 0 || offset+limit > total {
		limit = total - offset
	}

	end := offset + limit
	page := actions[int(offset):int(end)]

	pageRes := &query.PageResponse{}
	if pageReq.CountTotal {
		pageRes.Total = total
	}

	return page, pageRes
}

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
	useTypeIndex := !useStateIndex && req.ActionType != types.ActionTypeUnspecified

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
	} else if useTypeIndex {
		// When filtering only by type, use the type index
		typePrefix := []byte(ActionByTypePrefix + types.ActionType(req.ActionType).String() + "/")
		indexStore := prefix.NewStore(storeAdapter, typePrefix)

		onResult := func(key, _ []byte, accumulate bool) (bool, error) {
			actionID := string(key)
			act, found := q.k.GetActionByID(ctx, actionID)
			if !found {
				// Stale index entry; skip
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

		if shouldUseNumericReverseOrdering(req.Pagination) {
			iter := actionStore.Iterator(nil, nil)
			defer iter.Close()

			for ; iter.Valid(); iter.Next() {
				var act actiontypes.Action
				if unmarshalErr := q.k.cdc.Unmarshal(iter.Value(), &act); unmarshalErr != nil {
					return nil, status.Errorf(codes.Internal, "failed to unmarshal action: %v", unmarshalErr)
				}
				actions = append(actions, &act)
			}

			sortActionsByNumericID(actions)
			for i, j := 0, len(actions)-1; i < j; i, j = i+1, j-1 {
				actions[i], actions[j] = actions[j], actions[i]
			}

			actions, pageRes = paginateActionSlice(actions, req.Pagination)
		} else {
			onResult := func(key, value []byte, accumulate bool) (bool, error) {
				var act actiontypes.Action
				if err := q.k.cdc.Unmarshal(value, &act); err != nil {
					return false, err
				}

				if accumulate {
					actions = append(actions, &act)
				}

				return true, nil
			}
			pageRes, err = query.FilteredPaginate(actionStore, req.Pagination, onResult)
		}
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
