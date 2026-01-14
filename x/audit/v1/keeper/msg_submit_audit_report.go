package keeper

import (
	"context"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

func (m msgServer) SubmitAuditReport(ctx context.Context, req *types.MsgSubmitAuditReport) (*types.MsgSubmitAuditReportResponse, error) {
	if req == nil {
		return nil, errorsmod.Wrap(types.ErrInvalidSigner, "empty request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := m.GetParams(ctx).WithDefaults()
	origin := m.getOrInitWindowOriginHeight(sdkCtx)

	// Enforce window_id acceptance: allow submitting for a window until end+grace.
	windowStart := m.windowStartHeight(origin, params, req.WindowId)
	windowEnd := m.windowEndHeight(origin, params, req.WindowId)
	graceEnd := windowEnd + int64(params.MissingReportGraceBlocks)
	if sdkCtx.BlockHeight() < windowStart || sdkCtx.BlockHeight() > graceEnd {
		return nil, errorsmod.Wrapf(types.ErrInvalidWindowID, "window_id not accepted at height %d", sdkCtx.BlockHeight())
	}

	sn, found, err := m.supernodeKeeper.GetSuperNodeByAccount(sdkCtx, req.SupernodeAccount)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errorsmod.Wrap(types.ErrReporterNotFound, "unknown supernode_account")
	}

	reporterValidator := sn.ValidatorAddress
	if m.HasReport(sdkCtx, req.WindowId, reporterValidator) {
		return nil, errorsmod.Wrap(types.ErrDuplicateReport, "report already submitted for this window")
	}

	// Ensure the snapshot exists for deterministic assignment checks.
	snap, hasSnap := m.GetWindowSnapshot(sdkCtx, req.WindowId)
	if !hasSnap {
		return nil, errorsmod.Wrap(types.ErrWindowSnapshotNotFound, "missing window snapshot")
	}

	requiredPorts := params.RequiredOpenPorts
	requiredPortsCount := len(requiredPorts)
	for _, obs := range req.PeerObservations {
		if len(obs.PortStates) != requiredPortsCount {
			return nil, errorsmod.Wrapf(types.ErrInvalidPortStatesLength, "expected %d port states", requiredPortsCount)
		}
	}

	// Reporter state gating.
	lastState := sntypes.SuperNodeStateUnspecified
	if len(sn.States) > 0 {
		lastState = sn.States[len(sn.States)-1].State
	}

	switch lastState {
	case sntypes.SuperNodeStateActive:
		// Validate deterministic assignment only if the reporter is in the snapshot senders list.
		expectedTargets, ok := assignedTargets(snap, reporterValidator)
		if !ok {
			// ACTIVE but not a sender for this window snapshot: reject peer observations.
			if len(req.PeerObservations) != 0 {
				return nil, errorsmod.Wrap(types.ErrInvalidPeerObservations, "reporter not a sender for this window")
			}
			break
		}

		if len(req.PeerObservations) != len(expectedTargets) {
			return nil, errorsmod.Wrapf(types.ErrInvalidPeerObservations, "expected %d peer observations", len(expectedTargets))
		}
		for i := range expectedTargets {
			if req.PeerObservations[i].TargetValidatorAddress != expectedTargets[i] {
				return nil, errorsmod.Wrap(types.ErrInvalidPeerObservations, "peer observations must match assigned targets (order-sensitive)")
			}
		}
	case sntypes.SuperNodeStatePostponed:
		if len(req.PeerObservations) != 0 {
			return nil, errorsmod.Wrap(types.ErrInvalidPeerObservations, "postponed reporters must not submit peer observations")
		}
	default:
		return nil, errorsmod.Wrap(types.ErrInvalidReporterState, fmt.Sprintf("unsupported reporter state: %s", lastState.String()))
	}

	report := types.AuditReport{
		ReporterValidatorAddress: reporterValidator,
		SupernodeAccount:         req.SupernodeAccount,
		WindowId:                 req.WindowId,
		ReportHeight:             sdkCtx.BlockHeight(),
		SelfReport:               req.SelfReport,
		PeerObservations:         req.PeerObservations,
	}

	if err := m.SetReport(sdkCtx, report); err != nil {
		return nil, err
	}

	// Update reporter status (self-report tracking).
	status, ok := m.GetAuditStatus(sdkCtx, reporterValidator)
	if !ok {
		status = types.AuditStatus{
			ValidatorAddress: reporterValidator,
		}
	}
	status.LastReportedWindowId = req.WindowId
	status.LastReportHeight = sdkCtx.BlockHeight()
	status.Compliant = true
	status.Reasons = nil
	if err := m.SetAuditStatus(sdkCtx, status); err != nil {
		return nil, err
	}

	// Update evidence aggregates for targets.
	for _, obs := range req.PeerObservations {
		for portIdx, state := range obs.PortStates {
			if state == types.PortState_PORT_STATE_UNKNOWN {
				continue
			}

			idx := uint32(portIdx)
			agg, found := m.GetEvidenceAggregate(sdkCtx, req.WindowId, obs.TargetValidatorAddress, idx)
			if !found {
				agg = types.PortEvidenceAggregate{
					Count:      0,
					FirstState: state,
					Conflict:   false,
				}
			}

			agg.Count++
			if agg.Count == 1 {
				agg.FirstState = state
			} else if state != agg.FirstState {
				agg.Conflict = true
			}

			if err := m.SetEvidenceAggregate(sdkCtx, req.WindowId, obs.TargetValidatorAddress, idx, agg); err != nil {
				return nil, err
			}

			consensus := consensusFromAggregate(agg, params.PeerQuorumReports)
			if err := m.setRequiredPortsState(sdkCtx, obs.TargetValidatorAddress, requiredPortsCount, idx, consensus); err != nil {
				return nil, err
			}
		}
	}

	return &types.MsgSubmitAuditReportResponse{}, nil
}

