package keeper

import (
	"context"

	"github.com/LumeraProtocol/lumera/x/action/v1/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetAction returns the action for the given action-id
func (q queryServer) GetAction(goCtx context.Context, req *types.QueryGetActionRequest) (*types.QueryGetActionResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	action, ok := q.k.GetActionByID(ctx, req.ActionID)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "failed to get action by ID")
	}

	return &types.QueryGetActionResponse{Action: &types.Action{
		Creator:        action.Creator,
		ActionID:       action.ActionID,
		ActionType:     types.ActionType(action.ActionType),
		Metadata:       action.Metadata,
		Price:          action.Price,
		ExpirationTime: action.ExpirationTime,
		State:          types.ActionState(action.State),
		BlockHeight:    action.BlockHeight,
		SuperNodes:     action.SuperNodes,
	}}, nil
}
