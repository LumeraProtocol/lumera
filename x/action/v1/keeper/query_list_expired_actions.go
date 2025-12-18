package keeper

import (
	"context"

	"github.com/LumeraProtocol/lumera/x/action/v1/types"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ListExpiredActions returns actions with state = EXPIRED
func (q queryServer) ListExpiredActions(goCtx context.Context, req *types.QueryListExpiredActionsRequest) (*types.QueryListExpiredActionsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	// Reuse ListActions with a fixed EXPIRED state filter so it benefits
	// from the state index and consistent pagination semantics.
	listReq := &types.QueryListActionsRequest{
		ActionState: types.ActionStateExpired,
		Pagination:  req.Pagination,
	}

	listResp, err := q.ListActions(goCtx, listReq)
	if err != nil {
		return nil, err
	}

	return &types.QueryListExpiredActionsResponse{
		Actions:    listResp.Actions,
		Pagination: listResp.Pagination,
		Total:      listResp.Total,
	}, nil
}
