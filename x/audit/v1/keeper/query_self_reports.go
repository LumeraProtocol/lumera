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

func (q queryServer) SelfReports(ctx context.Context, req *types.QuerySelfReportsRequest) (*types.QuerySelfReportsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if req.SupernodeAccount == "" {
		return nil, status.Error(codes.InvalidArgument, "supernode_account is required")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	storeAdapter := runtime.KVStoreAdapter(q.k.storeService.OpenKVStore(sdkCtx))

	useWindowFilter := req.FilterByWindowId || req.WindowId != 0

	var store prefix.Store
	if useWindowFilter {
		store = prefix.NewStore(storeAdapter, types.SelfReportIndexKey(req.SupernodeAccount, req.WindowId))
	} else {
		store = prefix.NewStore(storeAdapter, types.SelfReportIndexPrefix(req.SupernodeAccount))
	}

	var reports []types.SelfReport

	pagination := req.Pagination
	if pagination == nil {
		pagination = &query.PageRequest{Limit: 100}
	}

	pageRes, err := query.Paginate(store, pagination, func(key, _ []byte) error {
		var windowID uint64
		if useWindowFilter {
			windowID = req.WindowId
		} else {
			if len(key) != 8 {
				return status.Error(codes.Internal, "invalid self report index key")
			}
			windowID = binary.BigEndian.Uint64(key)
		}

		r, found := q.k.GetReport(sdkCtx, windowID, req.SupernodeAccount)
		if !found {
			return nil
		}
		reports = append(reports, types.SelfReport{
			WindowId:     r.WindowId,
			ReportHeight: r.ReportHeight,
			SelfReport:   r.SelfReport,
		})
		return nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QuerySelfReportsResponse{
		Reports:    reports,
		Pagination: pageRes,
	}, nil
}
