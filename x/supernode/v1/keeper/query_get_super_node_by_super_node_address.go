package keeper

import (
	"context"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetSuperNodeBySuperNodeAddress returns the supernode for the given supernode address
func (k Keeper) GetSuperNodeBySuperNodeAddress(goCtx context.Context, req *types.QueryGetSuperNodeBySuperNodeAddressRequest) (*types.QueryGetSuperNodeBySuperNodeAddressResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	superNodes, err := k.GetAllSuperNodes(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get all supernodes: %v", err)
	}

	for _, sn := range superNodes {
		if sn.GetSupernodeAccount() == req.SupernodeAddress {
			return &types.QueryGetSuperNodeBySuperNodeAddressResponse{Supernode: &sn}, nil
		}
	}

	return nil, status.Errorf(codes.NotFound, "supernode not found: %v", err)
}
