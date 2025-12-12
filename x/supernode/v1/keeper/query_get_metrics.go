package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// GetMetrics returns the latest metrics state for a given validator address.
func (q queryServer) GetMetrics(goCtx context.Context, req *types.QueryGetMetricsRequest) (*types.QueryGetMetricsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	valOperAddr, err := sdk.ValAddressFromBech32(req.ValidatorAddress)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid validator address: %v", err)
	}

	state, found := q.k.GetMetricsState(ctx, valOperAddr)
	if !found {
		return nil, status.Errorf(codes.NotFound, "no metrics found for validator %s", req.ValidatorAddress)
	}

	return &types.QueryGetMetricsResponse{MetricsState: &state}, nil
}
