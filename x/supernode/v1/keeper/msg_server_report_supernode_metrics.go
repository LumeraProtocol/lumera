package keeper

import (
	"context"
	"strings"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

func (m msgServer) ReportSupernodeMetrics(goCtx context.Context, msg *types.MsgReportSupernodeMetrics) (*types.MsgReportSupernodeMetricsResponse, error) {
	if msg == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "message cannot be nil")
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	valAddr, err := sdk.ValAddressFromBech32(msg.ValidatorAddress)
	if err != nil {
		return nil, errorsmod.Wrap(err, "invalid validator address")
	}

	sn, found := m.QuerySuperNode(ctx, valAddr)
	if !found {
		return nil, errorsmod.Wrapf(sdkerrors.ErrNotFound, "supernode for validator %s not found", msg.ValidatorAddress)
	}

	// Enforce that metrics are reported only by the currently configured
	// supernode account for this validator. If no supernode account is set on-chain,
	// metrics reporting is not permitted.
	if sn.SupernodeAccount == "" {
		return nil, errorsmod.Wrapf(
			sdkerrors.ErrUnauthorized,
			"supernode account not set for validator %s",
			msg.ValidatorAddress,
		)
	}
	if msg.SupernodeAccount != sn.SupernodeAccount {
		return nil, errorsmod.Wrapf(
			sdkerrors.ErrUnauthorized,
			"reported supernode account %s does not match registered account %s",
			msg.SupernodeAccount,
			sn.SupernodeAccount,
		)
	}

	params := m.GetParams(ctx)
	// Compliance evaluation separates storage-capacity violations from other issues.
	result := evaluateCompliance(ctx, params, msg.Metrics)
	compliant := result.IsCompliant()
	allIssues := result.AllIssues()

	// Persist the latest structured metrics in the dedicated metrics state table.
	// Any report updates the metrics height/report count; UNKNOWN values are
	// handled by compliance rules (e.g. ports UNKNOWN do not trigger issues).
	var reportCount uint64
	if existing, ok := m.GetMetricsState(ctx, valAddr); ok {
		reportCount = existing.ReportCount
	}
	reportCount++

	metricsState := types.SupernodeMetricsState{
		ValidatorAddress: sn.ValidatorAddress,
		Metrics:          &msg.Metrics,
		ReportCount:      reportCount,
		Height:           ctx.BlockHeight(),
	}
	if err := m.SetMetricsState(ctx, metricsState); err != nil {
		return nil, err
	}

	// State transition handling with compliance bifurcation:
	//
	// - No issues and no storage full → fully compliant → recover from any degraded state
	// - Has other issues (regardless of storage) → POSTPONED (most restrictive wins)
	// - Storage full ONLY (no other issues) → STORAGE_FULL (compute-eligible)
	//
	// State diagram:
	//   [ACTIVE] ──storage full only──> [STORAGE_FULL]
	//   [ACTIVE] ──other issues──> [POSTPONED]
	//   [STORAGE_FULL] ──storage freed, no issues──> [ACTIVE]
	//   [STORAGE_FULL] ──other issues added──> [POSTPONED]
	//   [POSTPONED] ──all clear──> [last non-degraded state]
	//   [POSTPONED] ──storage only──> [STORAGE_FULL] (improvement from POSTPONED)
	stateChanged := false
	if len(sn.States) > 0 {
		lastState := sn.States[len(sn.States)-1].State
		hasOtherIssues := len(result.Issues) > 0

		if !hasOtherIssues && !result.StorageFull {
			// Fully compliant: recover from any degraded state.
			if lastState == types.SuperNodeStatePostponed {
				target := lastNonDegradedState(sn.States)
				if err := recoverFromPostponed(ctx, m.SupernodeKeeper, &sn, target); err != nil {
					return nil, err
				}
				stateChanged = true
			} else if lastState == types.SuperNodeStateStorageFull {
				target := lastNonDegradedState(sn.States)
				if err := recoverFromStorageFull(ctx, m.SupernodeKeeper, &sn, target); err != nil {
					return nil, err
				}
				stateChanged = true
			}
		} else if hasOtherIssues {
			// Has non-storage issues: POSTPONED always (most restrictive).
			if lastState != types.SuperNodeStatePostponed {
				if err := markPostponed(ctx, m.SupernodeKeeper, &sn, strings.Join(allIssues, ";")); err != nil {
					return nil, err
				}
				stateChanged = true
			}
		} else {
			// Storage full only, no other issues.
			if lastState == types.SuperNodeStatePostponed {
				// Improvement: was POSTPONED, now only storage issue → STORAGE_FULL.
				if err := markStorageFull(ctx, m.SupernodeKeeper, &sn); err != nil {
					return nil, err
				}
				stateChanged = true
			} else if lastState != types.SuperNodeStateStorageFull {
				if err := markStorageFull(ctx, m.SupernodeKeeper, &sn); err != nil {
					return nil, err
				}
				stateChanged = true
			}
		}
	}

	if !stateChanged {
		if err := m.SetSuperNode(ctx, sn); err != nil {
			return nil, err
		}
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeMetricsReported,
			sdk.NewAttribute(types.AttributeKeyValidatorAddress, msg.ValidatorAddress),
			sdk.NewAttribute(types.AttributeKeySupernodeAccount, msg.SupernodeAccount),
			sdk.NewAttribute(types.AttributeKeyCompliant, boolToString(compliant)),
			sdk.NewAttribute(types.AttributeKeyIssues, strings.Join(allIssues, ";")),
			sdk.NewAttribute(types.AttributeKeyHeight, stringHeight(ctx.BlockHeight())),
		),
	)

	return &types.MsgReportSupernodeMetricsResponse{Compliant: compliant, Issues: allIssues}, nil
}

func boolToString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
