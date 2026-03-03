package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func (q queryServer) CurrentEpoch(ctx context.Context, req *types.QueryCurrentEpochRequest) (*types.QueryCurrentEpochResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := q.k.GetParams(ctx).WithDefaults()

	epoch, err := deriveEpochAtHeight(sdkCtx.BlockHeight(), params)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryCurrentEpochResponse{
		EpochId:          epoch.EpochID,
		EpochStartHeight: epoch.StartHeight,
		EpochEndHeight:   epoch.EndHeight,
	}, nil
}
