package keeper

import (
	"context"

	"github.com/LumeraProtocol/lumera/x/supernode/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetSuperNode returns the supernode for the given validator address
func (k Keeper) GetSuperNode(goCtx context.Context, req *types.QueryGetSuperNodeRequest) (*types.QueryGetSuperNodeResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	valOperAddr, err := sdk.ValAddressFromBech32(req.ValidatorAddress)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid validator address: %v", err)
	}

	sn, found := k.QuerySuperNode(ctx, valOperAddr)
	if !found {
		return nil, status.Errorf(codes.NotFound, "no supernode found for validator %s", req.ValidatorAddress)
	}

	return &types.QueryGetSuperNodeResponse{Supernode: &sn}, nil
}
