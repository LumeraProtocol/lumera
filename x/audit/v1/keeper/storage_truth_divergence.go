package keeper

import (
	"encoding/json"
	"math/big"
	"sort"
	"strconv"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// ApplyReporterDivergenceAtEpochEnd checks all reporters with sufficient volume in the
// rolling window and penalizes chronic outliers whose negative rate exceeds 2x the
// network median. (LEP6.md §15.2)
//
// All ratio comparisons use integer cross-multiplication to eliminate float64
// non-determinism across validators (121-F16).
func (k Keeper) ApplyReporterDivergenceAtEpochEnd(ctx sdk.Context, epochID uint64, params types.Params) error {
	minReports := params.StorageTruthReporterMinReportsForDivergence
	if minReports == 0 {
		minReports = 5
	}

	states, err := k.GetAllReporterReliabilityStates(ctx)
	if err != nil {
		return err
	}
	if len(states) == 0 {
		return nil
	}

	type reporterEntry struct {
		account            string
		negative           uint64
		total              uint64
		confirmedNegatives uint64
	}

	qualifying := make([]reporterEntry, 0, len(states))
	startEpoch := storageTruthWindowStart(epochID, uint64(params.StorageTruthDivergenceWindowEpochs))
	for _, state := range states {
		stats, err := k.storageTruthReporterDivergenceStats(ctx, state.ReporterSupernodeAccount, startEpoch, epochID)
		if err != nil {
			return err
		}
		if stats.total < uint64(minReports) {
			continue
		}
		qualifying = append(qualifying, reporterEntry{
			account:            state.ReporterSupernodeAccount,
			negative:           stats.negative,
			total:              stats.total,
			confirmedNegatives: stats.confirmedNegative,
		})
	}

	if len(qualifying) == 0 {
		return nil
	}

	// Sort by neg/total ratio using *big.Int cross-multiply to avoid float64
	// non-determinism (121-F16) AND uint64 overflow when neg/total approach
	// or exceed 2^32 (CP-NEW-A-13).
	// a.negative/a.total < b.negative/b.total  ⟺  a.negative*b.total < b.negative*a.total
	sort.Slice(qualifying, func(i, j int) bool {
		lhs := new(big.Int).Mul(new(big.Int).SetUint64(qualifying[i].negative), new(big.Int).SetUint64(qualifying[j].total))
		rhs := new(big.Int).Mul(new(big.Int).SetUint64(qualifying[j].negative), new(big.Int).SetUint64(qualifying[i].total))
		return lhs.Cmp(rhs) < 0
	})

	// Compute median neg-rate as an integer pair (medianNeg, medianTotal).
	// Per NEW-A-16 — upper-pair selection is more conservative (harder to penalize):
	// a higher median raises the 2x threshold, making it harder to flag a reporter
	// as a divergence outlier. For odd-length slices index `mid` is the unique median;
	// for even-length slices index `mid` is the upper of the two middle elements.
	mid := len(qualifying) / 2
	medianNeg := qualifying[mid].negative
	medianTotal := qualifying[mid].total

	if medianTotal == 0 {
		return nil
	}

	// Penalize reporters whose neg_rate > 2x median.
	// entry.negative/entry.total > 2*medianNeg/medianTotal
	//   ⟺ entry.negative * medianTotal > 2 * medianNeg * entry.total
	// *big.Int cross-multiply protects against uint64 overflow (CP-NEW-A-13).
	bigMedianTotal := new(big.Int).SetUint64(medianTotal)
	bigMedianNegX2 := new(big.Int).Mul(new(big.Int).SetUint64(medianNeg), big.NewInt(2))
	for _, entry := range qualifying {
		if entry.total == 0 {
			continue
		}
		lhs := new(big.Int).Mul(new(big.Int).SetUint64(entry.negative), bigMedianTotal)
		rhs := new(big.Int).Mul(bigMedianNegX2, new(big.Int).SetUint64(entry.total))
		if lhs.Cmp(rhs) <= 0 {
			continue
		}
		if entry.negative != 0 && entry.confirmedNegatives*2 >= entry.negative {
			continue
		}

		// Apply +8 divergence penalty.
		if _, _, err := k.applyReporterReliabilityDelta(
			ctx,
			epochID,
			entry.account,
			8,
			params.StorageTruthReporterReliabilityDecayPerEpoch,
			0,
			params,
		); err != nil {
			return err
		}

		ctx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventTypeStorageTruthScoreUpdated,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(types.AttributeKeyEpochID, strconv.FormatUint(epochID, 10)),
			sdk.NewAttribute(types.AttributeKeyReporterSupernodeAccount, entry.account),
			sdk.NewAttribute("divergence_penalty", "8"),
			sdk.NewAttribute("reporter_neg_count", strconv.FormatUint(entry.negative, 10)),
			sdk.NewAttribute("reporter_total_count", strconv.FormatUint(entry.total, 10)),
			sdk.NewAttribute("median_neg_count", strconv.FormatUint(medianNeg, 10)),
			sdk.NewAttribute("median_total_count", strconv.FormatUint(medianTotal, 10)),
		))
	}

	return nil
}

type storageTruthDivergenceStats struct {
	total             uint64
	negative          uint64
	confirmedNegative uint64
}

// ApplyReporterCleanEpochRecoveryAtEpochEnd implements the per-EPOCH recovery
// half of NEW-A-18 (LEP6.md §15.3): for each reporter, if they produced >=5
// PASS results in the closing epoch AND no PASS was overturned by recheck,
// apply a single -4 reduction to their reliability score. Per-result PASS/
// TIMEOUT reporter deltas are 0 (see storageTruthScoreDeltasForResult); the
// recovery is consolidated here to avoid score deflation under high volume.
//
// The PASS-count threshold is intentionally hardcoded to 5 (Pitfall #13 —
// const-now/param-later); promote to a Param if governance ever needs tuning.
func (k Keeper) ApplyReporterCleanEpochRecoveryAtEpochEnd(ctx sdk.Context, epochID uint64, params types.Params) error {
	const cleanPassesRequired = 5

	states, err := k.GetAllReporterReliabilityStates(ctx)
	if err != nil {
		return err
	}
	reporterSet := make(map[string]struct{}, len(states))
	for _, state := range states {
		if state.ReporterSupernodeAccount != "" {
			reporterSet[state.ReporterSupernodeAccount] = struct{}{}
		}
	}
	epochReporters, err := k.storageTruthReporterAccountsForEpoch(ctx, epochID)
	if err != nil {
		return err
	}
	for _, reporter := range epochReporters {
		reporterSet[reporter] = struct{}{}
	}
	if len(reporterSet) == 0 {
		return nil
	}
	reporters := make([]string, 0, len(reporterSet))
	for reporter := range reporterSet {
		reporters = append(reporters, reporter)
	}
	sort.Strings(reporters)

	for _, reporterAccount := range reporters {
		passes, overturned, err := k.storageTruthReporterEpochPassStats(ctx, reporterAccount, epochID)
		if err != nil {
			return err
		}
		if overturned || passes < cleanPassesRequired {
			continue
		}

		if _, _, err := k.applyReporterReliabilityDelta(
			ctx,
			epochID,
			reporterAccount,
			-4,
			params.StorageTruthReporterReliabilityDecayPerEpoch,
			0,
			params,
		); err != nil {
			return err
		}

		ctx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventTypeStorageTruthScoreUpdated,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(types.AttributeKeyEpochID, strconv.FormatUint(epochID, 10)),
			sdk.NewAttribute(types.AttributeKeyReporterSupernodeAccount, reporterAccount),
			sdk.NewAttribute("clean_epoch_recovery_delta", "-4"),
			sdk.NewAttribute("epoch_pass_count", strconv.FormatUint(passes, 10)),
		))
	}
	return nil
}

// storageTruthReporterAccountsForEpoch returns all reporters that have at
// least one reporter-result fact in epochID, including fresh reporters that do
// not yet have a ReporterReliabilityState row.
func (k Keeper) storageTruthReporterAccountsForEpoch(ctx sdk.Context, epochID uint64) ([]string, error) {
	prefix := types.ReporterStorageTruthResultRootPrefix()
	it := k.kvStore(ctx).Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer it.Close()

	reporterSet := make(map[string]struct{})
	for ; it.Valid(); it.Next() {
		var record storageTruthReporterResultRecord
		if err := json.Unmarshal(it.Value(), &record); err != nil {
			return nil, err
		}
		if record.EpochID == epochID && record.Reporter != "" {
			reporterSet[record.Reporter] = struct{}{}
		}
	}
	reporters := make([]string, 0, len(reporterSet))
	for reporter := range reporterSet {
		reporters = append(reporters, reporter)
	}
	sort.Strings(reporters)
	return reporters, nil
}

// storageTruthReporterEpochPassStats counts PASS results for a reporter in a
// single epoch and reports whether any of them was overturned by recheck.
func (k Keeper) storageTruthReporterEpochPassStats(ctx sdk.Context, reporterAccount string, epochID uint64) (uint64, bool, error) {
	start, end := types.ReporterStorageTruthResultEpochScanRange(reporterAccount, epochID, epochID)
	it := k.kvStore(ctx).Iterator(start, end)
	defer it.Close()
	var passes uint64
	var overturned bool
	// Per CP-R3 B-F1 — `OverturnedByRecheck` is written by
	// markStorageTruthReporterResultRecheck onto the *failure-class* record
	// (the challenged transcript), never onto a PASS record. The previous
	// `continue` skipping non-PASS records made the gate structurally
	// unreachable. Spec §15.3 requires "no overturned fails" to suppress
	// the −4 reward, so we now scan all classes and check the overturn
	// flag on failure-class records, while still counting PASSes for the
	// ≥5 PASS gate.
	for ; it.Valid(); it.Next() {
		var record storageTruthReporterResultRecord
		if err := json.Unmarshal(it.Value(), &record); err != nil {
			return 0, false, err
		}
		class := types.StorageProofResultClass(record.ResultClass)
		if record.OverturnedByRecheck && isStorageTruthFailureClass(class) {
			overturned = true
		}
		if class == types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS {
			passes++
		}
	}
	return passes, overturned, nil
}

func (k Keeper) storageTruthReporterDivergenceStats(ctx sdk.Context, reporterAccount string, startEpoch uint64, endEpoch uint64) (storageTruthDivergenceStats, error) {
	var stats storageTruthDivergenceStats
	// Bounded epoch scan per CP-NEW-A-11 residue — key shape unchanged,
	// only iterator bounds use [startEpoch, endEpoch+1).
	start, end := types.ReporterStorageTruthResultEpochScanRange(reporterAccount, startEpoch, endEpoch)
	it := k.kvStore(ctx).Iterator(start, end)
	defer it.Close()
	for ; it.Valid(); it.Next() {
		var record storageTruthReporterResultRecord
		if err := json.Unmarshal(it.Value(), &record); err != nil {
			return stats, err
		}
		stats.total++
		if isStorageTruthFailureClass(types.StorageProofResultClass(record.ResultClass)) {
			stats.negative++
			if record.ConfirmedByRecheck {
				stats.confirmedNegative++
			}
		}
	}
	return stats, nil
}
