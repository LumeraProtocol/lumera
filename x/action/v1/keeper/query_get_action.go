package keeper

import (
    "context"

    types2 "github.com/LumeraProtocol/lumera/x/action/v1/types"

    "cosmossdk.io/math"
    sdk "github.com/cosmos/cosmos-sdk/types"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

// GetAction returns the action for the given action-id
func (k *Keeper) GetAction(goCtx context.Context, req *types2.QueryGetActionRequest) (*types2.QueryGetActionResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	action, ok := k.GetActionByID(ctx, req.ActionID)
	if !ok {
		return nil, status.Errorf(codes.Internal, "failed to get action by ID")
	}

    if action.Price == nil {
        k.Logger().Error("missing price", "action_id", action.ActionID)
        return nil, status.Errorf(codes.Internal, "invalid price")
    }
    amount, ok := math.NewIntFromString(action.Price.Amount)
    if !ok {
        k.Logger().Error("failed to parse price amount", "action_id", action.ActionID, "amount", action.Price.Amount)
        return nil, status.Errorf(codes.Internal, "invalid price")
    }
    price := sdk.NewCoin(action.Price.Denom, amount)

	return &types2.QueryGetActionResponse{Action: &types2.Action{
		Creator:        action.Creator,
		ActionID:       action.ActionID,
		ActionType:     types2.ActionType(action.ActionType),
		Metadata:       action.Metadata,
		Price:          price,
		ExpirationTime: action.ExpirationTime,
		State:          types2.ActionState(action.State),
		BlockHeight:    action.BlockHeight,
		SuperNodes:     action.SuperNodes,
	}}, nil
}
