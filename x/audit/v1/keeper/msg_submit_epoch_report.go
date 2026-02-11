package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func (m msgServer) SubmitEpochReport(ctx context.Context, req *types.MsgSubmitEpochReport) (*types.MsgSubmitEpochReportResponse, error) {
	if req == nil {
		return nil, errorsmod.Wrap(types.ErrInvalidSigner, "empty request")
	}
	if req.Creator == "" {
		return nil, errorsmod.Wrap(types.ErrInvalidSigner, "creator is required")
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

	_, found, err := m.supernodeKeeper.GetSuperNodeByAccount(sdkCtx, req.Creator)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errorsmod.Wrap(types.ErrReporterNotFound, "creator is not a registered supernode")
	}

	anchor, found := m.GetEpochAnchor(sdkCtx, req.EpochId)
	if !found {
		return nil, errorsmod.Wrapf(types.ErrInvalidEpochID, "epoch anchor not found for epoch_id %d", req.EpochId)
	}

	reporterAccount := req.Creator

	// Keep assignment/gating stable within the epoch by using the params snapshot captured
	// at epoch start (when available). Fallback to current params for backward compatibility.
	assignParams := params
	if snap, ok := m.GetEpochParamsSnapshot(sdkCtx, req.EpochId); ok {
		assignParams = snap.WithDefaults()
	}

	allowedTargetsList, isProber, err := computeAuditPeerTargetsForReporter(&assignParams, anchor.ActiveSupernodeAccounts, anchor.TargetSupernodeAccounts, anchor.Seed, reporterAccount)
	if err != nil {
		return nil, err
	}
	allowedTargets := make(map[string]struct{}, len(allowedTargetsList))
	for _, t := range allowedTargetsList {
		allowedTargets[t] = struct{}{}
	}

	requiredPortsLen := len(assignParams.RequiredOpenPorts)
	// Host report port states are persisted on-chain. To prevent state bloat and keep the
	// semantics clear, allow either:
	// - an empty list (unknown/unreported), or
	// - a full list matching required_open_ports length (same ordering).
	if len(req.HostReport.InboundPortStates) > requiredPortsLen {
		return nil, errorsmod.Wrapf(
			types.ErrInvalidPortStatesLength,
			"inbound_port_states length %d exceeds required_open_ports length %d",
			len(req.HostReport.InboundPortStates), requiredPortsLen,
		)
	}
	if len(req.HostReport.InboundPortStates) != 0 && len(req.HostReport.InboundPortStates) != requiredPortsLen {
		return nil, errorsmod.Wrapf(
			types.ErrInvalidPortStatesLength,
			"inbound_port_states length %d must be 0 or %d",
			len(req.HostReport.InboundPortStates), requiredPortsLen,
		)
	}
	if !isProber {
		// Not a prober for this epoch (e.g. POSTPONED). Peer observations are not accepted.
		if len(req.StorageChallengeObservations) > 0 {
			return nil, errorsmod.Wrap(types.ErrInvalidReporterState, "reporter not eligible for storage challenge observations in this epoch")
		}
	} else {
		// Probers must submit peer observations for all assigned targets for the epoch.
		if len(req.StorageChallengeObservations) != len(allowedTargets) {
			return nil, errorsmod.Wrapf(types.ErrInvalidPeerObservations, "expected storage challenge observations for %d assigned targets; got %d", len(allowedTargets), len(req.StorageChallengeObservations))
		}

		seenTargets := make(map[string]struct{}, len(req.StorageChallengeObservations))
		for _, obs := range req.StorageChallengeObservations {
			if obs == nil {
				return nil, errorsmod.Wrap(types.ErrInvalidPeerObservations, "nil storage challenge observation")
			}
			target := obs.TargetSupernodeAccount
			if target == "" {
				return nil, errorsmod.Wrap(types.ErrInvalidPeerObservations, "empty target_supernode_account")
			}
			if target == reporterAccount {
				return nil, errorsmod.Wrap(types.ErrInvalidPeerObservations, "self-targeting is not allowed")
			}
			if _, ok := allowedTargets[target]; !ok {
				return nil, errorsmod.Wrapf(types.ErrInvalidPeerObservations, "target %q is not assigned to reporter in this epoch", target)
			}
			if _, dup := seenTargets[target]; dup {
				return nil, errorsmod.Wrapf(types.ErrInvalidPeerObservations, "duplicate storage challenge observation for target %q", target)
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

	if m.HasReport(sdkCtx, req.EpochId, reporterAccount) {
		return nil, errorsmod.Wrap(types.ErrDuplicateReport, "report already submitted for this epoch")
	}

	report := types.EpochReport{
		SupernodeAccount:             reporterAccount,
		EpochId:                      req.EpochId,
		ReportHeight:                 sdkCtx.BlockHeight(),
		HostReport:                   req.HostReport,
		StorageChallengeObservations: req.StorageChallengeObservations,
	}

	if err := m.SetReport(sdkCtx, report); err != nil {
		return nil, err
	}
	m.SetReportIndex(sdkCtx, req.EpochId, reporterAccount)
	m.SetHostReportIndex(sdkCtx, req.EpochId, reporterAccount)

	seenSupernodes := make(map[string]struct{}, len(req.StorageChallengeObservations))
	for _, obs := range req.StorageChallengeObservations {
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
		m.SetStorageChallengeReportIndex(sdkCtx, supernodeAccount, req.EpochId, reporterAccount)
	}

	return &types.MsgSubmitEpochReportResponse{}, nil
}
