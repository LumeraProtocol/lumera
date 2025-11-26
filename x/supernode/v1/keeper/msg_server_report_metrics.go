package keeper

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

var canonicalMetricRanges = map[string]struct {
	min float64
	max float64
}{
	types.MetricKeyCPUUsage:     {min: 0, max: 100},
	types.MetricKeyMemoryUsage:  {min: 0, max: 100},
	types.MetricKeyStorageUsage: {min: 0, max: 100},
	types.MetricKeyP2PPortOpen:  {min: 0, max: 1},
	types.MetricKeyRPCPortOpen:  {min: 0, max: 1},
	// freshness is expressed in blocks; allow any non-negative number.
	types.MetricKeyFreshness: {min: 0, max: 1_000_000_000},
}

func parseMetricsThresholds(thresholds string) map[string]float64 {
	result := make(map[string]float64)
	parts := strings.Split(thresholds, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		kv := strings.SplitN(p, ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val, err := strconv.ParseFloat(strings.TrimSpace(kv[1]), 64)
		if err != nil {
			continue
		}
		result[key] = val
	}
	return result
}

// ReportSupernodeMetrics ingests telemetry reports and updates compliance state.
func (k msgServer) ReportSupernodeMetrics(goCtx context.Context, msg *types.MsgReportSupernodeMetrics) (*types.MsgReportSupernodeMetricsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	valOperAddr, err := sdk.ValAddressFromBech32(msg.ValidatorAddress)
	if err != nil {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid validator address: %s", err)
	}

	supernode, found := k.QuerySuperNode(ctx, valOperAddr)
	if !found {
		return nil, errorsmod.Wrapf(sdkerrors.ErrNotFound, "no supernode found for validator %s", msg.ValidatorAddress)
	}

	creatorAddr, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address: %s", err)
	}

	valAccAddr := sdk.AccAddress(valOperAddr)
	supernodeAcc, err := sdk.AccAddressFromBech32(supernode.SupernodeAccount)
	if err != nil {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid supernode account: %s", err)
	}

	if !(creatorAddr.Equals(valAccAddr) || creatorAddr.Equals(supernodeAcc)) {
		return nil, errorsmod.Wrapf(sdkerrors.ErrUnauthorized, "creator %s is not authorized for supernode", msg.Creator)
	}

	issues := make([]string, 0)
	sanitized := make(map[string]float64)

	for kKey, v := range msg.Metrics {
		rangeSpec, ok := canonicalMetricRanges[kKey]
		if !ok {
			issues = append(issues, fmt.Sprintf("unknown metric key: %s", kKey))
			continue
		}

		if v < rangeSpec.min || v > rangeSpec.max {
			issues = append(issues, fmt.Sprintf("metric %s out of range", kKey))
			continue
		}

		sanitized[kKey] = v
	}

	params := k.GetParams(ctx)
	thresholds := parseMetricsThresholds(params.MetricsThresholds)

	// Version compliance
	if msg.Version == "" {
		msg.Version = supernode.Note
	}
	if supernode.Note != "" && msg.Version != supernode.Note {
		issues = append(issues, fmt.Sprintf("version mismatch: expected %s", supernode.Note))
	}

	// Usage thresholds
	if threshold, ok := thresholds["cpu"]; ok {
		if val, exists := sanitized[types.MetricKeyCPUUsage]; exists && val > threshold {
			issues = append(issues, "cpu usage above threshold")
		}
	}
	if threshold, ok := thresholds["memory"]; ok {
		if val, exists := sanitized[types.MetricKeyMemoryUsage]; exists && val > threshold {
			issues = append(issues, "memory usage above threshold")
		}
	}
	if threshold, ok := thresholds["storage"]; ok {
		if val, exists := sanitized[types.MetricKeyStorageUsage]; exists && val > threshold {
			issues = append(issues, "storage usage above threshold")
		}
	}

	// Port checks: expect value >=1 to indicate open
	if val, ok := sanitized[types.MetricKeyP2PPortOpen]; ok && val < 1 {
		issues = append(issues, "p2p port not reachable")
	}
	if val, ok := sanitized[types.MetricKeyRPCPortOpen]; ok && val < 1 {
		issues = append(issues, "rpc port not reachable")
	}

	// Freshness based on reporting threshold parameter (in blocks)
	reportedHeight := msg.ReportedHeight
	if reportedHeight == 0 {
		reportedHeight = ctx.BlockHeight()
	}
	if params.ReportingThreshold > 0 {
		delta := ctx.BlockHeight() - reportedHeight
		if delta < 0 {
			delta = -delta
		}
		if uint64(delta) > params.ReportingThreshold {
			issues = append(issues, "metrics report is stale")
		}
	}

	metricsAgg := supernode.Metrics
	if metricsAgg == nil {
		metricsAgg = &types.MetricsAggregate{}
	}
	if metricsAgg.Metrics == nil {
		metricsAgg.Metrics = make(map[string]float64)
	}

	for kKey, v := range sanitized {
		metricsAgg.Metrics[kKey] = v
	}
	metricsAgg.ReportCount++
	metricsAgg.Height = ctx.BlockHeight()
	supernode.Metrics = metricsAgg

	// Manage state transitions
	var currentState types.SuperNodeState
	if len(supernode.States) > 0 {
		currentState = supernode.States[len(supernode.States)-1].State
	}

	previousEffective := currentState
	for i := len(supernode.States) - 1; i >= 0; i-- {
		if supernode.States[i].State != types.SuperNodeStatePostponed {
			previousEffective = supernode.States[i].State
			break
		}
	}

	transition := ""
	if len(issues) > 0 {
		switch currentState {
		case types.SuperNodeStateActive, types.SuperNodeStateDisabled:
			supernode.States = append(supernode.States, &types.SuperNodeStateRecord{State: types.SuperNodeStatePostponed, Height: ctx.BlockHeight()})
			transition = fmt.Sprintf("%s->%s", currentState.String(), types.SuperNodeStatePostponed.String())
		}
	} else {
		if currentState == types.SuperNodeStatePostponed {
			target := previousEffective
			if target == types.SuperNodeStatePostponed || target == types.SuperNodeStateUnspecified {
				target = types.SuperNodeStateActive
			}
			supernode.States = append(supernode.States, &types.SuperNodeStateRecord{State: target, Height: ctx.BlockHeight()})
			transition = fmt.Sprintf("%s->%s", currentState.String(), target.String())
		}
	}

	if err := k.SetSuperNode(ctx, supernode); err != nil {
		return nil, err
	}

	attrs := []sdk.Attribute{
		sdk.NewAttribute(types.AttributeKeyValidatorAddress, msg.ValidatorAddress),
		sdk.NewAttribute(types.AttributeKeyIssuesCount, strconv.Itoa(len(issues))),
		sdk.NewAttribute(types.AttributeKeyHeight, strconv.FormatInt(ctx.BlockHeight(), 10)),
	}
	if transition != "" {
		attrs = append(attrs, sdk.NewAttribute(types.AttributeKeyStateTransition, transition))
	}
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeSupernodeMetrics,
			attrs...,
		),
	)

	return &types.MsgReportSupernodeMetricsResponse{Issues: issues}, nil
}
