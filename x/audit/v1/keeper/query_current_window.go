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
	ws, found, err := q.k.getWindowState(sdkCtx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !found {
		return nil, status.Error(codes.NotFound, "current window not initialized")
	}

	return &types.QueryCurrentWindowResponse{
		WindowId:          ws.WindowID,
		WindowStartHeight: ws.StartHeight,
		WindowEndHeight:   ws.EndHeight,
	}, nil
}
