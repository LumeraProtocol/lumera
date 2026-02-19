package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func (q queryServer) EpochAnchor(ctx context.Context, req *types.QueryEpochAnchorRequest) (*types.QueryEpochAnchorResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	anchor, found := q.k.GetEpochAnchor(sdkCtx, req.EpochId)
	if !found {
		return nil, status.Error(codes.NotFound, "epoch anchor not found")
	}

	return &types.QueryEpochAnchorResponse{Anchor: anchor}, nil
}
