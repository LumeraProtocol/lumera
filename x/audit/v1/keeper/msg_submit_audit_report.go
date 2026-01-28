package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func (m msgServer) SubmitAuditReport(ctx context.Context, req *types.MsgSubmitAuditReport) (*types.MsgSubmitAuditReportResponse, error) {
	if req == nil {
		return nil, errorsmod.Wrap(types.ErrInvalidSigner, "empty request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := m.GetParams(ctx).WithDefaults()

	// Validate window_id acceptance: only the current window_id is accepted at the current height.
	ws, err := m.getCurrentWindowState(sdkCtx, params)
	if err != nil {
		return nil, err
	}
	if req.WindowId != ws.WindowID {
		return nil, errorsmod.Wrapf(types.ErrInvalidWindowID, "window_id %d not accepted at height %d", req.WindowId, sdkCtx.BlockHeight())
	}
	if sdkCtx.BlockHeight() < ws.StartHeight || sdkCtx.BlockHeight() > ws.EndHeight {
		return nil, errorsmod.Wrapf(types.ErrInvalidWindowID, "window_id not accepted at height %d", sdkCtx.BlockHeight())
	}

	_, found, err := m.supernodeKeeper.GetSuperNodeByAccount(sdkCtx, req.SupernodeAccount)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errorsmod.Wrap(types.ErrReporterNotFound, "unknown supernode_account")
	}

	// Enforce peer-observation gating at submission time using the persisted window snapshot.
	// Enforcement later assumes all stored peer observations were gated here.
	snap, found := m.GetWindowSnapshot(sdkCtx, req.WindowId)
	if !found {
		return nil, errorsmod.Wrapf(types.ErrWindowSnapshotNotFound, "window snapshot not found for window_id %d", req.WindowId)
	}

	allowedTargets := make(map[string]struct{})
	for _, a := range snap.Assignments {
		if a.ProberSupernodeAccount != req.SupernodeAccount {
			continue
		}
		for _, t := range a.TargetSupernodeAccounts {
			allowedTargets[t] = struct{}{}
		}
		break
	}

	requiredPortsLen := len(params.RequiredOpenPorts)
	if len(req.PeerObservations) > 0 {
		if len(allowedTargets) == 0 {
			return nil, errorsmod.Wrap(types.ErrInvalidReporterState, "reporter not eligible for peer observations in this window")
		}

		seenTargets := make(map[string]struct{}, len(req.PeerObservations))
		for _, obs := range req.PeerObservations {
			target := obs.TargetSupernodeAccount
			if target == "" {
				return nil, errorsmod.Wrap(types.ErrInvalidPeerObservations, "empty target_supernode_account")
			}
			if target == req.SupernodeAccount {
				return nil, errorsmod.Wrap(types.ErrInvalidPeerObservations, "self-targeting is not allowed")
			}
			if _, ok := allowedTargets[target]; !ok {
				return nil, errorsmod.Wrapf(types.ErrInvalidPeerObservations, "target %q is not assigned to reporter in this window", target)
			}
			if _, dup := seenTargets[target]; dup {
				return nil, errorsmod.Wrapf(types.ErrInvalidPeerObservations, "duplicate peer observation for target %q", target)
			}
			seenTargets[target] = struct{}{}

			if requiredPortsLen != 0 && len(obs.PortStates) != requiredPortsLen {
				return nil, errorsmod.Wrapf(types.ErrInvalidPortStatesLength, "port_states length %d does not match required_open_ports length %d", len(obs.PortStates), requiredPortsLen)
			}
		}
	}

	reporterAccount := req.SupernodeAccount
	if m.HasReport(sdkCtx, req.WindowId, reporterAccount) {
		return nil, errorsmod.Wrap(types.ErrDuplicateReport, "report already submitted for this window")
	}

	report := types.AuditReport{
		SupernodeAccount: reporterAccount,
		WindowId:         req.WindowId,
		ReportHeight:     sdkCtx.BlockHeight(),
		SelfReport:       req.SelfReport,
		PeerObservations: req.PeerObservations,
	}

	if err := m.SetReport(sdkCtx, report); err != nil {
		return nil, err
	}
	m.SetReportIndex(sdkCtx, req.WindowId, reporterAccount)
	m.SetSelfReportIndex(sdkCtx, req.WindowId, reporterAccount)

	seenSupernodes := make(map[string]struct{}, len(req.PeerObservations))
	for _, obs := range req.PeerObservations {
		supernodeAccount := obs.TargetSupernodeAccount
		if supernodeAccount == "" {
			continue
		}
		if _, seen := seenSupernodes[supernodeAccount]; seen {
			continue
		}
		seenSupernodes[supernodeAccount] = struct{}{}
		m.SetSupernodeReportIndex(sdkCtx, supernodeAccount, req.WindowId, reporterAccount)
	}

	return &types.MsgSubmitAuditReportResponse{}, nil
}
