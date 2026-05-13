package keeper

import (
	"context"
	"math"
	"strconv"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

const (
	incompleteReportReason                     = "INCOMPLETE_REPORT"
	incompleteReportReporterReliabilityPenalty = int64(8)
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

	sn, found, err := m.supernodeKeeper.GetSuperNodeByAccount(sdkCtx, req.Creator)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errorsmod.Wrap(types.ErrReporterNotFound, "creator is not a registered supernode")
	}

	// Validate self-reported HostReport host-metric fields. LEP-6 §12 left this
	// field as a metric-courier (no audit-side consensus meaning); the audit
	// handler still rejects malformed values defensively before bridging them
	// into x/supernode metrics state.
	if err := validateHostMetricFields(req.HostReport); err != nil {
		return nil, err
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

	eligibleChallengers := m.storageTruthEligibleChallengers(sdkCtx, anchor.ActiveSupernodeAccounts, req.EpochId, assignParams)
	allowedTargetsList, isProber, err := computeAuditPeerTargetsForReporter(&assignParams, eligibleChallengers, anchor.TargetSupernodeAccounts, anchor.Seed, reporterAccount)
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
	incompleteReport := false
	if !isProber {
		// Not a prober for this epoch (e.g. POSTPONED). Peer observations are not accepted.
		if len(req.StorageChallengeObservations) > 0 {
			return nil, errorsmod.Wrap(types.ErrInvalidReporterState, "reporter not eligible for storage challenge observations in this epoch")
		}
	} else {
		// Probers may submit a subset of assigned peer observations; missing
		// assigned targets are accepted but penalize the reporter once per epoch.
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
		incompleteReport = len(seenTargets) < len(allowedTargets)
	}
	// Per PR #118 / Zee F2 — cap storage proof results to bound processing cost.
	if len(req.StorageProofResults) > types.MaxStorageProofResultsPerReport {
		return nil, errorsmod.Wrapf(types.ErrInvalidStorageProofs,
			"too many storage proof results: got %d, max %d",
			len(req.StorageProofResults), types.MaxStorageProofResultsPerReport)
	}
	enforceCompoundStorageProofs := assignParams.StorageTruthEnforcementMode == types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_FULL
	if err := validateStorageProofResults(reporterAccount, allowedTargets, isProber, enforceCompoundStorageProofs, req.StorageProofResults); err != nil {
		return nil, err
	}
	if err := m.validateStorageProofArtifactCounts(sdkCtx, req.EpochId, assignParams, req.StorageProofResults); err != nil {
		return nil, err
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
		StorageProofResults:          req.StorageProofResults,
	}

	if err := m.SetReport(sdkCtx, report); err != nil {
		return nil, err
	}
	m.SetReportIndex(sdkCtx, req.EpochId, reporterAccount)
	m.SetHostReportIndex(sdkCtx, req.EpochId, reporterAccount)

	// Bridge cascade_kademlia_db_bytes from the audit HostReport into the
	// x/supernode SupernodeMetricsState. Post LEP-6 §12, this is the SOLE
	// writer of that field for Everlight payout / eligibility reads via
	// getLatestCascadeBytesFromAudit. The bridge is read-modify-write so
	// fields owned by the (now operationally dead) legacy
	// MsgReportSupernodeMetrics handler are preserved if previously present.
	if err := m.bridgeCascadeBytesToSupernodeMetrics(sdkCtx, sn, req.HostReport.CascadeKademliaDbBytes); err != nil {
		return nil, err
	}

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

	if err := m.indexStorageProofTranscripts(sdkCtx, req.EpochId, reporterAccount, req.StorageProofResults); err != nil {
		return nil, err
	}

	if err := m.applyStorageTruthScores(sdkCtx, req.EpochId, reporterAccount, req.StorageProofResults); err != nil {
		return nil, err
	}
	if incompleteReport {
		if err := m.applyIncompleteReportPenalty(sdkCtx, req.EpochId, reporterAccount, assignParams); err != nil {
			return nil, err
		}
	}

	return &types.MsgSubmitEpochReportResponse{}, nil
}

func (k Keeper) applyIncompleteReportPenalty(ctx sdk.Context, epochID uint64, reporterAccount string, params types.Params) error {
	state, updated, err := k.applyReporterReliabilityDelta(
		ctx,
		epochID,
		reporterAccount,
		incompleteReportReporterReliabilityPenalty,
		params.StorageTruthReporterReliabilityDecayPerEpoch,
		0,
		params,
	)
	if err != nil {
		return err
	}
	if !updated {
		return nil
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeStorageTruthScoreUpdated,
		sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
		sdk.NewAttribute(types.AttributeKeyEpochID, strconv.FormatUint(epochID, 10)),
		sdk.NewAttribute(types.AttributeKeyReporterSupernodeAccount, reporterAccount),
		sdk.NewAttribute(types.AttributeKeyReporterReliabilityScore, strconv.FormatInt(state.ReliabilityScore, 10)),
		sdk.NewAttribute(types.AttributeKeyReporterTrustBand, state.TrustBand.String()),
		sdk.NewAttribute("reason", incompleteReportReason),
	))
	return nil
}

// validateHostMetricFields rejects malformed host-metric values that the audit
// handler accepts purely as metric-couriers (no audit-side consensus meaning).
// This is the single enforcement point for HostReport host-metric invariants
// (LEP-6 §12 — see proto/lumera/audit/v1/audit.proto::HostReport).
func validateHostMetricFields(h types.HostReport) error {
	if math.IsNaN(h.CascadeKademliaDbBytes) || math.IsInf(h.CascadeKademliaDbBytes, 0) {
		return errorsmod.Wrap(types.ErrInvalidHostMetric, "cascade_kademlia_db_bytes must be a finite number")
	}
	if h.CascadeKademliaDbBytes < 0 {
		return errorsmod.Wrapf(types.ErrInvalidHostMetric, "cascade_kademlia_db_bytes must be >= 0, got %v", h.CascadeKademliaDbBytes)
	}
	return nil
}

// bridgeCascadeBytesToSupernodeMetrics writes the cascade_kademlia_db_bytes
// metric reported on the audit HostReport into x/supernode SupernodeMetricsState.
// Read-modify-write semantics: any other Metrics fields previously persisted
// (e.g. by the legacy MsgReportSupernodeMetrics handler) are preserved.
// Height is updated to the current block; ReportCount is incremented.
//
// Post LEP-6 §12, this is the SOLE writer of SupernodeMetricsState.Metrics.
// CascadeKademliaDbBytes used by Everlight payout / eligibility reads via
// getLatestCascadeBytesFromAudit.
//
// The bridge is defensively a no-op (with an event for observability) if the
// SuperNode record has an empty/invalid ValidatorAddress. That is a pre-existing
// x/supernode invariant violation (chain registration enforces non-empty),
// outside the audit module's scope; the bridge surfaces it via an event but
// does not fail the epoch report on someone else's data corruption.
func (k Keeper) bridgeCascadeBytesToSupernodeMetrics(ctx sdk.Context, sn sntypes.SuperNode, cascadeBytes float64) error {
	if sn.ValidatorAddress == "" {
		ctx.EventManager().EmitEvent(sdk.NewEvent(
			"audit_cascade_bytes_bridge_skipped",
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute("supernode_account", sn.SupernodeAccount),
			sdk.NewAttribute("reason", "empty_validator_address"),
		))
		return nil
	}
	valAddr, err := sdk.ValAddressFromBech32(sn.ValidatorAddress)
	if err != nil {
		ctx.EventManager().EmitEvent(sdk.NewEvent(
			"audit_cascade_bytes_bridge_skipped",
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute("supernode_account", sn.SupernodeAccount),
			sdk.NewAttribute("validator_address", sn.ValidatorAddress),
			sdk.NewAttribute("reason", "invalid_validator_address"),
			sdk.NewAttribute("error", err.Error()),
		))
		return nil
	}

	state, ok := k.supernodeKeeper.GetMetricsState(ctx, valAddr)
	if !ok {
		state = sntypes.SupernodeMetricsState{
			ValidatorAddress: sn.ValidatorAddress,
		}
	}
	if state.Metrics == nil {
		state.Metrics = &sntypes.SupernodeMetrics{}
	}
	state.Metrics.CascadeKademliaDbBytes = cascadeBytes
	state.Height = ctx.BlockHeight()
	state.ReportCount++

	return k.supernodeKeeper.SetMetricsState(ctx, state)
}
