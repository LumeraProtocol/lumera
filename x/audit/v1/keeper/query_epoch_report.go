package keeper

import (
	"context"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (q queryServer) EpochReport(ctx context.Context, req *types.QueryEpochReportRequest) (*types.QueryEpochReportResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if req.SupernodeAccount == "" {
		return nil, status.Error(codes.InvalidArgument, "supernode_account is required")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Validate the reporter is a registered supernode.
	_, foundSN, err := q.k.supernodeKeeper.GetSuperNodeByAccount(sdkCtx, req.SupernodeAccount)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !foundSN {
		return nil, status.Error(codes.NotFound, "supernode not found")
	}

	r, found := q.k.GetReport(sdkCtx, req.EpochId, req.SupernodeAccount)
	if !found {
		return nil, status.Error(codes.NotFound, "epoch report not found")
	}

	return &types.QueryEpochReportResponse{Report: r}, nil
}
