package keeper

import (
	"context"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func (q queryServer) Params(ctx context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	params := q.k.GetParams(ctx).WithDefaults()
	return &types.QueryParamsResponse{Params: params}, nil
}
