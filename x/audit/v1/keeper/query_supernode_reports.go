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

func (q queryServer) SupernodeReports(ctx context.Context, req *types.QuerySupernodeReportsRequest) (*types.QuerySupernodeReportsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if req.SupernodeAccount == "" {
		return nil, status.Error(codes.InvalidArgument, "supernode_account is required")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	storeAdapter := runtime.KVStoreAdapter(q.k.storeService.OpenKVStore(sdkCtx))

	var store prefix.Store
	useWindowFilter := req.FilterByWindowId || req.WindowId != 0
	if useWindowFilter {
		store = prefix.NewStore(storeAdapter, types.SupernodeReportIndexWindowPrefix(req.SupernodeAccount, req.WindowId))
	} else {
		store = prefix.NewStore(storeAdapter, types.SupernodeReportIndexPrefix(req.SupernodeAccount))
	}

	var reports []types.SupernodeReport

	pagination := req.Pagination
	if pagination == nil {
		pagination = &query.PageRequest{Limit: 100}
	}

	pageRes, err := query.Paginate(store, pagination, func(key, _ []byte) error {
		var (
			windowID uint64
			reporter string
		)

		if useWindowFilter {
			windowID = req.WindowId
			reporter = string(key)
		} else {
			if len(key) < 9 || key[8] != '/' {
				return status.Error(codes.Internal, "invalid supernode report index key")
			}
			windowID = binary.BigEndian.Uint64(key[:8])
			reporter = string(key[9:])
		}

		if reporter == "" || reporter == req.SupernodeAccount {
			return nil
		}

		r, found := q.k.GetReport(sdkCtx, windowID, reporter)
		if !found {
			return nil
		}

		var portStates []types.PortState
		for _, obs := range r.PeerObservations {
			if obs.TargetSupernodeAccount != req.SupernodeAccount {
				continue
			}
			for _, ps := range obs.PortStates {
				portStates = append(portStates, ps)
			}
			break
		}
		if len(portStates) == 0 {
			return nil
		}

		reports = append(reports, types.SupernodeReport{
			ReporterSupernodeAccount: reporter,
			WindowId:                 r.WindowId,
			ReportHeight:             r.ReportHeight,
			PortStates:               portStates,
		})
		return nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QuerySupernodeReportsResponse{
		Reports:    reports,
		Pagination: pageRes,
	}, nil
}
