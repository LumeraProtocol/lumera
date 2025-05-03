package keeper

import (
	"context"

	"github.com/LumeraProtocol/lumera/x/action/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetAction returns the action for the given action-id
func (k *Keeper) GetAction(goCtx context.Context, req *types.QueryGetActionRequest) (*types.QueryGetActionResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	action, ok := k.GetActionByID(ctx, req.ActionID)
	if !ok {
		return nil, status.Errorf(codes.Internal, "failed to get action by ID")
	}

	price, err := sdk.ParseCoinNormalized(action.Price)
	if err != nil {
		k.Logger().Error("failed to parse price", "action_id", action.ActionID, "price", action.Price, "error", err)
		return nil, status.Errorf(codes.Internal, "invalid price")
	}

	return &types.QueryGetActionResponse{Action: &types.Action{
		Creator:        action.Creator,
		ActionID:       action.ActionID,
		ActionType:     types.ActionType(action.ActionType),
		Metadata:       action.Metadata,
		Price:          &price,
		ExpirationTime: action.ExpirationTime,
		State:          types.ActionState(action.State),
		BlockHeight:    action.BlockHeight,
		SuperNodes:     action.SuperNodes,
	}}, nil
}
