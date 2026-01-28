package keeper

import (
	"context"
	"encoding/binary"

	"cosmossdk.io/store/prefix"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (q queryServer) AuditReportsByReporter(ctx context.Context, req *types.QueryAuditReportsByReporterRequest) (*types.QueryAuditReportsByReporterResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if req.SupernodeAccount == "" {
		return nil, status.Error(codes.InvalidArgument, "supernode_account is required")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Validate the reporter is a registered supernode.
	_, found, err := q.k.supernodeKeeper.GetSuperNodeByAccount(sdkCtx, req.SupernodeAccount)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !found {
		return nil, status.Error(codes.NotFound, "supernode not found")
	}

	storeAdapter := runtime.KVStoreAdapter(q.k.storeService.OpenKVStore(sdkCtx))
	store := prefix.NewStore(storeAdapter, types.ReportIndexPrefix(req.SupernodeAccount))

	var reports []types.AuditReport

	pagination := req.Pagination
	if pagination == nil {
		pagination = &query.PageRequest{Limit: 100}
	}

	pageRes, err := query.Paginate(store, pagination, func(key, _ []byte) error {
		if len(key) != 8 {
			return status.Error(codes.Internal, "invalid report index key")
		}
		windowID := binary.BigEndian.Uint64(key)
		r, found := q.k.GetReport(sdkCtx, windowID, req.SupernodeAccount)
		if !found {
			return nil
		}
		reports = append(reports, r)
		return nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryAuditReportsByReporterResponse{
		Reports:    reports,
		Pagination: pageRes,
	}, nil
}
