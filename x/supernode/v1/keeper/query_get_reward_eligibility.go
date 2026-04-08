package keeper

import (
	"context"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (q queryServer) SNEligibility(goCtx context.Context, req *types.QuerySNEligibilityRequest) (*types.QuerySNEligibilityResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := q.k.GetParams(ctx)
	dist := params.RewardDistribution

	if req == nil || req.ValidatorAddress == "" {
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

	// Check metrics.
	metricsState, found := q.k.GetMetricsState(ctx, valAddr)
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
		cappedBytes := applyGrowthCap(rawBytes, distState.PrevRawBytes, dist.UsageGrowthCapBpsPerPeriod)
		smoothedBytes = applyEMA(distState.SmoothedBytes, cappedBytes, dist.MeasurementSmoothingPeriods)
	}

	if floatToUint64(smoothedBytes) < dist.MinCascadeBytesForPayment {
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
