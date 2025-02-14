package keeper

import (
	"context"

	"github.com/LumeraProtocol/lumera/x/action/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetAction returns the action for the given action-id
func (k Keeper) GetAction(goCtx context.Context, req *types.QueryGetActionRequest) (*types.QueryGetActionResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	return &types.QueryGetActionResponse{}, nil
}
