package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func (q queryServer) WindowSnapshot(ctx context.Context, req *types.QueryWindowSnapshotRequest) (*types.QueryWindowSnapshotResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	snap, found := q.k.GetWindowSnapshot(sdkCtx, req.WindowId)
	if !found {
		return nil, status.Error(codes.NotFound, "window snapshot not found")
	}

	return &types.QueryWindowSnapshotResponse{Snapshot: snap}, nil
}
