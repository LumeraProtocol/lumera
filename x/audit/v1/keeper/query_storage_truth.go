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

func (q queryServer) NodeSuspicionState(ctx context.Context, req *types.QueryNodeSuspicionStateRequest) (*types.QueryNodeSuspicionStateResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if req.SupernodeAccount == "" {
		return nil, status.Error(codes.InvalidArgument, "supernode_account is required")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	state, found := q.k.GetNodeSuspicionState(sdkCtx, req.SupernodeAccount)
	if !found {
		return nil, status.Error(codes.NotFound, "node suspicion state not found")
	}

	return &types.QueryNodeSuspicionStateResponse{State: state}, nil
}

func (q queryServer) ReporterReliabilityState(ctx context.Context, req *types.QueryReporterReliabilityStateRequest) (*types.QueryReporterReliabilityStateResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if req.ReporterSupernodeAccount == "" {
		return nil, status.Error(codes.InvalidArgument, "reporter_supernode_account is required")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	state, found := q.k.GetReporterReliabilityState(sdkCtx, req.ReporterSupernodeAccount)
	if !found {
		return nil, status.Error(codes.NotFound, "reporter reliability state not found")
	}

	return &types.QueryReporterReliabilityStateResponse{State: state}, nil
}

func (q queryServer) TicketDeteriorationState(ctx context.Context, req *types.QueryTicketDeteriorationStateRequest) (*types.QueryTicketDeteriorationStateResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if req.TicketId == "" {
		return nil, status.Error(codes.InvalidArgument, "ticket_id is required")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	state, found := q.k.GetTicketDeteriorationState(sdkCtx, req.TicketId)
	if !found {
		return nil, status.Error(codes.NotFound, "ticket deterioration state not found")
	}

	return &types.QueryTicketDeteriorationStateResponse{State: state}, nil
}

func (q queryServer) HealOp(ctx context.Context, req *types.QueryHealOpRequest) (*types.QueryHealOpResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if req.HealOpId == 0 {
		return nil, status.Error(codes.InvalidArgument, "heal_op_id is required")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	healOp, found := q.k.GetHealOp(sdkCtx, req.HealOpId)
	if !found {
		return nil, status.Error(codes.NotFound, "heal op not found")
	}

	return &types.QueryHealOpResponse{HealOp: healOp}, nil
}

func (q queryServer) HealOpsByTicket(ctx context.Context, req *types.QueryHealOpsByTicketRequest) (*types.QueryHealOpsByTicketResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if req.TicketId == "" {
		return nil, status.Error(codes.InvalidArgument, "ticket_id is required")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	storeAdapter := runtime.KVStoreAdapter(q.k.storeService.OpenKVStore(sdkCtx))
	store := prefix.NewStore(storeAdapter, types.HealOpByTicketIndexPrefix(req.TicketId))

	var healOps []types.HealOp
	pagination := req.Pagination
	if pagination == nil {
		pagination = &query.PageRequest{Limit: 100}
	}

	pageRes, err := query.Paginate(store, pagination, func(key, _ []byte) error {
		if len(key) != 8 {
			return status.Error(codes.Internal, "invalid heal op ticket index key")
		}
		healOpID := binary.BigEndian.Uint64(key)
		healOp, found := q.k.GetHealOp(sdkCtx, healOpID)
		if !found {
			return nil
		}
		healOps = append(healOps, healOp)
		return nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryHealOpsByTicketResponse{
		HealOps:    healOps,
		Pagination: pageRes,
	}, nil
}

func (q queryServer) HealOpsByStatus(ctx context.Context, req *types.QueryHealOpsByStatusRequest) (*types.QueryHealOpsByStatusResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if req.Status == types.HealOpStatus_HEAL_OP_STATUS_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "status is required")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	storeAdapter := runtime.KVStoreAdapter(q.k.storeService.OpenKVStore(sdkCtx))
	store := prefix.NewStore(storeAdapter, types.HealOpByStatusIndexPrefix(req.Status))

	var healOps []types.HealOp
	pagination := req.Pagination
	if pagination == nil {
		pagination = &query.PageRequest{Limit: 100}
	}

	pageRes, err := query.Paginate(store, pagination, func(key, _ []byte) error {
		if len(key) != 8 {
			return status.Error(codes.Internal, "invalid heal op status index key")
		}
		healOpID := binary.BigEndian.Uint64(key)
		healOp, found := q.k.GetHealOp(sdkCtx, healOpID)
		if !found {
			return nil
		}
		healOps = append(healOps, healOp)
		return nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryHealOpsByStatusResponse{
		HealOps:    healOps,
		Pagination: pageRes,
	}, nil
}
