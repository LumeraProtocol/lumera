package keeper

import (
	"fmt"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

const (
	postponeReasonActionFinalizationSignatureFailure = "audit_action_finalization_signature_failure"
	postponeReasonActionFinalizationNotInTop10       = "audit_action_finalization_not_in_top_10"
)

// EnforceEpochEnd evaluates the completed epoch and updates supernode states accordingly.
// It does not re-check storage-challenge assignment gating; that must be enforced at MsgSubmitEpochReport time.
func (k Keeper) EnforceEpochEnd(ctx sdk.Context, epochID uint64, params types.Params) error {
	params = params.WithDefaults()

	active, err := k.supernodeKeeper.GetAllSuperNodes(ctx, sntypes.SuperNodeStateActive)
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

		shouldRecover, err := k.shouldRecoverAtEpochEnd(ctx, sn.SupernodeAccount, epochID, params)
		if err != nil {
			return err
		}
		if !shouldRecover {
			continue
		}

		if err := k.recoverSupernodeActive(ctx, sn); err != nil {
			return err
		}
		k.clearActionFinalizationPostponedAtEpochID(ctx, sn.SupernodeAccount)
	}

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

	// If any known metric is below minimum free%, postpone.
	if violatesMinFree(r.HostReport.CpuUsagePercent, params.MinCpuFreePercent) {
		return true, nil
	}
	if violatesMinFree(r.HostReport.MemUsagePercent, params.MinMemFreePercent) {
		return true, nil
	}
	if violatesMinFree(r.HostReport.DiskUsagePercent, params.MinDiskFreePercent) {
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
	if !compliesMinFree(r.HostReport.DiskUsagePercent, params.MinDiskFreePercent) {
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

func (k Keeper) recoverSupernodeActive(ctx sdk.Context, sn sntypes.SuperNode) error {
	if sn.ValidatorAddress == "" {
		return fmt.Errorf("missing validator address for supernode %q", sn.SupernodeAccount)
	}
	valAddr, err := sdk.ValAddressFromBech32(sn.ValidatorAddress)
	if err != nil {
		return err
	}
	return k.supernodeKeeper.RecoverSuperNodeFromPostponed(ctx, valAddr)
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
