package keeper

import (
	"math"
	"math/bits"
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

		deltas := storageTruthScoreDeltasForResultClass(result.ResultClass)
		bookkeeping, err := k.storageTruthBookkeepingForResult(ctx, epochID, reporterAccount, result, params)
		if err != nil {
			return err
		}

		deltas.reporterReliability = addInt64Saturated(deltas.reporterReliability, bookkeeping.currentReporterPenalty)
		deltas.nodeSuspicion = addInt64Saturated(deltas.nodeSuspicion, bookkeeping.nodeBonus)
		deltas.ticketDeterioration = addInt64Saturated(deltas.ticketDeterioration, bookkeeping.ticketBonus)
		deltas.nodeSuspicion = scaleInt64TowardZero(deltas.nodeSuspicion, reporterTrustMultiplierNumerator(bookkeeping.reporterTrustBand), 100)
		deltas.ticketDeterioration = scaleInt64TowardZero(deltas.ticketDeterioration, reporterTrustMultiplierNumerator(bookkeeping.reporterTrustBand), 100)

		nodeScore, nodeUpdated, err := k.applyNodeSuspicionDelta(
			ctx,
			epochID,
			result.TargetSupernodeAccount,
			deltas.nodeSuspicion,
			params.StorageTruthNodeSuspicionDecayPerEpoch,
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

		ticketState, ticketUpdated, err := k.applyTicketDeteriorationDelta(
			ctx,
			epochID,
			reporterAccount,
			result,
			result.TicketId,
			deltas.ticketDeterioration,
			params.StorageTruthTicketDeteriorationDecayPerEpoch,
			params,
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
	supernodeAccount string,
	delta int64,
	decayPerEpoch int64,
) (int64, bool, error) {
	if supernodeAccount == "" {
		return 0, false, nil
	}
	state, found := k.GetNodeSuspicionState(ctx, supernodeAccount)
	if !found && delta == 0 {
		return 0, false, nil
	}

	current := int64(0)
	if found {
		current = decayTowardZero(state.SuspicionScore, decayPerEpoch, epochDelta(epochID, state.LastUpdatedEpoch))
	}
	next := addInt64Saturated(current, delta)

	nextState := types.NodeSuspicionState{
		SupernodeAccount: supernodeAccount,
		SuspicionScore:   next,
		LastUpdatedEpoch: epochID,
	}
	if err := k.SetNodeSuspicionState(ctx, nextState); err != nil {
		return 0, false, err
	}
	return next, true, nil
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

	nextState := types.ReporterReliabilityState{
		ReporterSupernodeAccount: reporterAccount,
		ReliabilityScore:         next,
		LastUpdatedEpoch:         epochID,
		TrustBand:                reporterTrustBandForScore(next, params),
		ContradictionCount:       state.ContradictionCount + contradictionIncrements,
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
	params types.Params,
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

	nextState := state
	nextState.TicketId = ticketID
	nextState.DeteriorationScore = next
	nextState.LastUpdatedEpoch = epochID
	if result != nil {
		if isStorageTruthFailureClass(result.ResultClass) && epochID != state.LastFailureEpoch {
			nextState.LastFailureEpoch = epochID
			nextState.RecentFailureEpochCount = updateRecentFailureEpochCount(state, epochID, params)
		} else if !found {
			nextState.RecentFailureEpochCount = 0
		}
		if result.TicketId != "" {
			nextState.LastTargetSupernodeAccount = result.TargetSupernodeAccount
			nextState.LastReporterSupernodeAccount = reporterAccount
			nextState.LastResultClass = result.ResultClass
			nextState.LastResultEpoch = epochID
			if state.LastResultEpoch < epochID &&
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

func storageTruthScoreDeltasForResultClass(class types.StorageProofResultClass) storageTruthScoreDeltas {
	switch class {
	case types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS:
		return storageTruthScoreDeltas{
			nodeSuspicion:       -2,
			reporterReliability: 2,
			ticketDeterioration: -3,
		}
	case types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH:
		return storageTruthScoreDeltas{
			nodeSuspicion:       12,
			reporterReliability: 1,
			ticketDeterioration: 12,
		}
	case types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_TIMEOUT_OR_NO_RESPONSE:
		return storageTruthScoreDeltas{
			nodeSuspicion:       4,
			reporterReliability: -1,
			ticketDeterioration: 4,
		}
	case types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_OBSERVER_QUORUM_FAIL:
		return storageTruthScoreDeltas{
			nodeSuspicion:       3,
			reporterReliability: -3,
			ticketDeterioration: 5,
		}
	case types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_NO_ELIGIBLE_TICKET:
		return storageTruthScoreDeltas{
			nodeSuspicion:       0,
			reporterReliability: 1,
			ticketDeterioration: 0,
		}
	case types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_INVALID_TRANSCRIPT:
		return storageTruthScoreDeltas{
			nodeSuspicion:       0,
			reporterReliability: -8,
			ticketDeterioration: 0,
		}
	case types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL:
		return storageTruthScoreDeltas{
			nodeSuspicion:       20,
			reporterReliability: 3,
			ticketDeterioration: 20,
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
		reporterTrustBand: types.ReporterTrustBand_REPORTER_TRUST_BAND_NORMAL,
	}
	if result == nil {
		return bookkeeping, nil
	}

	reliabilityScore := int64(0)
	if state, found := k.GetReporterReliabilityState(ctx, reporterAccount); found {
		reliabilityScore = decayTowardZero(state.ReliabilityScore, params.StorageTruthReporterReliabilityDecayPerEpoch, epochDelta(epochID, state.LastUpdatedEpoch))
	}
	bookkeeping.reporterTrustBand = reporterTrustBandForScore(reliabilityScore, params)

	if result.TicketId == "" {
		return bookkeeping, nil
	}

	ticketState, found := k.GetTicketDeteriorationState(ctx, result.TicketId)
	if !found {
		if isStorageTruthFailureClass(result.ResultClass) {
			bookkeeping.repeatedFailureCount = 1
		}
		return bookkeeping, nil
	}

	if isStorageTruthFailureClass(result.ResultClass) {
		bookkeeping.repeatedFailureCount = updateRecentFailureEpochCount(ticketState, epochID, params)
		if bookkeeping.repeatedFailureCount > 1 && epochID != ticketState.LastFailureEpoch {
			bonus := repeatedFailureEscalationBonus(bookkeeping.repeatedFailureCount)
			bookkeeping.nodeBonus = bonus
			bookkeeping.ticketBonus = bonus
		}
	} else {
		bookkeeping.repeatedFailureCount = ticketState.RecentFailureEpochCount
	}

	if ticketState.LastResultEpoch < epochID &&
		ticketState.LastTargetSupernodeAccount == result.TargetSupernodeAccount &&
		storageTruthResultsContradict(ticketState.LastResultClass, result.ResultClass) {
		bookkeeping.contradictionDetected = true
		bookkeeping.currentReporterPenalty = -6
		if ticketState.LastReporterSupernodeAccount != "" && ticketState.LastReporterSupernodeAccount != reporterAccount {
			bookkeeping.contradictedReporter = ticketState.LastReporterSupernodeAccount
			bookkeeping.contradictedReporterDelta = -6
		}
	}

	return bookkeeping, nil
}

func reporterTrustBandForScore(score int64, params types.Params) types.ReporterTrustBand {
	switch {
	case score <= params.StorageTruthReporterReliabilityIneligibleThreshold:
		return types.ReporterTrustBand_REPORTER_TRUST_BAND_CHALLENGER_INELIGIBLE
	case score <= params.StorageTruthReporterReliabilityLowTrustThreshold:
		return types.ReporterTrustBand_REPORTER_TRUST_BAND_LOW_TRUST
	default:
		return types.ReporterTrustBand_REPORTER_TRUST_BAND_NORMAL
	}
}

func reporterTrustMultiplierNumerator(band types.ReporterTrustBand) int64 {
	switch band {
	case types.ReporterTrustBand_REPORTER_TRUST_BAND_LOW_TRUST:
		return 50
	case types.ReporterTrustBand_REPORTER_TRUST_BAND_CHALLENGER_INELIGIBLE:
		return 25
	default:
		return 100
	}
}

func scaleInt64TowardZero(value, numerator, denominator int64) int64 {
	if denominator <= 0 || numerator <= 0 || value == 0 {
		return 0
	}
	if numerator >= denominator {
		return value
	}

	absValue := absInt64ToUint64(value)
	denominatorU := uint64(denominator)
	numeratorU := uint64(numerator)

	quotient := absValue / denominatorU
	remainder := absValue % denominatorU

	// Safe because numerator < denominator, therefore quotient*numerator <= absValue.
	scaledQuotient := quotient * numeratorU

	// remainder*numerator can overflow int64 in intermediate math; use 128-bit arithmetic.
	hi, lo := bits.Mul64(remainder, numeratorU)
	scaledRemainder, _ := bits.Div64(hi, lo, denominatorU)

	scaled := scaledQuotient + scaledRemainder
	if value < 0 {
		return -int64(scaled)
	}
	return int64(scaled)
}

func absInt64ToUint64(v int64) uint64 {
	if v >= 0 {
		return uint64(v)
	}
	// For MinInt64 this computes abs without overflow.
	return uint64(-(v + 1)) + 1
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
	window := uint64(params.StorageTruthProbationEpochs)
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

func repeatedFailureEscalationBonus(count uint32) int64 {
	if count <= 1 {
		return 0
	}
	bonusSteps := count - 1
	if bonusSteps > 3 {
		bonusSteps = 3
	}
	return int64(bonusSteps) * 2
}

func boolToUint64(v bool) uint64 {
	if v {
		return 1
	}
	return 0
}

func decayTowardZero(score, decayPerEpoch int64, elapsedEpochs uint64) int64 {
	if score == 0 || decayPerEpoch <= 0 || elapsedEpochs == 0 {
		return score
	}
	decayTotal := mulInt64ByUint64Saturated(decayPerEpoch, elapsedEpochs)
	if score > 0 {
		if decayTotal >= score {
			return 0
		}
		return score - decayTotal
	}
	if decayTotal >= -score {
		return 0
	}
	return score + decayTotal
}

func mulInt64ByUint64Saturated(v int64, m uint64) int64 {
	if v <= 0 || m == 0 {
		return 0
	}
	if m > uint64(math.MaxInt64/v) {
		return math.MaxInt64
	}
	return v * int64(m)
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
