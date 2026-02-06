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

	useEpochFilter := req.FilterByEpochId || req.EpochId != 0

	var store prefix.Store
	if useEpochFilter {
		store = prefix.NewStore(storeAdapter, types.SelfReportIndexKey(req.SupernodeAccount, req.EpochId))
	} else {
		store = prefix.NewStore(storeAdapter, types.SelfReportIndexPrefix(req.SupernodeAccount))
	}

	var reports []types.SelfReport

	pagination := req.Pagination
	if pagination == nil {
		pagination = &query.PageRequest{Limit: 100}
	}

	pageRes, err := query.Paginate(store, pagination, func(key, _ []byte) error {
		var epochID uint64
		if useEpochFilter {
			epochID = req.EpochId
		} else {
			if len(key) != 8 {
				return status.Error(codes.Internal, "invalid self report index key")
			}
			epochID = binary.BigEndian.Uint64(key)
		}

		r, found := q.k.GetReport(sdkCtx, epochID, req.SupernodeAccount)
		if !found {
			return nil
		}
		reports = append(reports, types.SelfReport{
			EpochId:      r.EpochId,
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
