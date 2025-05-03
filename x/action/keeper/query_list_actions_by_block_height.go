package keeper

import (
	"context"

	actionapi "github.com/LumeraProtocol/lumera/api/lumera/action"
	"github.com/LumeraProtocol/lumera/x/action/types"

	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ListActionsByBlockHeight returns all actions created at a specific block height
func (k Keeper) ListActionsByBlockHeight(goCtx context.Context, req *types.QueryListActionsByBlockHeightRequest) (*types.QueryListActionsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if req.BlockHeight < 0 {
		return nil, status.Error(codes.InvalidArgument, "block height must be non-negative")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	store := k.storeService.OpenKVStore(ctx)
	storeAdapter := runtime.KVStoreAdapter(store)
	actionStore := prefix.NewStore(storeAdapter, []byte(ActionKeyPrefix))

	var actions []*types.Action

	onResult := func(key, value []byte, accumulate bool) (bool, error) {
		var act actionapi.Action
		if err := k.cdc.Unmarshal(value, &act); err != nil {
			return false, err
		}

		if act.BlockHeight == req.BlockHeight && accumulate {
			price, err := sdk.ParseCoinNormalized(act.Price)
			if err != nil {
				k.Logger().Error("failed to parse price", "action_id", act.ActionID, "price", act.Price, "error", err)
				return false, err
			}

			actions = append(actions, &types.Action{
				Creator:        act.Creator,
				ActionID:       act.ActionID,
				ActionType:     types.ActionType(act.ActionType),
				Metadata:       act.Metadata,
				Price:          &price,
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

	return &types.QueryListActionsResponse{
		Actions:    actions,
		Pagination: pageRes,
	}, nil
}
