package keeper

import (
	"context"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (q queryServer) AuditReport(ctx context.Context, req *types.QueryAuditReportRequest) (*types.QueryAuditReportResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	r, found := q.k.GetReport(sdkCtx, req.WindowId, req.SupernodeAccount)
	if !found {
		return nil, status.Error(codes.NotFound, "audit report not found")
	}

	return &types.QueryAuditReportResponse{Report: r}, nil
}
