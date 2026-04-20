package keeper

import (
	"encoding/json"
	"sort"
	"strconv"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// ApplyReporterDivergenceAtEpochEnd checks all reporters with sufficient volume in the
// rolling window and penalizes chronic outliers whose negative rate exceeds 2x the
// network median. (LEP6.md §15.2)
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
		negRate            float64
		negativeCount      uint64
		confirmedNegatives uint64
	}

	qualifying := make([]reporterEntry, 0, len(states))
	startEpoch := storageTruthWindowStart(epochID, uint64(params.StorageTruthDivergenceWindowEpochs))
	for _, state := range states {
		stats, err := k.storageTruthReporterDivergenceStats(ctx, state.ReporterSupernodeAccount, startEpoch, epochID)
		if err != nil {
			return err
		}
		if stats.total == 0 {
			stats.total = uint64(state.WindowPositiveCount) + uint64(state.WindowNegativeCount)
			stats.negative = uint64(state.WindowNegativeCount)
		}
		if stats.total < uint64(minReports) {
			continue
		}
		negRate := float64(stats.negative) / float64(stats.total)
		qualifying = append(qualifying, reporterEntry{
			account:            state.ReporterSupernodeAccount,
			negRate:            negRate,
			negativeCount:      stats.negative,
			confirmedNegatives: stats.confirmedNegative,
		})
	}

	if len(qualifying) == 0 {
		return nil
	}

	// Compute median neg_rate across qualifying reporters.
	rates := make([]float64, len(qualifying))
	for i, e := range qualifying {
		rates[i] = e.negRate
	}
	sort.Float64s(rates)
	medianNegRate := medianFloat64(rates)

	// Penalize reporters whose neg_rate > 2x median.
	for _, entry := range qualifying {
		if entry.negRate <= 2.0*medianNegRate {
			continue
		}
		if entry.negativeCount != 0 && entry.confirmedNegatives*2 >= entry.negativeCount {
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
			sdk.NewAttribute("reporter_neg_rate", strconv.FormatFloat(entry.negRate, 'f', 4, 64)),
			sdk.NewAttribute("median_neg_rate", strconv.FormatFloat(medianNegRate, 'f', 4, 64)),
		))
	}

	return nil
}

type storageTruthDivergenceStats struct {
	total             uint64
	negative          uint64
	confirmedNegative uint64
}

func (k Keeper) storageTruthReporterDivergenceStats(ctx sdk.Context, reporterAccount string, startEpoch uint64, endEpoch uint64) (storageTruthDivergenceStats, error) {
	var stats storageTruthDivergenceStats
	prefix := types.ReporterStorageTruthResultPrefix(reporterAccount)
	it := k.kvStore(ctx).Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer it.Close()
	for ; it.Valid(); it.Next() {
		var record storageTruthReporterResultRecord
		if err := json.Unmarshal(it.Value(), &record); err != nil {
			return stats, err
		}
		if record.EpochID < startEpoch || record.EpochID > endEpoch {
			continue
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

func medianFloat64(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2.0
}
