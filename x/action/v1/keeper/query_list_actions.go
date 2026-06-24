package keeper

import (
	"context"
	"encoding/binary"
	"fmt"
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

// shouldUseNumericReverseOrdering returns true when reverse pagination should use
// numeric action ID ordering instead of lexical store-key ordering.
func shouldUseNumericReverseOrdering(pageReq *query.PageRequest) bool {
	return pageReq != nil && pageReq.Reverse
}

// parseNumericActionID parses action IDs that are strictly base-10 uint64 values.
func parseNumericActionID(actionID string) (uint64, bool) {
	parsed, err := strconv.ParseUint(actionID, 10, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

// sortActionsByNumericID sorts actions by ActionID using numeric ordering when
// possible, falling back to lexical ordering for non-numeric IDs.
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

// applyNumericReverseOrderingAndPaginate applies numeric ActionID ordering in
// descending order and then paginates the resulting slice.
func applyNumericReverseOrderingAndPaginate(actions []*types.Action, pageReq *query.PageRequest) ([]*types.Action, *query.PageResponse, error) {
	sortActionsByNumericID(actions)
	for i, j := 0, len(actions)-1; i < j; i, j = i+1, j-1 {
		actions[i], actions[j] = actions[j], actions[i]
	}

	return paginateActionSlice(actions, pageReq)
}

// collectActionsFromIDIndexStore loads actions by ID from an index store whose keys
// are action IDs. Stale index entries are ignored.
func (q queryServer) collectActionsFromIDIndexStore(
	ctx sdk.Context,
	indexStore prefix.Store,
	actionTypeFilter types.ActionType,
) ([]*types.Action, error) {
	actions := make([]*types.Action, 0)
	iter := indexStore.Iterator(nil, nil)
	defer func() { _ = iter.Close() }()

	for ; iter.Valid(); iter.Next() {
		actionID := string(iter.Key())
		act, found := q.k.GetActionByID(ctx, actionID)
		if !found {
			continue
		}
		if actionTypeFilter != types.ActionTypeUnspecified && act.ActionType != actiontypes.ActionType(actionTypeFilter) {
			continue
		}

		actions = append(actions, act)
	}

	return actions, nil
}

// collectActionsFromPrimaryStore loads all actions from the primary action store.
func (q queryServer) collectActionsFromPrimaryStore(actionStore prefix.Store) ([]*types.Action, error) {
	actions := make([]*types.Action, 0)
	iter := actionStore.Iterator(nil, nil)
	defer func() { _ = iter.Close() }()

	for ; iter.Valid(); iter.Next() {
		var act actiontypes.Action
		if unmarshalErr := q.k.cdc.Unmarshal(iter.Value(), &act); unmarshalErr != nil {
			return nil, status.Errorf(codes.Internal, "failed to unmarshal action: %v", unmarshalErr)
		}
		actions = append(actions, &act)
	}

	return actions, nil
}

// decodeActionPaginationOffset decodes an opaque pagination key into an offset.
func decodeActionPaginationOffset(key []byte) (uint64, error) {
	if len(key) != 8 {
		return 0, fmt.Errorf("invalid key length %d", len(key))
	}
	return binary.BigEndian.Uint64(key), nil
}

// encodeActionPaginationOffset encodes an offset as an opaque pagination key.
func encodeActionPaginationOffset(offset uint64) []byte {
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, offset)
	return key
}

// paginateActionSlice paginates an already materialized action slice and returns
// a PageResponse compatible with cursor- and offset-based pagination.
func paginateActionSlice(actions []*types.Action, pageReq *query.PageRequest) ([]*types.Action, *query.PageResponse, error) {
	if pageReq == nil {
		return actions, &query.PageResponse{}, nil
	}
	if pageReq.Offset > 0 && pageReq.Key != nil {
		return nil, nil, status.Error(codes.InvalidArgument, "paginate: invalid request, either offset or key is expected, got both")
	}

	total := uint64(len(actions))
	offset := pageReq.Offset

	if len(pageReq.Key) > 0 {
		decodedOffset, err := decodeActionPaginationOffset(pageReq.Key)
		if err != nil {
			return nil, nil, status.Error(codes.InvalidArgument, "invalid pagination key")
		}
		offset = decodedOffset
	}

	if offset > total {
		offset = total
	}

	limit := pageReq.Limit
	if limit == 0 {
		limit = query.DefaultLimit
	}

	remaining := total - offset
	if limit > remaining {
		limit = remaining
	}

	end := offset + limit
	page := actions[int(offset):int(end)]

	pageRes := &query.PageResponse{}
	if pageReq.CountTotal {
		pageRes.Total = total
	}
	if end < total {
		pageRes.NextKey = encodeActionPaginationOffset(end)
	}

	return page, pageRes, nil
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

		if shouldUseNumericReverseOrdering(req.Pagination) {
			// Numeric reverse ordering cannot be derived from lexical KV iteration, so
			// we materialize the matched set, sort it numerically, then paginate.
			actions, err = q.collectActionsFromIDIndexStore(ctx, indexStore, req.ActionType)
			if err != nil {
				return nil, err
			}

			actions, pageRes, err = applyNumericReverseOrderingAndPaginate(actions, req.Pagination)
		} else {
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
		}
	} else if useTypeIndex {
		// When filtering only by type, use the type index
		typePrefix := []byte(ActionByTypePrefix + types.ActionType(req.ActionType).String() + "/")
		indexStore := prefix.NewStore(storeAdapter, typePrefix)

		if shouldUseNumericReverseOrdering(req.Pagination) {
			// Numeric reverse ordering cannot be derived from lexical KV iteration, so
			// we materialize the matched set, sort it numerically, then paginate.
			actions, err = q.collectActionsFromIDIndexStore(ctx, indexStore, types.ActionTypeUnspecified)
			if err != nil {
				return nil, err
			}

			actions, pageRes, err = applyNumericReverseOrderingAndPaginate(actions, req.Pagination)
		} else {
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
		}
	} else {
		actionStore := prefix.NewStore(storeAdapter, []byte(ActionKeyPrefix))

		if shouldUseNumericReverseOrdering(req.Pagination) {
			// Numeric reverse ordering cannot be derived from lexical KV iteration, so
			// we materialize the matched set, sort it numerically, then paginate.
			actions, err = q.collectActionsFromPrimaryStore(actionStore)
			if err != nil {
				return nil, err
			}

			actions, pageRes, err = applyNumericReverseOrderingAndPaginate(actions, req.Pagination)
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
		if st, ok := status.FromError(err); ok && st.Code() != codes.Unknown {
			return nil, err
		}
		return nil, status.Errorf(codes.Internal, "failed to paginate actions: %v", err)
	}

	return &types.QueryListActionsResponse{
		Actions:    actions,
		Pagination: pageRes,
		Total:      pageRes.GetTotal(),
	}, nil
}
