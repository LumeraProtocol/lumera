package keeper

import (
	"context"
	types2 "github.com/LumeraProtocol/lumera/x/action/v1/types"

	"cosmossdk.io/store/prefix"
	actionapi "github.com/LumeraProtocol/lumera/api/lumera/action"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ListActions returns a list of actions, optionally filtered by type and state
func (k Keeper) ListActions(goCtx context.Context, req *types2.QueryListActionsRequest) (*types2.QueryListActionsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	store := k.storeService.OpenKVStore(ctx)
	storeAdapter := runtime.KVStoreAdapter(store)
	actionStore := prefix.NewStore(storeAdapter, []byte(ActionKeyPrefix))

	var actions []*types2.Action

	onResult := func(key, value []byte, accumulate bool) (bool, error) {
		var act actionapi.Action
		if err := k.cdc.Unmarshal(value, &act); err != nil {
			return false, err
		}

		if req.ActionState != types2.ActionStateUnspecified && act.State != actionapi.ActionState(req.ActionState) {
			return false, nil
		}

		if req.ActionType != types2.ActionTypeUnspecified && act.ActionType != actionapi.ActionType(req.ActionType) {
			return false, nil
		}

		if accumulate {
			price, err := sdk.ParseCoinNormalized(act.Price)
			if err != nil {
				k.Logger().Error("failed to parse price", "action_id", act.ActionID, "price", act.Price, "error", err)
				return false, err
			}

			actions = append(actions, &types2.Action{
				Creator:        act.Creator,
				ActionID:       act.ActionID,
				ActionType:     types2.ActionType(act.ActionType),
				Metadata:       act.Metadata,
				Price:          &price,
				ExpirationTime: act.ExpirationTime,
				State:          types2.ActionState(act.State),
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

	return &types2.QueryListActionsResponse{
		Actions:    actions,
		Pagination: pageRes,
	}, nil
}
