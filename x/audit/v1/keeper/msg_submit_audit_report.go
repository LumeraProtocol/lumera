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

	// Validate epoch_id acceptance: only the current epoch_id is accepted at the current height.
	epoch, err := deriveEpochAtHeight(sdkCtx.BlockHeight(), params)
	if err != nil {
		return nil, err
	}
	if req.EpochId != epoch.EpochID {
		return nil, errorsmod.Wrapf(types.ErrInvalidEpochID, "epoch_id %d not accepted at height %d", req.EpochId, sdkCtx.BlockHeight())
	}
	if sdkCtx.BlockHeight() < epoch.StartHeight || sdkCtx.BlockHeight() > epoch.EndHeight {
		return nil, errorsmod.Wrapf(types.ErrInvalidEpochID, "epoch_id not accepted at height %d", sdkCtx.BlockHeight())
	}

	_, found, err := m.supernodeKeeper.GetSuperNodeByAccount(sdkCtx, req.SupernodeAccount)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errorsmod.Wrap(types.ErrReporterNotFound, "unknown supernode_account")
	}

	anchor, found := m.GetEpochAnchor(sdkCtx, req.EpochId)
	if !found {
		return nil, errorsmod.Wrapf(types.ErrInvalidEpochID, "epoch anchor not found for epoch_id %d", req.EpochId)
	}

	allowedTargetsList, isProber, err := computeAuditPeerTargetsForReporter(&params, anchor.ActiveSupernodeAccounts, anchor.TargetSupernodeAccounts, anchor.Seed, req.SupernodeAccount)
	if err != nil {
		return nil, err
	}
	allowedTargets := make(map[string]struct{}, len(allowedTargetsList))
	for _, t := range allowedTargetsList {
		allowedTargets[t] = struct{}{}
	}

	requiredPortsLen := len(params.RequiredOpenPorts)
	// Self report port states are persisted on-chain. To prevent state bloat and keep the
	// semantics clear, allow either:
	// - an empty list (unknown/unreported), or
	// - a full list matching required_open_ports length (same ordering).
	if len(req.SelfReport.InboundPortStates) > requiredPortsLen {
		return nil, errorsmod.Wrapf(
			types.ErrInvalidPortStatesLength,
			"inbound_port_states length %d exceeds required_open_ports length %d",
			len(req.SelfReport.InboundPortStates), requiredPortsLen,
		)
	}
	if len(req.SelfReport.InboundPortStates) != 0 && len(req.SelfReport.InboundPortStates) != requiredPortsLen {
		return nil, errorsmod.Wrapf(
			types.ErrInvalidPortStatesLength,
			"inbound_port_states length %d must be 0 or %d",
			len(req.SelfReport.InboundPortStates), requiredPortsLen,
		)
	}
	if !isProber {
		// Not a prober for this epoch (e.g. POSTPONED). Peer observations are not accepted.
		if len(req.PeerObservations) > 0 {
			return nil, errorsmod.Wrap(types.ErrInvalidReporterState, "reporter not eligible for peer observations in this epoch")
		}
	} else {
		// Probers must submit peer observations for all assigned targets for the epoch.
		if len(req.PeerObservations) != len(allowedTargets) {
			return nil, errorsmod.Wrapf(types.ErrInvalidPeerObservations, "expected peer observations for %d assigned targets; got %d", len(allowedTargets), len(req.PeerObservations))
		}

		seenTargets := make(map[string]struct{}, len(req.PeerObservations))
		for _, obs := range req.PeerObservations {
			if obs == nil {
				return nil, errorsmod.Wrap(types.ErrInvalidPeerObservations, "nil peer observation")
			}
			target := obs.TargetSupernodeAccount
			if target == "" {
				return nil, errorsmod.Wrap(types.ErrInvalidPeerObservations, "empty target_supernode_account")
			}
			if target == req.SupernodeAccount {
				return nil, errorsmod.Wrap(types.ErrInvalidPeerObservations, "self-targeting is not allowed")
			}
			if _, ok := allowedTargets[target]; !ok {
				return nil, errorsmod.Wrapf(types.ErrInvalidPeerObservations, "target %q is not assigned to reporter in this epoch", target)
			}
			if _, dup := seenTargets[target]; dup {
				return nil, errorsmod.Wrapf(types.ErrInvalidPeerObservations, "duplicate peer observation for target %q", target)
			}
			seenTargets[target] = struct{}{}

			if requiredPortsLen != 0 && len(obs.PortStates) != requiredPortsLen {
				return nil, errorsmod.Wrapf(types.ErrInvalidPortStatesLength, "port_states length %d does not match required_open_ports length %d", len(obs.PortStates), requiredPortsLen)
			}
		}
		if len(seenTargets) != len(allowedTargets) {
			return nil, errorsmod.Wrap(types.ErrInvalidPeerObservations, "peer observations do not cover all assigned targets")
		}
	}

	reporterAccount := req.SupernodeAccount
	if m.HasReport(sdkCtx, req.EpochId, reporterAccount) {
		return nil, errorsmod.Wrap(types.ErrDuplicateReport, "report already submitted for this epoch")
	}

	report := types.AuditReport{
		SupernodeAccount: reporterAccount,
		EpochId:          req.EpochId,
		ReportHeight:     sdkCtx.BlockHeight(),
		SelfReport:       req.SelfReport,
		PeerObservations: req.PeerObservations,
	}

	if err := m.SetReport(sdkCtx, report); err != nil {
		return nil, err
	}
	m.SetReportIndex(sdkCtx, req.EpochId, reporterAccount)
	m.SetSelfReportIndex(sdkCtx, req.EpochId, reporterAccount)

	seenSupernodes := make(map[string]struct{}, len(req.PeerObservations))
	for _, obs := range req.PeerObservations {
		if obs == nil {
			continue
		}
		supernodeAccount := obs.TargetSupernodeAccount
		if supernodeAccount == "" {
			continue
		}
		if _, seen := seenSupernodes[supernodeAccount]; seen {
			continue
		}
		seenSupernodes[supernodeAccount] = struct{}{}
		m.SetSupernodeReportIndex(sdkCtx, supernodeAccount, req.EpochId, reporterAccount)
	}

	return &types.MsgSubmitAuditReportResponse{}, nil
}
