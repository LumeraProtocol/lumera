package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/everlight/v1/types"
)

type queryServer struct {
	k Keeper
}

// NewQueryServerImpl returns an implementation of the QueryServer interface
// for the provided Keeper.
func NewQueryServerImpl(keeper Keeper) types.QueryServer {
	return &queryServer{k: keeper}
}

var _ types.QueryServer = queryServer{}

func (q queryServer) Params(goCtx context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := q.k.GetParams(ctx)
	return &types.QueryParamsResponse{Params: params}, nil
}

func (q queryServer) PoolState(goCtx context.Context, _ *types.QueryPoolStateRequest) (*types.QueryPoolStateResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	balance := q.k.GetPoolBalance(ctx)
	lastHeight := q.k.GetLastDistributionHeight(ctx)

	return &types.QueryPoolStateResponse{
		Balance:                balance,
		LastDistributionHeight: lastHeight,
		TotalDistributed:       sdk.Coins{}, // TODO: implement in S13
		EligibleSnCount:        0,           // TODO: implement in S13
	}, nil
}

func (q queryServer) SNEligibility(_ context.Context, _ *types.QuerySNEligibilityRequest) (*types.QuerySNEligibilityResponse, error) {
	// TODO: implement in S13 when distribution logic is added
	return &types.QuerySNEligibilityResponse{
		Eligible: false,
		Reason:   "eligibility check not yet implemented",
	}, nil
}
