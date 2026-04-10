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
		return &types.QuerySNEligibilityResponse{Eligible: false, Reason: "invalid validator address: " + err.Error()}, nil
	}

	sn, found := q.k.QuerySuperNode(ctx, valAddr)
	if !found {
		return &types.QuerySNEligibilityResponse{Eligible: false, Reason: "supernode not found"}, nil
	}

	// State gate: only ACTIVE or STORAGE_FULL are payout-eligible.
	if len(sn.States) == 0 {
		return &types.QuerySNEligibilityResponse{Eligible: false, Reason: "supernode has no state history"}, nil
	}
	lastState := sn.States[len(sn.States)-1].State
	if lastState != types.SuperNodeStateActive && lastState != types.SuperNodeStateStorageFull {
		return &types.QuerySNEligibilityResponse{Eligible: false, Reason: "supernode state is not eligible"}, nil
	}

	rawBytes, reportHeight, ok := q.k.GetLatestCascadeBytesForPayout(ctx, sn.SupernodeAccount)
	if !ok {
		return &types.QuerySNEligibilityResponse{Eligible: false, Reason: "no audit epoch report found"}, nil
	}
	if !isFreshByBlockHeight(ctx.BlockHeight(), reportHeight, params.MetricsFreshnessMaxBlocks) {
		return &types.QuerySNEligibilityResponse{Eligible: false, Reason: "audit report is stale", CascadeKademliaDbBytes: rawBytes}, nil
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
