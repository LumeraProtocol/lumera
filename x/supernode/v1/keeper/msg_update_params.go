package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

func (k msgServer) UpdateParams(goCtx context.Context, req *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if k.GetAuthority() != req.Authority {
		return nil, errorsmod.Wrapf(types.ErrInvalidSigner, "invalid authority; expected %s, got %s", k.GetAuthority(), req.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	current := k.GetParams(ctx)
	merged := mergeParams(current, req.Params).WithDefaults()

	if err := merged.Validate(); err != nil {
		return nil, err
	}

	if err := k.SetParams(ctx, merged); err != nil {
		return nil, err
	}

	return &types.MsgUpdateParamsResponse{}, nil
}

func mergeParams(base, incoming types.Params) types.Params {
	merged := base

	if incoming.MinimumStakeForSn.IsValid() && !incoming.MinimumStakeForSn.IsZero() {
		merged.MinimumStakeForSn = incoming.MinimumStakeForSn
	}

	if incoming.ReportingThreshold != 0 {
		merged.ReportingThreshold = incoming.ReportingThreshold
	}

	if incoming.SlashingThreshold != 0 {
		merged.SlashingThreshold = incoming.SlashingThreshold
	}

	if incoming.MetricsThresholds != "" {
		merged.MetricsThresholds = incoming.MetricsThresholds
	}

	if incoming.EvidenceRetentionPeriod != "" {
		merged.EvidenceRetentionPeriod = incoming.EvidenceRetentionPeriod
	}

	if incoming.SlashingFraction != "" {
		merged.SlashingFraction = incoming.SlashingFraction
	}

	if incoming.InactivityPenaltyPeriod != "" {
		merged.InactivityPenaltyPeriod = incoming.InactivityPenaltyPeriod
	}

	if incoming.MetricsUpdateIntervalBlocks != 0 {
		merged.MetricsUpdateIntervalBlocks = incoming.MetricsUpdateIntervalBlocks
	}

	if incoming.MetricsGracePeriodBlocks != 0 {
		merged.MetricsGracePeriodBlocks = incoming.MetricsGracePeriodBlocks
	}

	if incoming.MetricsFreshnessMaxBlocks != 0 {
		merged.MetricsFreshnessMaxBlocks = incoming.MetricsFreshnessMaxBlocks
	}

	if incoming.MinSupernodeVersion != "" {
		merged.MinSupernodeVersion = incoming.MinSupernodeVersion
	}

	if incoming.MinCpuCores != 0 {
		merged.MinCpuCores = incoming.MinCpuCores
	}

	if incoming.MaxCpuUsagePercent != 0 {
		merged.MaxCpuUsagePercent = incoming.MaxCpuUsagePercent
	}

	if incoming.MinMemGb != 0 {
		merged.MinMemGb = incoming.MinMemGb
	}

	if incoming.MaxMemUsagePercent != 0 {
		merged.MaxMemUsagePercent = incoming.MaxMemUsagePercent
	}

	if incoming.MinStorageGb != 0 {
		merged.MinStorageGb = incoming.MinStorageGb
	}

	if incoming.MaxStorageUsagePercent != 0 {
		merged.MaxStorageUsagePercent = incoming.MaxStorageUsagePercent
	}

	if len(incoming.RequiredOpenPorts) > 0 {
		merged.RequiredOpenPorts = incoming.RequiredOpenPorts
	}

	return merged
}
