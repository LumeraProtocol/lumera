package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func (q queryServer) CurrentEpochAnchor(ctx context.Context, req *types.QueryCurrentEpochAnchorRequest) (*types.QueryCurrentEpochAnchorResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := q.k.GetParams(ctx).WithDefaults()
	epoch, err := deriveEpochAtHeight(sdkCtx.BlockHeight(), params)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	anchor, found := q.k.GetEpochAnchor(sdkCtx, epoch.EpochID)
	if !found {
		return nil, status.Error(codes.NotFound, "current epoch anchor not found")
	}

	return &types.QueryCurrentEpochAnchorResponse{Anchor: anchor}, nil
}
