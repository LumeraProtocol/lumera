package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pastelnetwork/pastel/x/supernode/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (k Keeper) GetTopSuperNodesForBlock(goCtx context.Context, req *types.QueryGetTopSuperNodesForBlockRequest) (*types.QueryGetTopSuperNodesForBlockResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	// TODO: Process the query
	_ = ctx

	return &types.QueryGetTopSuperNodesForBlockResponse{}, nil
}
