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

	// Sort by neg/total ratio using integer cross-multiply to avoid float64 non-determinism.
	// a.negative/a.total < b.negative/b.total  ⟺  a.negative*b.total < b.negative*a.total
	sort.Slice(qualifying, func(i, j int) bool {
		return qualifying[i].negative*qualifying[j].total < qualifying[j].negative*qualifying[i].total
	})

	// Compute median neg-rate as an integer pair (medianNeg, medianTotal).
	// For even-length slices, use the lower-median element to stay conservative.
	mid := len(qualifying) / 2
	var medianNeg, medianTotal uint64
	if len(qualifying)%2 == 1 {
		medianNeg = qualifying[mid].negative
		medianTotal = qualifying[mid].total
	} else {
		medianNeg = qualifying[mid-1].negative
		medianTotal = qualifying[mid-1].total
	}

	if medianTotal == 0 {
		return nil
	}

	// Penalize reporters whose neg_rate > 2x median.
	// entry.negative/entry.total > 2*medianNeg/medianTotal
	//   ⟺ entry.negative * medianTotal > 2 * medianNeg * entry.total
	for _, entry := range qualifying {
		if entry.total == 0 || entry.negative*medianTotal <= 2*medianNeg*entry.total {
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
