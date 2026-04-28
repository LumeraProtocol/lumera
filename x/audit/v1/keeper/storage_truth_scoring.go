package keeper

import (
	"math"
	"math/big"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

type storageTruthScoreDeltas struct {
	nodeSuspicion       int64
	reporterReliability int64
	ticketDeterioration int64
}

type storageTruthResultBookkeeping struct {
	reporterTrustBand         types.ReporterTrustBand
	reporterTrustMultiplier   int64
	applyTrustScaling         bool
	repeatedFailureCount      uint32
	contradictionDetected     bool
	contradictedReporter      string
	currentReporterPenalty    int64
	contradictedReporterDelta int64
	nodeBonus                 int64
	ticketBonus               int64
}

// applyStorageTruthScores updates storage-truth scoring states from report results.
// This remains shadow-safe: it only updates LEP-6 score state and emits score events.
func (k Keeper) applyStorageTruthScores(
	ctx sdk.Context,
	epochID uint64,
	reporterAccount string,
	results []*types.StorageProofResult,
) error {
	if len(results) == 0 {
		return nil
	}

	params := k.GetParams(ctx).WithDefaults()
	switch params.StorageTruthEnforcementMode {
	case types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_SHADOW,
		types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_SOFT,
		types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_FULL:
	default:
		return nil
	}

	for _, result := range results {
		if result == nil {
			continue
		}

		deltas := storageTruthScoreDeltasForResult(result)
		// RECHECK bucket results bypass bookkeeping to avoid double-applying the
		// contradiction penalty already handled in SubmitStorageRecheckEvidence (121-F1).
		var bookkeeping storageTruthResultBookkeeping
		if result.BucketType != types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECHECK {
			var err error
			bookkeeping, err = k.storageTruthBookkeepingForResult(ctx, epochID, reporterAccount, result, params)
			if err != nil {
				return err
			}
		}

		deltas.reporterReliability = addInt64Saturated(deltas.reporterReliability, bookkeeping.currentReporterPenalty)
		// Per CP-NEW-A-14/A-15 — trust multiplier scales ONLY the base
		// Class-A failure delta. Pattern-escalation bonuses (nodeBonus,
		// ticketBonus) are spec-§14/§16 deterrents added unscaled, so a
		// degraded reporter cannot under-attribute them.
		if bookkeeping.applyTrustScaling {
			if deltas.nodeSuspicion > 0 {
				deltas.nodeSuspicion = scaleInt64TowardZero(deltas.nodeSuspicion, bookkeeping.reporterTrustMultiplier, 100)
			}
			if deltas.ticketDeterioration > 0 {
				deltas.ticketDeterioration = scaleInt64TowardZero(deltas.ticketDeterioration, bookkeeping.reporterTrustMultiplier, 100)
			}
		}
		deltas.nodeSuspicion = addInt64Saturated(deltas.nodeSuspicion, bookkeeping.nodeBonus)
		deltas.ticketDeterioration = addInt64Saturated(deltas.ticketDeterioration, bookkeeping.ticketBonus)

		// Clamp positive (failure) node and ticket deltas to >= 0 after scaling.
		if deltas.nodeSuspicion < 0 {
			// Pass deltas are negative and should stay negative — only clamp the result after applying.
		}

		nodeScore, nodeUpdated, err := k.applyNodeSuspicionDelta(
			ctx,
			epochID,
			result,
			deltas.nodeSuspicion,
			params.StorageTruthNodeSuspicionDecayPerEpoch,
			params,
		)
		if err != nil {
			return err
		}

		reporterState, reporterUpdated, err := k.applyReporterReliabilityDelta(
			ctx,
			epochID,
			reporterAccount,
			deltas.reporterReliability,
			params.StorageTruthReporterReliabilityDecayPerEpoch,
			boolToUint64(bookkeeping.contradictionDetected),
			params,
		)
		if err != nil {
			return err
		}

		if bookkeeping.contradictedReporter != "" && bookkeeping.contradictedReporterDelta != 0 {
			if _, _, err := k.applyReporterReliabilityDelta(
				ctx,
				epochID,
				bookkeeping.contradictedReporter,
				bookkeeping.contradictedReporterDelta,
				params.StorageTruthReporterReliabilityDecayPerEpoch,
				1,
				params,
			); err != nil {
				return err
			}
		}

		if result.BucketType != types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECHECK {
			if err := k.setStorageTruthReporterResult(ctx, epochID, reporterAccount, result); err != nil {
				return err
			}
		}
		if err := k.setStorageTruthNodeFailure(ctx, epochID, reporterAccount, result); err != nil {
			return err
		}

		ticketState, ticketUpdated, err := k.applyTicketDeteriorationDelta(
			ctx,
			epochID,
			reporterAccount,
			result,
			result.TicketId,
			deltas.ticketDeterioration,
			params.StorageTruthTicketDeteriorationDecayPerEpoch,
			bookkeeping.contradictionDetected,
		)
		if err != nil {
			return err
		}

		if !nodeUpdated && !reporterUpdated && !ticketUpdated {
			continue
		}

		attrs := []sdk.Attribute{
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(types.AttributeKeyEpochID, strconv.FormatUint(epochID, 10)),
			sdk.NewAttribute(types.AttributeKeyReporterSupernodeAccount, reporterAccount),
			sdk.NewAttribute(types.AttributeKeyTargetSupernodeAccount, result.TargetSupernodeAccount),
			sdk.NewAttribute(types.AttributeKeyTicketID, result.TicketId),
			sdk.NewAttribute(types.AttributeKeyResultClass, result.ResultClass.String()),
			sdk.NewAttribute(types.AttributeKeyBucketType, result.BucketType.String()),
			sdk.NewAttribute(types.AttributeKeyReporterTrustBand, reporterState.TrustBand.String()),
			sdk.NewAttribute(types.AttributeKeyRepeatedFailureCount, strconv.FormatUint(uint64(ticketState.RecentFailureEpochCount), 10)),
			sdk.NewAttribute(types.AttributeKeyContradictionDetected, strconv.FormatBool(bookkeeping.contradictionDetected)),
		}
		if nodeUpdated {
			attrs = append(attrs, sdk.NewAttribute(types.AttributeKeyNodeSuspicionScore, strconv.FormatInt(nodeScore, 10)))
		}
		if reporterUpdated {
			attrs = append(attrs, sdk.NewAttribute(types.AttributeKeyReporterReliabilityScore, strconv.FormatInt(reporterState.ReliabilityScore, 10)))
		}
		if ticketUpdated {
			attrs = append(attrs, sdk.NewAttribute(types.AttributeKeyTicketDeteriorationScore, strconv.FormatInt(ticketState.DeteriorationScore, 10)))
		}
		if bookkeeping.contradictedReporter != "" {
			attrs = append(attrs, sdk.NewAttribute(types.AttributeKeyContradictedReporter, bookkeeping.contradictedReporter))
		}
		ctx.EventManager().EmitEvent(sdk.NewEvent(types.EventTypeStorageTruthScoreUpdated, attrs...))
	}

	return nil
}

func (k Keeper) applyNodeSuspicionDelta(
	ctx sdk.Context,
	epochID uint64,
	result *types.StorageProofResult,
	delta int64,
	decayPerEpoch int64,
	params types.Params,
) (int64, bool, error) {
	if result == nil || result.TargetSupernodeAccount == "" {
		return 0, false, nil
	}
	supernodeAccount := result.TargetSupernodeAccount
	state, found := k.GetNodeSuspicionState(ctx, supernodeAccount)
	if !found && delta == 0 {
		return 0, false, nil
	}

	current := int64(0)
	if found {
		current = decayTowardZero(state.SuspicionScore, decayPerEpoch, epochDelta(epochID, state.LastUpdatedEpoch))
	}
	next := addInt64Saturated(current, delta)
	// Clamp node suspicion at >= 0.
	if next < 0 {
		next = 0
	}

	nextState := state
	nextState.SupernodeAccount = supernodeAccount
	nextState.SuspicionScore = next
	nextState.LastUpdatedEpoch = epochID

	// Update state history tracking fields.
	if result != nil {
		k.updateNodeSuspicionHistoryFields(&nextState, result, epochID, params)
	}

	if err := k.SetNodeSuspicionState(ctx, nextState); err != nil {
		return 0, false, err
	}
	return next, true, nil
}

// updateNodeSuspicionHistoryFields updates the history-tracking fields of NodeSuspicionState.
func (k Keeper) updateNodeSuspicionHistoryFields(state *types.NodeSuspicionState, result *types.StorageProofResult, epochID uint64, params types.Params) {
	window := uint64(params.StorageTruthPatternEscalationWindow)
	if window == 0 {
		window = 14
	}

	isFailure := isStorageTruthFailureClass(result.ResultClass)
	isPass := result.ResultClass == types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS

	if isPass {
		state.CleanPassCount++
		state.LastCleanPassEpoch = epochID
	}

	if isFailure {
		// Track bucket-specific fail epochs.
		switch result.BucketType {
		case types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT:
			state.LastRecentFailEpoch = epochID
		case types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_OLD:
			state.LastOldFailEpoch = epochID
		}

		// Track index fail epoch.
		if result.ArtifactClass == types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_INDEX {
			state.LastIndexFailEpoch = epochID
		}

		// Reset window if stale. Use epochDelta to avoid uint64 underflow when
		// WindowStartEpoch > epochID (e.g. genesis-imported future-pointing field).
		// Per NEW-A-12 / NEW-A-17.
		if epochDelta(epochID, state.WindowStartEpoch) >= window {
			state.WindowStartEpoch = epochID
			state.DistinctTicketFailWindow = 0
			state.ClassACountWindow = 0
			state.ClassBCountWindow = 0
		}

		// Track distinct ticket fail (simplified: increment per failure in window).
		state.DistinctTicketFailWindow++

		// Per CP-R3 B-F2 — Class A is failure-class driven, not artifact-class
		// driven. HASH_MISMATCH against an INDEX artifact still carries the +26
		// score delta via storageTruthScoreDeltasForResult, but TIMEOUT-on-INDEX
		// remains a liveness/Class-B failure and must not reset Class-A recovery
		// gates or increment ClassACountWindow.
		isClassA := result.ResultClass == types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH ||
			result.ResultClass == types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL
		if isClassA {
			state.ClassACountWindow++
			state.LastClassAEpoch = epochID
			// Recovery needs clean passes with no new Class A failures.
			state.CleanPassCount = 0
		}
		if result.ResultClass == types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_TIMEOUT_OR_NO_RESPONSE {
			state.ClassBCountWindow++
			state.LastClassBEpoch = epochID
		}
	}
}

func (k Keeper) applyReporterReliabilityDelta(
	ctx sdk.Context,
	epochID uint64,
	reporterAccount string,
	delta int64,
	decayPerEpoch int64,
	contradictionIncrements uint64,
	params types.Params,
) (types.ReporterReliabilityState, bool, error) {
	if reporterAccount == "" {
		return types.ReporterReliabilityState{}, false, nil
	}
	state, found := k.GetReporterReliabilityState(ctx, reporterAccount)
	if !found && delta == 0 && contradictionIncrements == 0 {
		return types.ReporterReliabilityState{}, false, nil
	}

	current := int64(0)
	if found {
		current = decayTowardZero(state.ReliabilityScore, decayPerEpoch, epochDelta(epochID, state.LastUpdatedEpoch))
	}
	next := addInt64Saturated(current, delta)
	// Clamp reporter reliability at >= 0 (positive-penalty model).
	if next < 0 {
		next = 0
	}

	// Update window tracking.
	nextState := state
	nextState.ReporterSupernodeAccount = reporterAccount
	nextState.ReliabilityScore = next
	nextState.LastUpdatedEpoch = epochID
	nextState.TrustBand = reporterTrustBandForScore(next, params)
	nextState.ContradictionCount = state.ContradictionCount + contradictionIncrements
	if nextState.TrustBand == types.ReporterTrustBand_REPORTER_TRUST_BAND_CHALLENGER_INELIGIBLE {
		ineligibleDuration := uint64(params.StorageTruthReporterIneligibleDurationEpochs)
		if ineligibleDuration == 0 {
			ineligibleDuration = 7
		}
		nextState.IneligibleUntilEpoch = epochID + ineligibleDuration
	} else if next < params.StorageTruthReporterReliabilityIneligibleThreshold {
		nextState.IneligibleUntilEpoch = 0
	}

	// Update divergence window tracking.
	divergenceWindow := uint64(params.StorageTruthDivergenceWindowEpochs)
	if divergenceWindow == 0 {
		divergenceWindow = 14
	}
	if epochDelta(epochID, state.WindowStartEpoch) >= divergenceWindow {
		nextState.WindowStartEpoch = epochID
		nextState.WindowPositiveCount = 0
		nextState.WindowNegativeCount = 0
	}
	if delta > 0 {
		if nextState.WindowNegativeCount < math.MaxUint32 {
			nextState.WindowNegativeCount++
		}
	} else if delta < 0 {
		if nextState.WindowPositiveCount < math.MaxUint32 {
			nextState.WindowPositiveCount++
		}
	}

	if err := k.SetReporterReliabilityState(ctx, nextState); err != nil {
		return types.ReporterReliabilityState{}, false, err
	}
	return nextState, true, nil
}

func (k Keeper) applyTicketDeteriorationDelta(
	ctx sdk.Context,
	epochID uint64,
	reporterAccount string,
	result *types.StorageProofResult,
	ticketID string,
	delta int64,
	decayPerEpoch int64,
	contradictionConfirmed bool,
) (types.TicketDeteriorationState, bool, error) {
	if ticketID == "" {
		return types.TicketDeteriorationState{}, false, nil
	}
	state, found := k.GetTicketDeteriorationState(ctx, ticketID)
	if !found && delta == 0 {
		return types.TicketDeteriorationState{}, false, nil
	}

	current := int64(0)
	if found {
		current = decayTowardZero(state.DeteriorationScore, decayPerEpoch, epochDelta(epochID, state.LastUpdatedEpoch))
	}
	next := addInt64Saturated(current, delta)
	// Per F119-F3 residue — clean-pass-by-different-holder bonus (spec §15.4).
	// When PASS lands on a ticket whose prior failure was from a DIFFERENT
	// holder, apply an additional -3 ticket-deterioration delta on top of the
	// base bucket reduction. This rewards a successful recovery on a fresh
	// holder distinctly from a clean-pass on the same holder.
	if result != nil &&
		result.ResultClass == types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS &&
		found &&
		state.LastTargetSupernodeAccount != "" &&
		state.LastTargetSupernodeAccount != result.TargetSupernodeAccount &&
		isStorageTruthFailureClass(state.LastResultClass) {
		next = addInt64Saturated(next, -3)
	}
	// Clamp ticket deterioration at >= 0.
	if next < 0 {
		next = 0
	}

	nextState := state
	nextState.TicketId = ticketID
	nextState.DeteriorationScore = next
	nextState.LastUpdatedEpoch = epochID
	if result != nil {
		isFailure := isStorageTruthFailureClass(result.ResultClass)
		if isFailure && (!found || epochID != state.LastFailureEpoch) {
			nextState.LastFailureEpoch = epochID
			nextState.RecentFailureEpochCount = updateRecentFailureEpochCount(state, epochID, k.GetParams(ctx).WithDefaults())
		}
		if result.TicketId != "" {
			// Track distinct holder failure count.
			if isFailure && state.LastTargetSupernodeAccount != "" && state.LastTargetSupernodeAccount != result.TargetSupernodeAccount {
				if nextState.DistinctHolderFailureCount < math.MaxUint32 {
					nextState.DistinctHolderFailureCount++
				}
			}

			// Track last index failure epoch.
			if isFailure && result.ArtifactClass == types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_INDEX {
				nextState.LastIndexFailureEpoch = epochID
			}

			// Track bucket-specific failure epochs.
			if isFailure {
				switch result.BucketType {
				case types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT:
					nextState.RecentBucketFailureEpoch = epochID
				case types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_OLD:
					nextState.OldBucketFailureEpoch = epochID
				}
			}

			nextState.LastTargetSupernodeAccount = result.TargetSupernodeAccount
			nextState.LastReporterSupernodeAccount = reporterAccount
			nextState.LastResultClass = result.ResultClass
			nextState.LastResultEpoch = epochID
			// Per Zee 119-F7 — same-epoch contradictions must be counted; <= not <.
			// Per F121-F10 / F119-F3 — only count ticket-side contradictions when the
			// reporter-side confirmation predicate held (PASS-after-fail with no
			// independent reporter PASS in window AND no clean recheck transcript).
			if contradictionConfirmed &&
				state.LastResultEpoch <= epochID &&
				state.LastTargetSupernodeAccount == result.TargetSupernodeAccount &&
				storageTruthResultsContradict(state.LastResultClass, result.ResultClass) {
				nextState.ContradictionCount = state.ContradictionCount + 1
			}
		}
	}
	if err := k.SetTicketDeteriorationState(ctx, nextState); err != nil {
		return types.TicketDeteriorationState{}, false, err
	}
	return nextState, true, nil
}

// storageTruthScoreDeltasForResult returns score deltas based on result class, artifact class, and bucket type.
// This replaces the old storageTruthScoreDeltasForResultClass function.
func storageTruthScoreDeltasForResult(result *types.StorageProofResult) storageTruthScoreDeltas {
	if result == nil {
		return storageTruthScoreDeltas{}
	}
	switch result.ResultClass {
	case types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS:
		// PASS deltas are REDUCTIONS (negative). Per CP-NEW-A-18, reporter
		// reliability recovery is per-EPOCH not per-result; emission moved to
		// ApplyReporterCleanEpochRecoveryAtEpochEnd. Per-result reporter delta = 0.
		// F119-F3 residue: cross-holder PASS bonus is applied in
		// applyTicketDeteriorationDelta (where the prior-holder state is in
		// scope), not at this per-result delta level.
		switch result.BucketType {
		case types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT:
			return storageTruthScoreDeltas{
				nodeSuspicion:       -3,
				reporterReliability: 0,
				ticketDeterioration: -2,
			}
		case types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_OLD:
			return storageTruthScoreDeltas{
				nodeSuspicion:       -2,
				reporterReliability: 0,
				ticketDeterioration: -3,
			}
		default:
			return storageTruthScoreDeltas{
				nodeSuspicion:       -2,
				reporterReliability: 0,
				ticketDeterioration: -2,
			}
		}
	case types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH:
		// Dispatch on artifact class.
		switch result.ArtifactClass {
		case types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_INDEX:
			return storageTruthScoreDeltas{
				nodeSuspicion:       26,
				reporterReliability: 1,
				ticketDeterioration: 12,
			}
		default: // SYMBOL or UNSPECIFIED — use symbol values as safe default.
			return storageTruthScoreDeltas{
				nodeSuspicion:       18,
				reporterReliability: 1,
				ticketDeterioration: 5,
			}
		}
	case types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_TIMEOUT_OR_NO_RESPONSE:
		// Per CP-NEW-A-18 — TIMEOUT reporter delta moved off per-result.
		return storageTruthScoreDeltas{
			nodeSuspicion:       7,
			reporterReliability: 0,
			ticketDeterioration: 3,
		}
	case types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_OBSERVER_QUORUM_FAIL:
		return storageTruthScoreDeltas{
			nodeSuspicion:       4, // Per LEP6.md §14:405 — unresolved OBSERVER_QUORUM_FAIL: +4
			reporterReliability: 0,
			ticketDeterioration: 0,
		}
	case types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_NO_ELIGIBLE_TICKET:
		return storageTruthScoreDeltas{
			nodeSuspicion:       0,
			reporterReliability: 0,
			ticketDeterioration: 0,
		}
	case types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_INVALID_TRANSCRIPT:
		return storageTruthScoreDeltas{
			nodeSuspicion:       0,
			reporterReliability: 0,
			ticketDeterioration: 0,
		}
	case types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL:
		return storageTruthScoreDeltas{
			nodeSuspicion:       15,
			reporterReliability: 3,
			ticketDeterioration: 8,
		}
	default:
		return storageTruthScoreDeltas{}
	}
}

func epochDelta(currentEpoch, lastUpdatedEpoch uint64) uint64 {
	if currentEpoch <= lastUpdatedEpoch {
		return 0
	}
	return currentEpoch - lastUpdatedEpoch
}

func (k Keeper) storageTruthBookkeepingForResult(
	ctx sdk.Context,
	epochID uint64,
	reporterAccount string,
	result *types.StorageProofResult,
	params types.Params,
) (storageTruthResultBookkeeping, error) {
	bookkeeping := storageTruthResultBookkeeping{
		reporterTrustBand:       types.ReporterTrustBand_REPORTER_TRUST_BAND_NORMAL,
		reporterTrustMultiplier: 100,
	}
	if result == nil {
		return bookkeeping, nil
	}

	reliabilityScore := int64(0)
	if state, found := k.GetReporterReliabilityState(ctx, reporterAccount); found {
		reliabilityScore = decayTowardZero(state.ReliabilityScore, params.StorageTruthReporterReliabilityDecayPerEpoch, epochDelta(epochID, state.LastUpdatedEpoch))
	}
	bookkeeping.reporterTrustBand = reporterTrustBandForScore(reliabilityScore, params)
	bookkeeping.reporterTrustMultiplier = reporterTrustMultiplierNumerator(reliabilityScore)
	// Per CP-R3 B-F2 / spec §15.4 — trust multiplier applies to Class A
	// failures only. INDEX artifact status affects HASH_MISMATCH delta magnitude
	// (+26 for index vs +18 for symbol), but does not by itself make a liveness
	// failure Class A. TIMEOUT/OBSERVER_QUORUM_FAIL/INVALID_TRANSCRIPT remain
	// non-scaled Class B/C failures. Recheck-confirmed failures and recheck-bucket
	// results bypass scaling regardless of class.
	isClassA := result.ResultClass == types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH
	bookkeeping.applyTrustScaling = isClassA &&
		isStorageTruthFailureClass(result.ResultClass) &&
		result.ResultClass != types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL &&
		result.BucketType != types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECHECK

	if result.TicketId == "" {
		return bookkeeping, nil
	}

	ticketState, found := k.GetTicketDeteriorationState(ctx, result.TicketId)
	if isStorageTruthFailureClass(result.ResultClass) {
		patternWindow := uint64(params.StorageTruthPatternEscalationWindow)
		if patternWindow == 0 {
			patternWindow = 14
		}
		tickets, _, err := k.distinctNodeFailedTickets(ctx, result.TargetSupernodeAccount, storageTruthWindowStart(epochID, patternWindow), epochID, nil)
		if err != nil {
			return bookkeeping, err
		}
		tickets[result.TicketId] = struct{}{}
		bookkeeping.repeatedFailureCount = uint32(len(tickets))
		if bookkeeping.repeatedFailureCount > 1 {
			// §14: node suspicion pattern escalation based on distinct ticket count.
			bookkeeping.nodeBonus = repeatedFailureEscalationBonus(bookkeeping.repeatedFailureCount)
		}
		if found {
			// §16: ticket deterioration escalation distinguishes holder identity.
			// Different holder failing same ticket in window: +10.
			// Same holder failing same ticket in a different epoch: +6.
			if epochID != ticketState.LastFailureEpoch && ticketState.LastTargetSupernodeAccount != "" {
				if ticketState.LastTargetSupernodeAccount != result.TargetSupernodeAccount {
					bookkeeping.ticketBonus = 10
				} else {
					bookkeeping.ticketBonus = 6
				}
			}
		}

		// §14 cross-bucket pattern escalation: +12 if both recent AND old fails within pattern window.
		if result.TargetSupernodeAccount != "" {
			currentIsRecent := result.BucketType == types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT
			currentIsOld := result.BucketType == types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_OLD
			recentFailed, err := k.hasNodeFailure(ctx, result.TargetSupernodeAccount, storageTruthWindowStart(epochID, patternWindow), epochID, func(record storageTruthNodeFailureRecord) bool {
				return types.StorageProofBucketType(record.BucketType) == types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT
			})
			if err != nil {
				return bookkeeping, err
			}
			oldFailed, err := k.hasNodeFailure(ctx, result.TargetSupernodeAccount, storageTruthWindowStart(epochID, patternWindow), epochID, func(record storageTruthNodeFailureRecord) bool {
				return types.StorageProofBucketType(record.BucketType) == types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_OLD
			})
			if err != nil {
				return bookkeeping, err
			}
			if (currentIsRecent || recentFailed) && (currentIsOld || oldFailed) {
				bookkeeping.nodeBonus = addInt64Saturated(bookkeeping.nodeBonus, 12)
			}
		}
	} else if found {
		bookkeeping.repeatedFailureCount = ticketState.RecentFailureEpochCount
	}

	if found && ticketState.LastResultEpoch <= epochID &&
		ticketState.LastTargetSupernodeAccount == result.TargetSupernodeAccount &&
		isStorageTruthFailureClass(ticketState.LastResultClass) &&
		result.ResultClass == types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS {
		contradictionWindow := uint64(params.StorageTruthContradictionWindowEpochs)
		if contradictionWindow == 0 {
			contradictionWindow = 7
		}
		confirmed, err := k.hasStorageTruthContradictionConfirmation(
			ctx,
			epochID,
			result.TicketId,
			result.TargetSupernodeAccount,
			reporterAccount,
			contradictionWindow,
		)
		if err != nil {
			return bookkeeping, err
		}
		if confirmed {
			bookkeeping.contradictionDetected = true
			if ticketState.LastReporterSupernodeAccount != "" && ticketState.LastReporterSupernodeAccount != reporterAccount {
				bookkeeping.contradictedReporter = ticketState.LastReporterSupernodeAccount
				bookkeeping.contradictedReporterDelta = 12
			}
		}
	}

	return bookkeeping, nil
}

func (k Keeper) hasStorageTruthContradictionConfirmation(
	ctx sdk.Context,
	epochID uint64,
	ticketID string,
	targetAccount string,
	currentReporter string,
	window uint64,
) (bool, error) {
	if ticketID == "" || targetAccount == "" {
		return false, nil
	}
	startEpoch := storageTruthWindowStart(epochID, window)
	independentPass, err := k.hasIndependentReporterPassInWindow(ctx, ticketID, targetAccount, currentReporter, startEpoch, epochID)
	if err != nil {
		return false, err
	}
	if independentPass {
		return true, nil
	}
	return k.hasCleanRecheckInWindow(ctx, ticketID, targetAccount, startEpoch, epochID)
}

func reporterTrustBandForScore(score int64, params types.Params) types.ReporterTrustBand {
	// Positive-penalty model: R=0 is clean, higher R = more problematic.
	ineligibleThreshold := params.StorageTruthReporterReliabilityIneligibleThreshold
	if ineligibleThreshold <= 0 {
		ineligibleThreshold = 90
	}
	degradedThreshold := params.StorageTruthReporterReliabilityDegradedThreshold
	if degradedThreshold <= 0 {
		degradedThreshold = 50
	}
	lowTrustThreshold := params.StorageTruthReporterReliabilityLowTrustThreshold
	if lowTrustThreshold <= 0 {
		lowTrustThreshold = 20
	}

	switch {
	case score >= ineligibleThreshold:
		return types.ReporterTrustBand_REPORTER_TRUST_BAND_CHALLENGER_INELIGIBLE
	case score >= degradedThreshold:
		return types.ReporterTrustBand_REPORTER_TRUST_BAND_DEGRADED
	case score >= lowTrustThreshold:
		return types.ReporterTrustBand_REPORTER_TRUST_BAND_LOW_TRUST
	default:
		return types.ReporterTrustBand_REPORTER_TRUST_BAND_NORMAL
	}
}

// reporterTrustMultiplierNumerator implements the continuous formula: max(50, 100 - score)
// for positive-penalty model where score >= 0. Returns numerator/100 as multiplier.
func reporterTrustMultiplierNumerator(score int64) int64 {
	if score <= 0 {
		return 100
	}
	numerator := 100 - score
	if numerator < 50 {
		return 50
	}
	return numerator
}

// Per 119-Roomote-B / Copilot-2 — big.Int avoids int64 overflow.
func scaleInt64TowardZero(value, numerator, denominator int64) int64 {
	if denominator <= 0 || numerator <= 0 || value == 0 {
		return 0
	}
	if numerator >= denominator {
		return value
	}
	bv := new(big.Int).SetInt64(value)
	bn := new(big.Int).SetInt64(numerator)
	bd := new(big.Int).SetInt64(denominator)
	bv.Mul(bv, bn).Quo(bv, bd)
	return bv.Int64()
}

func isStorageTruthFailureClass(class types.StorageProofResultClass) bool {
	switch class {
	case types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
		types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_TIMEOUT_OR_NO_RESPONSE,
		types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_OBSERVER_QUORUM_FAIL,
		types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_INVALID_TRANSCRIPT,
		types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL:
		return true
	default:
		return false
	}
}

func storageTruthResultsContradict(prev, current types.StorageProofResultClass) bool {
	prevFailure := isStorageTruthFailureClass(prev)
	currentFailure := isStorageTruthFailureClass(current)
	return (prev == types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS && currentFailure) ||
		(current == types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS && prevFailure)
}

func updateRecentFailureEpochCount(state types.TicketDeteriorationState, epochID uint64, params types.Params) uint32 {
	if epochID == state.LastFailureEpoch {
		if state.RecentFailureEpochCount == 0 {
			return 1
		}
		return state.RecentFailureEpochCount
	}
	if state.RecentFailureEpochCount == 0 {
		return 1
	}
	window := uint64(params.StorageTruthPatternEscalationWindow)
	if window == 0 {
		window = 14
	}
	if window < 2 {
		window = 2
	}
	if epochDelta(epochID, state.LastFailureEpoch) > window {
		return 1
	}
	if state.RecentFailureEpochCount == math.MaxUint32 {
		return math.MaxUint32
	}
	return state.RecentFailureEpochCount + 1
}

// repeatedFailureEscalationBonus implements spec-aligned pattern escalation.
// Returns the pattern bonus for the node and ticket based on distinct ticket fail count.
// Per LEP6.md §14: second distinct failed ticket in last 14 epochs: +10;
// third or more: +15.
func repeatedFailureEscalationBonus(count uint32) int64 {
	switch {
	case count <= 1:
		return 0
	case count == 2:
		return 10
	default: // count >= 3
		return 15
	}
}

func storageTruthWindowStart(epochID uint64, window uint64) uint64 {
	if window == 0 || epochID+1 <= window {
		return 0
	}
	return epochID - window + 1
}

func boolToUint64(v bool) uint64 {
	if v {
		return 1
	}
	return 0
}

// decayTowardZero applies exponential decay to score.
// factorNumerator is the decay factor * 1000 (e.g., 920 means 0.920 per epoch).
// Formula: score * (factorNumerator/1000)^elapsedEpochs using integer arithmetic.
// Returns max(0, result) for positive scores, min(0, result) for negative.
// Capped at 50 iterations to prevent runaway (beyond 50 epochs, any reasonable factor decays below 1).
// For factorNumerator > 1000, returns score unchanged (no decay).
// For factorNumerator <= 0 or elapsedEpochs == 0, returns score unchanged.
func decayTowardZero(score, factorNumerator int64, elapsedEpochs uint64) int64 {
	if score == 0 || elapsedEpochs == 0 {
		return score
	}
	if factorNumerator <= 0 {
		return score
	}
	if factorNumerator > 1000 {
		// Factor > 1.0 means growth, not decay — treat as no decay.
		return score
	}
	if factorNumerator == 1000 {
		// Factor = 1.0 means no decay.
		return score
	}

	// Iterative multiplication to avoid floating point.
	// Cap at 50 iterations.
	iterations := elapsedEpochs
	if iterations > 50 {
		iterations = 50
	}

	result := score
	for i := uint64(0); i < iterations; i++ {
		if result > 0 {
			result = (result * factorNumerator) / 1000
			if result <= 0 {
				return 0
			}
		} else {
			result = (result * factorNumerator) / 1000
			if result >= 0 {
				return 0
			}
		}
	}

	if score > 0 {
		if result < 0 {
			return 0
		}
		return result
	}
	if result > 0 {
		return 0
	}
	return result
}

func addInt64Saturated(a, b int64) int64 {
	if b > 0 && a > math.MaxInt64-b {
		return math.MaxInt64
	}
	if b < 0 && a < math.MinInt64-b {
		return math.MinInt64
	}
	return a + b
}
