package keeper

import (
	"context"

<<<<<<<< HEAD:x/supernode/v1/keeper/query_params.go
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
========
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/LumeraProtocol/lumera/x/action/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
>>>>>>>> f8437a0 (IBC & Wasm upgrade):x/action/v1/keeper/query_params.go
)

func (q queryServer) Params(goCtx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := q.k.GetParams(ctx)

	return &types.QueryParamsResponse{Params: params}, nil
}
