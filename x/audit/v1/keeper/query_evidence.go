package keeper

import (
	"context"
	"errors"

	"cosmossdk.io/collections"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (q queryServer) EvidenceById(ctx context.Context, req *types.QueryEvidenceByIdRequest) (*types.QueryEvidenceByIdResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ev, err := q.k.Evidences.Get(ctx, req.EvidenceId)
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "evidence not found")
		}
		return nil, status.Error(codes.Internal, "internal error")
	}

	return &types.QueryEvidenceByIdResponse{Evidence: ev}, nil
}

func (q queryServer) EvidenceBySubject(ctx context.Context, req *types.QueryEvidenceBySubjectRequest) (*types.QueryEvidenceBySubjectResponse, error) {
	if req == nil || req.SubjectAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "subject_address is required")
	}

	if _, err := q.k.addressCodec.StringToBytes(req.SubjectAddress); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid subject_address")
	}

	keys, pageRes, err := paginatePrefixedPairs(ctx, q.k.BySubject, req.Pagination, req.SubjectAddress)
	if err != nil {
		return nil, err
	}

	evidence := make([]types.Evidence, 0, len(keys))
	for _, id := range keys {
		ev, err := q.k.Evidences.Get(ctx, id)
		if err != nil {
			if errors.Is(err, collections.ErrNotFound) {
				continue
			}
			return nil, status.Error(codes.Internal, "internal error")
		}
		evidence = append(evidence, ev)
	}

	return &types.QueryEvidenceBySubjectResponse{Evidence: evidence, Pagination: pageRes}, nil
}

func (q queryServer) EvidenceByAction(ctx context.Context, req *types.QueryEvidenceByActionRequest) (*types.QueryEvidenceByActionResponse, error) {
	if req == nil || req.ActionId == "" {
		return nil, status.Error(codes.InvalidArgument, "action_id is required")
	}

	keys, pageRes, err := paginatePrefixedPairs(ctx, q.k.ByActionID, req.Pagination, req.ActionId)
	if err != nil {
		return nil, err
	}

	evidence := make([]types.Evidence, 0, len(keys))
	for _, id := range keys {
		ev, err := q.k.Evidences.Get(ctx, id)
		if err != nil {
			if errors.Is(err, collections.ErrNotFound) {
				continue
			}
			return nil, status.Error(codes.Internal, "internal error")
		}
		evidence = append(evidence, ev)
	}

	return &types.QueryEvidenceByActionResponse{Evidence: evidence, Pagination: pageRes}, nil
}

func paginatePrefixedPairs(
	ctx context.Context,
	idx collections.KeySet[collections.Pair[string, uint64]],
	pageReq *query.PageRequest,
	prefix string,
) ([]uint64, *query.PageResponse, error) {
	var (
		offset uint64
		limit  uint64 = 100
		total  uint64
	)
	if pageReq != nil {
		offset = pageReq.Offset
		if pageReq.Limit != 0 {
			limit = pageReq.Limit
		}
	}

	// Count total if requested.
	if pageReq != nil && pageReq.CountTotal {
		rng := collections.NewPrefixedPairRange[string, uint64](prefix)
		iter, err := idx.Iterate(ctx, rng)
		if err != nil {
			return nil, nil, status.Error(codes.Internal, "internal error")
		}
		for ; iter.Valid(); iter.Next() {
			total++
		}
		if err := iter.Close(); err != nil {
			return nil, nil, status.Error(codes.Internal, "internal error")
		}
	}

	rng := collections.NewPrefixedPairRange[string, uint64](prefix)
	iter, err := idx.Iterate(ctx, rng)
	if err != nil {
		return nil, nil, status.Error(codes.Internal, "internal error")
	}
	defer func() { _ = iter.Close() }()

	ids := make([]uint64, 0)
	var skipped uint64
	for ; iter.Valid(); iter.Next() {
		key, err := iter.Key()
		if err != nil {
			return nil, nil, status.Error(codes.Internal, "internal error")
		}
		if skipped < offset {
			skipped++
			continue
		}
		if uint64(len(ids)) >= limit {
			break
		}
		ids = append(ids, key.K2())
	}

	return ids, &query.PageResponse{Total: total}, nil
}
