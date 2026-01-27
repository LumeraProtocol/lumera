package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func (q queryServer) CurrentWindow(ctx context.Context, req *types.QueryCurrentWindowRequest) (*types.QueryCurrentWindowResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := q.k.GetParams(ctx).WithDefaults()

	origin, found := q.k.getWindowOriginHeight(sdkCtx)
	if !found {
		return nil, status.Error(codes.NotFound, "window origin height not initialized")
	}

	windowID := q.k.windowIDAtHeight(origin, params, sdkCtx.BlockHeight())
	windowStart := q.k.windowStartHeight(origin, params, windowID)
	windowEnd := q.k.windowEndHeight(origin, params, windowID)

	return &types.QueryCurrentWindowResponse{
		WindowId:          windowID,
		WindowStartHeight: windowStart,
		WindowEndHeight:   windowEnd,
	}, nil
}
