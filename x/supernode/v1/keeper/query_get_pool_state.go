package keeper

import (
	"context"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// PoolState returns the current state of the pool, including balance, last distribution height, total distributed funds, and count of eligible supernodes.
func (q queryServer) PoolState(goCtx context.Context, _ *types.QueryPoolStateRequest) (*types.QueryPoolStateResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	balance := q.k.GetPoolBalance(ctx)
	lastHeight := q.k.GetLastDistributionHeight(ctx)
	totalDistributed := q.k.GetTotalDistributed(ctx)
	eligibleCount := q.k.CountEligibleSNs(ctx)

	return &types.QueryPoolStateResponse{
		Balance:                balance,
		LastDistributionHeight: lastHeight,
		TotalDistributed:       totalDistributed,
		EligibleSnCount:        eligibleCount,
	}, nil
}
