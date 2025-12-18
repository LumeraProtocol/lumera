package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// GetSuperNodeBySuperNodeAddress returns the supernode for the given supernode address
func (q queryServer) GetSuperNodeBySuperNodeAddress(goCtx context.Context, req *types.QueryGetSuperNodeBySuperNodeAddressRequest) (*types.QueryGetSuperNodeBySuperNodeAddressResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	sn, found, err := q.k.GetSuperNodeByAccount(ctx, req.SupernodeAddress)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get supernode: %v", err)
	}

	if !found {
		return nil, status.Errorf(codes.NotFound, "supernode not found")
	}

	return &types.QueryGetSuperNodeBySuperNodeAddressResponse{Supernode: &sn}, nil
}
