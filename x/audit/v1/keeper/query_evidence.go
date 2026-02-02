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

func (q queryServer) EvidenceById(ctx context.Context, req *types.QueryEvidenceByIdRequest) (*types.QueryEvidenceByIdResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ev, found := q.k.GetEvidence(sdkCtx, req.EvidenceId)
	if !found {
		return nil, status.Error(codes.NotFound, "evidence not found")
	}

	return &types.QueryEvidenceByIdResponse{Evidence: ev}, nil
}

func (q queryServer) EvidenceBySubject(ctx context.Context, req *types.QueryEvidenceBySubjectRequest) (*types.QueryEvidenceBySubjectResponse, error) {
	if req == nil || req.SubjectAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "subject_address is required")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if _, err := q.k.addressCodec.StringToBytes(req.SubjectAddress); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid subject_address")
	}

	storeAdapter := runtime.KVStoreAdapter(q.k.storeService.OpenKVStore(sdkCtx))
	store := prefix.NewStore(storeAdapter, types.EvidenceBySubjectIndexPrefix(req.SubjectAddress))

	var evidence []types.Evidence

	pagination := req.Pagination
	if pagination == nil {
		pagination = &query.PageRequest{Limit: 100}
	}

	pageRes, err := query.Paginate(store, pagination, func(key, _ []byte) error {
		if len(key) != 8 {
			return status.Error(codes.Internal, "invalid evidence index key")
		}
		evidenceID := binary.BigEndian.Uint64(key)
		ev, found := q.k.GetEvidence(sdkCtx, evidenceID)
		if !found {
			return nil
		}
		evidence = append(evidence, ev)
		return nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryEvidenceBySubjectResponse{
		Evidence:    evidence,
		Pagination: pageRes,
	}, nil
}

func (q queryServer) EvidenceByAction(ctx context.Context, req *types.QueryEvidenceByActionRequest) (*types.QueryEvidenceByActionResponse, error) {
	if req == nil || req.ActionId == "" {
		return nil, status.Error(codes.InvalidArgument, "action_id is required")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	storeAdapter := runtime.KVStoreAdapter(q.k.storeService.OpenKVStore(sdkCtx))
	store := prefix.NewStore(storeAdapter, types.EvidenceByActionIndexPrefix(req.ActionId))

	var evidence []types.Evidence

	pagination := req.Pagination
	if pagination == nil {
		pagination = &query.PageRequest{Limit: 100}
	}

	pageRes, err := query.Paginate(store, pagination, func(key, _ []byte) error {
		if len(key) != 8 {
			return status.Error(codes.Internal, "invalid evidence index key")
		}
		evidenceID := binary.BigEndian.Uint64(key)
		ev, found := q.k.GetEvidence(sdkCtx, evidenceID)
		if !found {
			return nil
		}
		evidence = append(evidence, ev)
		return nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryEvidenceByActionResponse{
		Evidence:    evidence,
		Pagination: pageRes,
	}, nil
}

