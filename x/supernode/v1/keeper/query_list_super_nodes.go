package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

func (q queryServer) ListSuperNodes(goCtx context.Context, req *types.QueryListSuperNodesRequest) (*types.QueryListSuperNodesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	supernodes, pageRes, err := q.k.GetSuperNodesPaginated(ctx, req.Pagination)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list supernodes: %v", err)
	}

	return &types.QueryListSuperNodesResponse{Supernodes: supernodes, Pagination: pageRes}, nil
}
