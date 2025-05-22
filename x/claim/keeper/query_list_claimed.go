package keeper

import (
	"context"

	"github.com/LumeraProtocol/lumera/x/claim/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (k Keeper) ListClaimed(goCtx context.Context, req *types.QueryListClaimedRequest) (*types.QueryListClaimedResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	filter := func(record *types.ClaimRecord) bool {
		return record.Claimed && record.VestedTier == req.VestedTerm
	}
	claims, err := k.ListClaimRecords(ctx, filter)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryListClaimedResponse{
		Claims: claims,
	}, nil
}
