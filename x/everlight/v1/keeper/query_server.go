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
	totalDistributed := q.k.GetTotalDistributed(ctx)
	eligibleCount := q.k.countEligibleSNs(ctx)

	return &types.QueryPoolStateResponse{
		Balance:                balance,
		LastDistributionHeight: lastHeight,
		TotalDistributed:       totalDistributed,
		EligibleSnCount:        eligibleCount,
	}, nil
}

func (q queryServer) SNEligibility(goCtx context.Context, req *types.QuerySNEligibilityRequest) (*types.QuerySNEligibilityResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if req.ValidatorAddress == "" {
		return &types.QuerySNEligibilityResponse{
			Eligible: false,
			Reason:   "validator_address is required",
		}, nil
	}

	valAddr, err := sdk.ValAddressFromBech32(req.ValidatorAddress)
	if err != nil {
		return &types.QuerySNEligibilityResponse{
			Eligible: false,
			Reason:   "invalid validator address: " + err.Error(),
		}, nil
	}

	params := q.k.GetParams(ctx)

	// Check metrics.
	metricsState, found := q.k.supernodeKeeper.GetMetricsState(ctx, valAddr)
	if !found {
		return &types.QuerySNEligibilityResponse{
			Eligible: false,
			Reason:   "no metrics reported",
		}, nil
	}

	rawBytes := float64(0)
	if metricsState.Metrics != nil {
		rawBytes = metricsState.Metrics.CascadeKademliaDbBytes
	}

	// Load distribution state.
	distState, exists := q.k.GetSNDistState(ctx, req.ValidatorAddress)
	smoothedBytes := rawBytes
	if exists {
		cappedBytes := applyGrowthCap(rawBytes, distState.PrevRawBytes, params.UsageGrowthCapBpsPerPeriod)
		smoothedBytes = applyEMA(distState.SmoothedBytes, cappedBytes, params.MeasurementSmoothingPeriods)
	}

	if floatToUint64(smoothedBytes) < params.MinCascadeBytesForPayment {
		return &types.QuerySNEligibilityResponse{
			Eligible:               false,
			Reason:                 "cascade bytes below minimum threshold",
			CascadeKademliaDbBytes: rawBytes,
			SmoothedWeight:         smoothedBytes,
		}, nil
	}

	return &types.QuerySNEligibilityResponse{
		Eligible:               true,
		Reason:                 "eligible",
		CascadeKademliaDbBytes: rawBytes,
		SmoothedWeight:         smoothedBytes,
	}, nil
}
