package keeper

import (
	"fmt"
	"strconv"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

const (
	postponeReasonActionFinalizationSignatureFailure = "audit_action_finalization_signature_failure"
	postponeReasonActionFinalizationNotInTop10       = "audit_action_finalization_not_in_top_10"
	postponeReasonStorageTruth                       = "audit_storage_truth_suspicion"
)

// EnforceEpochEnd evaluates the completed epoch and updates supernode states accordingly.
// It does not re-check storage-challenge assignment gating; that must be enforced at MsgSubmitEpochReport time.
func (k Keeper) EnforceEpochEnd(ctx sdk.Context, epochID uint64, params types.Params) error {
	params = params.WithDefaults()

	active, err := k.supernodeKeeper.GetAllSuperNodes(ctx, sntypes.SuperNodeStateActive, sntypes.SuperNodeStateStorageFull)
	if err != nil {
		return err
	}
	postponed, err := k.supernodeKeeper.GetAllSuperNodes(ctx, sntypes.SuperNodeStatePostponed)
	if err != nil {
		return err
	}

	// Postpone ACTIVE supernodes that fail criteria.
	for _, sn := range active {
		if sn.SupernodeAccount == "" {
			continue
		}

		// Emit storage-truth band events (all modes >= SHADOW) and postpone if mode >= SOFT.
		if err := k.applyStorageTruthBandAtEpochEnd(ctx, sn, epochID, params); err != nil {
			return err
		}

		// Skip legacy postpone checks if already postponed by storage-truth enforcement above.
		if _, alreadyStorageTruthPostponed := k.getStorageTruthPostponedAtEpochID(ctx, sn.SupernodeAccount); alreadyStorageTruthPostponed {
			continue
		}

		// Avoid stale action-finalization postponement state if the supernode is ACTIVE.
		k.clearActionFinalizationPostponedAtEpochID(ctx, sn.SupernodeAccount)

		shouldPostpone, reason, err := k.shouldPostponeAtEpochEnd(ctx, sn.SupernodeAccount, epochID, params)
		if err != nil {
			return err
		}
		if !shouldPostpone {
			continue
		}

		if err := k.setSupernodePostponed(ctx, sn, reason); err != nil {
			return err
		}
		switch reason {
		case postponeReasonActionFinalizationSignatureFailure, postponeReasonActionFinalizationNotInTop10:
			k.setActionFinalizationPostponedAtEpochID(ctx, sn.SupernodeAccount, epochID)
		default:
			k.clearActionFinalizationPostponedAtEpochID(ctx, sn.SupernodeAccount)
		}
	}

	// Recover POSTPONED supernodes that meet recovery criteria.
	for _, sn := range postponed {
		if sn.SupernodeAccount == "" {
			continue
		}
		_, storageTruthPostponed := k.getStorageTruthPostponedAtEpochID(ctx, sn.SupernodeAccount)

		shouldRecover, err := k.shouldRecoverAtEpochEnd(ctx, sn.SupernodeAccount, epochID, params)
		if err != nil {
			return err
		}
		if !shouldRecover {
			continue
		}

		if err := k.recoverSupernodeFromPostponed(ctx, sn, epochID); err != nil {
			return err
		}
		k.clearActionFinalizationPostponedAtEpochID(ctx, sn.SupernodeAccount)
		k.clearStorageTruthPostponedAtEpochID(ctx, sn.SupernodeAccount)

		if storageTruthPostponed {
			ctx.EventManager().EmitEvent(sdk.NewEvent(
				types.EventTypeStorageTruthRecovered,
				sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
				sdk.NewAttribute(types.AttributeKeyTargetSupernodeAccount, sn.SupernodeAccount),
				sdk.NewAttribute(types.AttributeKeyEpochID, strconv.FormatUint(epochID, 10)),
			))
		}
	}

	return nil
}

// applyStorageTruthBandAtEpochEnd emits band events for all modes >= SHADOW and
// postpones the node if mode >= SOFT and the suspicion score meets the postpone threshold.
func (k Keeper) applyStorageTruthBandAtEpochEnd(ctx sdk.Context, sn sntypes.SuperNode, epochID uint64, params types.Params) error {
	mode := params.StorageTruthEnforcementMode
	if mode == types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_UNSPECIFIED {
		return nil
	}

	state, found := k.GetNodeSuspicionState(ctx, sn.SupernodeAccount)
	if !found {
		return nil
	}

	score := decayTowardZero(state.SuspicionScore, params.StorageTruthNodeSuspicionDecayPerEpoch, epochDelta(epochID, state.LastUpdatedEpoch))
	if score <= 0 {
		return nil
	}

	band := storageTruthBandForScore(score, params)
	if band == storageTruthBandNone {
		return nil
	}

	// Emit band event.
	eventType := storageTruthBandEventType(band)
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		eventType,
		sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
		sdk.NewAttribute(types.AttributeKeyTargetSupernodeAccount, sn.SupernodeAccount),
		sdk.NewAttribute(types.AttributeKeyEpochID, strconv.FormatUint(epochID, 10)),
		sdk.NewAttribute(types.AttributeKeyNodeSuspicionScore, strconv.FormatInt(score, 10)),
		sdk.NewAttribute(types.AttributeKeyStorageTruthBand, strconv.Itoa(int(band))),
		sdk.NewAttribute(types.AttributeKeyEnforcementMode, mode.String()),
	))

	// SHADOW mode: events only — no state transitions.
	if mode == types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_SHADOW {
		return nil
	}

	// SOFT/FULL: actually postpone when at or above the postpone threshold AND predicates are met.
	if band < storageTruthBandPostpone {
		return nil
	}

	// Check enforcement matrix predicates before postponing.
	if !k.storageTruthPostponePredicatesMet(ctx, sn.SupernodeAccount, band, epochID, params) {
		// Score is above threshold but predicates not met — event already emitted, no postpone.
		return nil
	}

	if err := k.setSupernodePostponed(ctx, sn, postponeReasonStorageTruth); err != nil {
		return err
	}
	k.setStorageTruthPostponedAtEpochID(ctx, sn.SupernodeAccount, epochID)
	k.clearActionFinalizationPostponedAtEpochID(ctx, sn.SupernodeAccount)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeStorageTruthEnforced,
		sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
		sdk.NewAttribute(types.AttributeKeyTargetSupernodeAccount, sn.SupernodeAccount),
		sdk.NewAttribute(types.AttributeKeyEpochID, strconv.FormatUint(epochID, 10)),
		sdk.NewAttribute(types.AttributeKeyNodeSuspicionScore, strconv.FormatInt(score, 10)),
		sdk.NewAttribute(types.AttributeKeyEnforcementMode, mode.String()),
	))
	return nil
}

func (k Keeper) shouldPostponeAtEpochEnd(ctx sdk.Context, supernodeAccount string, epochID uint64, params types.Params) (bool, string, error) {
	// Action finalization evidence-based postponement.
	if shouldPostpone, reason := k.shouldPostponeForActionFinalizationEvidence(ctx, supernodeAccount, epochID, params); shouldPostpone {
		return true, reason, nil
	}

	// Missing-report based postponement.
	consecutive := params.ConsecutiveEpochsToPostpone
	if consecutive == 0 {
		consecutive = 1
	}
	if k.missingReportsForConsecutiveEpochs(ctx, supernodeAccount, epochID, consecutive) {
		return true, "audit_missing_reports", nil
	}

	// Self host-metrics-based postponement (if a self report exists and violates minimums).
	if ok, err := k.selfHostViolatesMinimums(ctx, supernodeAccount, epochID, params); err != nil {
		return false, "", err
	} else if ok {
		return true, "audit_host_requirements", nil
	}

	// Peer ports-based postponement.
	requiredPortsLen := len(params.RequiredOpenPorts)
	if requiredPortsLen == 0 {
		return false, "", nil
	}

	if consecutive > uint32(epochID+1) {
		// Not enough history on-chain to satisfy the consecutive rule.
		return false, "", nil
	}

	for portIndex := 0; portIndex < requiredPortsLen; portIndex++ {
		streak := uint32(0)
		for offset := uint32(0); offset < consecutive; offset++ {
			e := epochID - uint64(offset)
			closed, err := k.peersPortStateMeetsThreshold(ctx, supernodeAccount, e, portIndex, types.PortState_PORT_STATE_CLOSED, params.PeerPortPostponeThresholdPercent)
			if err != nil {
				return false, "", err
			}
			if !closed {
				break
			}
			streak++
		}
		if streak == consecutive {
			return true, "audit_peer_ports", nil
		}
	}

	return false, "", nil
}

func (k Keeper) shouldRecoverAtEpochEnd(ctx sdk.Context, supernodeAccount string, epochID uint64, params types.Params) (bool, error) {
	// If the supernode was postponed due to storage-truth suspicion, use score-decay-based recovery.
	if _, ok := k.getStorageTruthPostponedAtEpochID(ctx, supernodeAccount); ok {
		return k.shouldRecoverFromStorageTruthPostponement(ctx, supernodeAccount, epochID, params), nil
	}

	// If the supernode was postponed due to action-finalization evidence, it recovers using the
	// action-finalization recovery rules (not the host/peer-port recovery rules).
	if postponedAtEpochID, ok := k.getActionFinalizationPostponedAtEpochID(ctx, supernodeAccount); ok {
		return k.shouldRecoverFromActionFinalizationPostponement(ctx, supernodeAccount, epochID, postponedAtEpochID, params), nil
	}

	// Need one compliant host report.
	selfCompliant, err := k.selfHostCompliant(ctx, supernodeAccount, epochID, params)
	if err != nil || !selfCompliant {
		return false, err
	}

	// Need at least one compliant peer report that shows all required ports OPEN.
	requiredPortsLen := len(params.RequiredOpenPorts)
	if requiredPortsLen == 0 {
		return true, nil
	}

	peers, err := k.peerReportersForTargetEpoch(ctx, supernodeAccount, epochID)
	if err != nil {
		return false, err
	}
	if len(peers) == 0 {
		return false, nil
	}

	// Recovery requires at least one peer report that shows all required ports OPEN for this supernode in this epoch.
	for _, reporter := range peers {
		r, found := k.GetReport(ctx, epochID, reporter)
		if !found {
			continue
		}

		var obs *types.StorageChallengeObservation
		for i := range r.StorageChallengeObservations {
			if r.StorageChallengeObservations[i] != nil && r.StorageChallengeObservations[i].TargetSupernodeAccount == supernodeAccount {
				obs = r.StorageChallengeObservations[i]
				break
			}
		}
		if obs == nil {
			continue
		}
		if len(obs.PortStates) != requiredPortsLen {
			continue
		}

		allOpen := true
		for portIndex := 0; portIndex < requiredPortsLen; portIndex++ {
			if obs.PortStates[portIndex] != types.PortState_PORT_STATE_OPEN {
				allOpen = false
				break
			}
		}
		if allOpen {
			return true, nil
		}
	}

	return false, nil
}

func (k Keeper) shouldPostponeForActionFinalizationEvidence(ctx sdk.Context, supernodeAccount string, epochID uint64, params types.Params) (bool, string) {
	if k.evidenceMeetsConsecutiveEpochsThreshold(
		ctx,
		supernodeAccount,
		epochID,
		types.EvidenceType_EVIDENCE_TYPE_ACTION_FINALIZATION_SIGNATURE_FAILURE,
		params.ActionFinalizationSignatureFailureEvidencesPerEpoch,
		params.ActionFinalizationSignatureFailureConsecutiveEpochs,
	) {
		return true, postponeReasonActionFinalizationSignatureFailure
	}

	if k.evidenceMeetsConsecutiveEpochsThreshold(
		ctx,
		supernodeAccount,
		epochID,
		types.EvidenceType_EVIDENCE_TYPE_ACTION_FINALIZATION_NOT_IN_TOP_10,
		params.ActionFinalizationNotInTop10EvidencesPerEpoch,
		params.ActionFinalizationNotInTop10ConsecutiveEpochs,
	) {
		return true, postponeReasonActionFinalizationNotInTop10
	}

	return false, ""
}

func (k Keeper) evidenceMeetsConsecutiveEpochsThreshold(
	ctx sdk.Context,
	supernodeAccount string,
	epochID uint64,
	evidenceType types.EvidenceType,
	minEvidencesPerEpoch uint32,
	consecutiveEpochs uint32,
) bool {
	if minEvidencesPerEpoch == 0 || consecutiveEpochs == 0 {
		return false
	}
	if consecutiveEpochs > uint32(epochID+1) {
		// Not enough history on-chain to satisfy the consecutive rule.
		return false
	}

	streak := uint32(0)
	for offset := uint32(0); offset < consecutiveEpochs; offset++ {
		e := epochID - uint64(offset)
		if k.getEvidenceEpochCount(ctx, e, supernodeAccount, evidenceType) < uint64(minEvidencesPerEpoch) {
			break
		}
		streak++
	}
	return streak == consecutiveEpochs
}

func (k Keeper) shouldRecoverFromActionFinalizationPostponement(
	ctx sdk.Context,
	supernodeAccount string,
	epochID uint64,
	postponedAtEpochID uint64,
	params types.Params,
) bool {
	recoveryEpochs := params.ActionFinalizationRecoveryEpochs
	if recoveryEpochs == 0 {
		recoveryEpochs = 1
	}
	if epochID < postponedAtEpochID+uint64(recoveryEpochs) {
		return false
	}

	var startEpochID uint64
	if epochID+1 > uint64(recoveryEpochs) {
		startEpochID = epochID + 1 - uint64(recoveryEpochs)
	} else {
		startEpochID = 0
	}

	totalBad := uint64(0)
	for e := startEpochID; e <= epochID; e++ {
		totalBad += k.getEvidenceEpochCount(ctx, e, supernodeAccount, types.EvidenceType_EVIDENCE_TYPE_ACTION_FINALIZATION_SIGNATURE_FAILURE)
		totalBad += k.getEvidenceEpochCount(ctx, e, supernodeAccount, types.EvidenceType_EVIDENCE_TYPE_ACTION_FINALIZATION_NOT_IN_TOP_10)
	}

	maxTotal := params.ActionFinalizationRecoveryMaxTotalBadEvidences
	if maxTotal == 0 {
		maxTotal = 1
	}
	return totalBad < uint64(maxTotal)
}

func (k Keeper) selfHostViolatesMinimums(ctx sdk.Context, supernodeAccount string, epochID uint64, params types.Params) (bool, error) {
	r, found := k.GetReport(ctx, epochID, supernodeAccount)
	if !found {
		return false, nil
	}

	// If any known non-storage metric is below minimum free%, postpone.
	// Disk pressure is modeled via STORAGE_FULL transitions, not POSTPONED.
	if violatesMinFree(r.HostReport.CpuUsagePercent, params.MinCpuFreePercent) {
		return true, nil
	}
	if violatesMinFree(r.HostReport.MemUsagePercent, params.MinMemFreePercent) {
		return true, nil
	}

	return false, nil
}

func (k Keeper) selfHostCompliant(ctx sdk.Context, supernodeAccount string, epochID uint64, params types.Params) (bool, error) {
	r, found := k.GetReport(ctx, epochID, supernodeAccount)
	if !found {
		return false, nil
	}

	if !compliesMinFree(r.HostReport.CpuUsagePercent, params.MinCpuFreePercent) {
		return false, nil
	}
	if !compliesMinFree(r.HostReport.MemUsagePercent, params.MinMemFreePercent) {
		return false, nil
	}

	return true, nil
}

func violatesMinFree(usagePercent float64, minFreePercent uint32) bool {
	if minFreePercent == 0 {
		return false
	}
	if usagePercent == 0 {
		// Unknown: skip action.
		return false
	}
	if usagePercent < 0 || usagePercent > 100 {
		return true
	}
	free := 100.0 - usagePercent
	return free < float64(minFreePercent)
}

func compliesMinFree(usagePercent float64, minFreePercent uint32) bool {
	if minFreePercent == 0 {
		return true
	}
	if usagePercent == 0 {
		// Unknown: does not block compliance.
		return true
	}
	if usagePercent < 0 || usagePercent > 100 {
		return false
	}
	free := 100.0 - usagePercent
	return free >= float64(minFreePercent)
}

func (k Keeper) peersPortStateMeetsThreshold(ctx sdk.Context, target string, epochID uint64, portIndex int, desired types.PortState, thresholdPercent uint32) (bool, error) {
	peers, err := k.peerReportersForTargetEpoch(ctx, target, epochID)
	if err != nil {
		return false, err
	}
	if len(peers) == 0 {
		return false, nil
	}
	return k.peersPortStateMeetsThresholdWithPeers(ctx, target, epochID, portIndex, desired, thresholdPercent, peers)
}

func (k Keeper) peersPortStateMeetsThresholdWithPeers(ctx sdk.Context, target string, epochID uint64, portIndex int, desired types.PortState, thresholdPercent uint32, peers []string) (bool, error) {
	if len(peers) == 0 {
		return false, nil
	}

	matches := uint64(0)
	for _, reporter := range peers {
		r, found := k.GetReport(ctx, epochID, reporter)
		if !found {
			return false, nil
		}

		var obs *types.StorageChallengeObservation
		for i := range r.StorageChallengeObservations {
			if r.StorageChallengeObservations[i] != nil && r.StorageChallengeObservations[i].TargetSupernodeAccount == target {
				obs = r.StorageChallengeObservations[i]
				break
			}
		}
		if obs == nil {
			return false, nil
		}
		if portIndex < 0 || portIndex >= len(obs.PortStates) {
			return false, nil
		}
		if obs.PortStates[portIndex] == desired {
			matches++
		}
	}

	// Require a minimum share of matching peer observations.
	total := uint64(len(peers))
	return matches*100 >= uint64(thresholdPercent)*total, nil
}

func (k Keeper) peerReportersForTargetEpoch(ctx sdk.Context, target string, epochID uint64) ([]string, error) {
	store := k.kvStore(ctx)
	prefix := types.StorageChallengeReportIndexEpochPrefix(target, epochID)

	it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer it.Close()

	reporters := make([]string, 0, 8)
	for ; it.Valid(); it.Next() {
		// Key is "<reporter_supernode_account>" under the epoch-specific prefix.
		key := it.Key()
		if len(key) < len(prefix) {
			return nil, fmt.Errorf("invalid supernode report index key")
		}
		reporter := string(key[len(prefix):])
		if reporter == "" {
			return nil, fmt.Errorf("empty reporter in supernode report index")
		}
		reporters = append(reporters, reporter)
	}
	return reporters, nil
}

func (k Keeper) setSupernodePostponed(ctx sdk.Context, sn sntypes.SuperNode, reason string) error {
	if sn.ValidatorAddress == "" {
		return fmt.Errorf("missing validator address for supernode %q", sn.SupernodeAccount)
	}
	valAddr, err := sdk.ValAddressFromBech32(sn.ValidatorAddress)
	if err != nil {
		return err
	}
	return k.supernodeKeeper.SetSuperNodePostponed(ctx, valAddr, reason)
}

func (k Keeper) recoverSupernodeFromPostponed(ctx sdk.Context, sn sntypes.SuperNode, epochID uint64) error {
	if sn.ValidatorAddress == "" {
		return fmt.Errorf("missing validator address for supernode %q", sn.SupernodeAccount)
	}
	valAddr, err := sdk.ValAddressFromBech32(sn.ValidatorAddress)
	if err != nil {
		return err
	}

	target := sntypes.SuperNodeStateActive
	if report, found := k.GetReport(ctx, epochID, sn.SupernodeAccount); found {
		maxStorage := float64(k.supernodeKeeper.GetParams(ctx).MaxStorageUsagePercent)
		if report.HostReport.DiskUsagePercent > maxStorage {
			target = sntypes.SuperNodeStateStorageFull
		}
	}

	if target == sntypes.SuperNodeStateActive {
		return k.supernodeKeeper.RecoverSuperNodeFromPostponed(ctx, valAddr)
	}

	current, found := k.supernodeKeeper.QuerySuperNode(ctx, valAddr)
	if !found {
		return fmt.Errorf("supernode not found for validator %q", sn.ValidatorAddress)
	}
	if len(current.States) == 0 {
		return fmt.Errorf("supernode state history missing for validator %q", sn.ValidatorAddress)
	}
	if current.States[len(current.States)-1].State != sntypes.SuperNodeStatePostponed {
		return nil
	}

	current.States = append(current.States, &sntypes.SuperNodeStateRecord{State: sntypes.SuperNodeStateStorageFull, Height: ctx.BlockHeight()})
	return k.supernodeKeeper.SetSuperNode(ctx, current)
}

// storageTruthBand represents a node suspicion severity level.
type storageTruthBand int

const (
	storageTruthBandNone           storageTruthBand = iota // score < watch threshold
	storageTruthBandWatch                                  // score >= watch threshold
	storageTruthBandProbation                              // score >= probation threshold
	storageTruthBandPostpone                               // score >= postpone threshold
	storageTruthBandStrongPostpone                         // score >= strong_postpone threshold
)

func storageTruthBandForScore(score int64, params types.Params) storageTruthBand {
	switch {
	case params.StorageTruthNodeSuspicionThresholdStrongPostpone > 0 && score >= params.StorageTruthNodeSuspicionThresholdStrongPostpone:
		return storageTruthBandStrongPostpone
	case params.StorageTruthNodeSuspicionThresholdPostpone > 0 && score >= params.StorageTruthNodeSuspicionThresholdPostpone:
		return storageTruthBandPostpone
	case params.StorageTruthNodeSuspicionThresholdProbation > 0 && score >= params.StorageTruthNodeSuspicionThresholdProbation:
		return storageTruthBandProbation
	case params.StorageTruthNodeSuspicionThresholdWatch > 0 && score >= params.StorageTruthNodeSuspicionThresholdWatch:
		return storageTruthBandWatch
	default:
		return storageTruthBandNone
	}
}

func storageTruthBandEventType(band storageTruthBand) string {
	switch band {
	case storageTruthBandStrongPostpone:
		return types.EventTypeStorageTruthBandPostpone // reuse postpone event type for strong postpone
	case storageTruthBandPostpone:
		return types.EventTypeStorageTruthBandPostpone
	case storageTruthBandProbation:
		return types.EventTypeStorageTruthBandProbation
	default:
		return types.EventTypeStorageTruthBandWatch
	}
}

// shouldRecoverFromStorageTruthPostponement returns true if the node's current
// (decayed) suspicion score has fallen below the watch threshold AND the node
// has accumulated the required number of clean passes.
func (k Keeper) shouldRecoverFromStorageTruthPostponement(ctx sdk.Context, supernodeAccount string, epochID uint64, params types.Params) bool {
	state, found := k.GetNodeSuspicionState(ctx, supernodeAccount)
	if !found {
		// No score state means no suspicion — allow recovery (no clean pass requirement without state).
		return true
	}
	score := decayTowardZero(state.SuspicionScore, params.StorageTruthNodeSuspicionDecayPerEpoch, epochDelta(epochID, state.LastUpdatedEpoch))
	watchThreshold := params.StorageTruthNodeSuspicionThresholdWatch
	if watchThreshold <= 0 {
		watchThreshold = 1
	}
	if score >= watchThreshold {
		return false
	}
	// Score is below watch threshold — also require sufficient clean passes.
	requiredPasses := params.StorageTruthRecoveryCleanPassCount
	if requiredPasses == 0 {
		requiredPasses = 3
	}
	if state.CleanPassCount < requiredPasses {
		return false
	}
	// Recovery additionally requires no new Class A failure after the clean-pass streak starts.
	if state.LastClassAEpoch != 0 && state.LastCleanPassEpoch <= state.LastClassAEpoch {
		return false
	}
	return true
}

// storageTruthPostponePredicatesMet checks whether the enforcement matrix predicates
// are satisfied for the given band (postpone or strong-postpone).
func (k Keeper) storageTruthPostponePredicatesMet(ctx sdk.Context, supernodeAccount string, band storageTruthBand, epochID uint64, params types.Params) bool {
	state, found := k.GetNodeSuspicionState(ctx, supernodeAccount)
	if !found {
		return false
	}

	switch band {
	case storageTruthBandPostpone:
		// Postpone predicates per LEP6.md §17 — any one of three conditions:
		// 1. 1 recent Class A fault plus any second failure in 14 epochs.
		classAWindow := uint64(params.StorageTruthClassAFaultWindow)
		if classAWindow == 0 {
			classAWindow = 14
		}
		classBWindow := uint64(params.StorageTruthClassBFaultWindow)
		if classBWindow == 0 {
			classBWindow = 7
		}
		recentClassA, err := k.hasNodeFailure(ctx, supernodeAccount, storageTruthWindowStart(epochID, classAWindow), epochID, func(record storageTruthNodeFailureRecord) bool {
			return types.StorageProofBucketType(record.BucketType) == types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT && storageTruthIsClassAFault(record)
		})
		if err != nil {
			return false
		}
		_, secondFailureEvents, err := k.distinctNodeFailedTickets(ctx, supernodeAccount, storageTruthWindowStart(epochID, classAWindow), epochID, nil)
		if err != nil {
			return false
		}
		classAMet := recentClassA && secondFailureEvents >= 2
		if secondFailureEvents == 0 {
			classAMet = state.ClassACountWindow >= 1 && (state.ClassACountWindow+state.ClassBCountWindow) >= 2
		}
		// 2. 4 Class B faults in 7 epochs.
		_, classBEvents, err := k.distinctNodeFailedTickets(ctx, supernodeAccount, storageTruthWindowStart(epochID, classBWindow), epochID, func(record storageTruthNodeFailureRecord) bool {
			return storageTruthIsClassBFault(record)
		})
		if err != nil {
			return false
		}
		classBMet := classBEvents >= 4
		if classBEvents == 0 {
			classBMet = state.ClassBCountWindow >= 4
		}
		// 3. 2 old Class A faults on distinct tickets in the configured old-Class-A window.
		oldClassAFaultWindow := uint64(params.StorageTruthOldClassAFaultWindow)
		if oldClassAFaultWindow == 0 {
			oldClassAFaultWindow = 21
		}
		oldClassATickets, _, err := k.distinctNodeFailedTickets(ctx, supernodeAccount, storageTruthWindowStart(epochID, oldClassAFaultWindow), epochID, func(record storageTruthNodeFailureRecord) bool {
			return types.StorageProofBucketType(record.BucketType) == types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_OLD && storageTruthIsClassAFault(record)
		})
		if err != nil {
			return false
		}
		oldClassAMet := len(oldClassATickets) >= 2
		if len(oldClassATickets) == 0 {
			oldClassAMet = state.ClassACountWindow >= 2 && state.LastOldFailEpoch > 0 && epochDelta(epochID, state.LastOldFailEpoch) < oldClassAFaultWindow
		}
		return classAMet || classBMet || oldClassAMet

	case storageTruthBandStrongPostpone:
		classAWindow := uint64(params.StorageTruthClassAFaultWindow)
		if classAWindow == 0 {
			classAWindow = 14
		}
		classATickets, _, err := k.distinctNodeFailedTickets(ctx, supernodeAccount, storageTruthWindowStart(epochID, classAWindow), epochID, func(record storageTruthNodeFailureRecord) bool {
			return storageTruthIsClassAFault(record)
		})
		if err != nil {
			return false
		}
		indexFailure, err := k.hasNodeFailure(ctx, supernodeAccount, storageTruthWindowStart(epochID, classAWindow), epochID, func(record storageTruthNodeFailureRecord) bool {
			return types.StorageProofArtifactClass(record.ArtifactClass) == types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_INDEX
		})
		if err != nil {
			return false
		}
		classAMet := len(classATickets) >= 2
		if len(classATickets) == 0 {
			classAMet = state.ClassACountWindow >= 2
		}
		indexMet := indexFailure || state.LastIndexFailEpoch > 0
		return classAMet || indexMet || k.hasStorageTruthFailedHeal(ctx, supernodeAccount, storageTruthWindowStart(epochID, classAWindow), epochID)

	default:
		return true
	}
}

func storageTruthIsClassAFault(record storageTruthNodeFailureRecord) bool {
	class := types.StorageProofResultClass(record.ResultClass)
	if class == types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH ||
		class == types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL {
		return true
	}
	return types.StorageProofArtifactClass(record.ArtifactClass) == types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_INDEX
}

func storageTruthIsClassBFault(record storageTruthNodeFailureRecord) bool {
	return types.StorageProofResultClass(record.ResultClass) == types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_TIMEOUT_OR_NO_RESPONSE
}

func (k Keeper) missingReportsForConsecutiveEpochs(ctx sdk.Context, supernodeAccount string, epochID uint64, consecutive uint32) bool {
	if consecutive == 0 {
		consecutive = 1
	}
	if consecutive > uint32(epochID+1) {
		// Not enough history on-chain to satisfy the consecutive rule.
		return false
	}
	for offset := uint32(0); offset < consecutive; offset++ {
		e := epochID - uint64(offset)
		if k.HasReport(ctx, e, supernodeAccount) {
			return false
		}
	}
	return true
}
