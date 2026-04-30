package keeper

import (
	"encoding/json"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// SetStorageTruthReporterResultForTest exposes the internal setter so
// external (keeper_test) test packages can populate per-record divergence stats.
var SetStorageTruthReporterResultForTest = func(k Keeper, ctx sdk.Context, epochID uint64, reporterAccount string, result *types.StorageProofResult) error {
	return k.setStorageTruthReporterResult(ctx, epochID, reporterAccount, result)
}

// SetStorageTruthNodeFailureForTest exposes the internal setter so external
// (keeper_test) test packages can seed fact-index node failure records.
var SetStorageTruthNodeFailureForTest = func(k Keeper, ctx sdk.Context, epochID uint64, reporterAccount string, result *types.StorageProofResult) error {
	return k.setStorageTruthNodeFailure(ctx, epochID, reporterAccount, result)
}

// ApplyTicketDeteriorationDeltaForTest exposes the internal apply path so
// external tests can exercise sibling-symmetry rules (e.g. the F119-F3 residue
// cross-holder PASS bonus).
var ApplyTicketDeteriorationDeltaForTest = func(k Keeper, ctx sdk.Context, epochID uint64, reporterAccount string, result *types.StorageProofResult, ticketID string, delta int64, decayPerEpoch int64, contradictionConfirmed bool) (types.TicketDeteriorationState, bool, error) {
	return k.applyTicketDeteriorationDelta(ctx, epochID, reporterAccount, result, ticketID, delta, decayPerEpoch, contradictionConfirmed)
}

// WriteRawNextHealOpIDForTest writes raw bytes to the next-heal-op-id store key,
// bypassing the well-formed encoder. Used to test panic-on-malformed (NEW-B-7).
var WriteRawNextHealOpIDForTest = func(k Keeper, ctx sdk.Context, raw []byte) {
	k.kvStore(ctx).Set(types.NextHealOpIDKey(), raw)
}

// SetReporterResultOverturnFlagForTest writes a reporter-result record with the
// OverturnedByRecheck flag explicitly set. Used to verify CP-R3 B-F1 — the
// "no overturned fails" gate must inspect failure-class records, not PASS.
var SetReporterResultOverturnFlagForTest = func(k Keeper, ctx sdk.Context, epochID uint64, reporterAccount string, result *types.StorageProofResult, overturned bool) error {
	if err := k.setStorageTruthReporterResult(ctx, epochID, reporterAccount, result); err != nil {
		return err
	}
	store := k.kvStore(ctx)
	primary := types.ReporterStorageTruthResultKey(reporterAccount, epochID, result.TicketId, result.TargetSupernodeAccount)
	bz := store.Get(primary)
	if bz == nil {
		return nil
	}
	var rec storageTruthReporterResultRecord
	if err := json.Unmarshal(bz, &rec); err != nil {
		return err
	}
	rec.OverturnedByRecheck = overturned
	updated, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	store.Set(primary, updated)
	store.Set(types.ReporterStorageTruthResultByTargetKey(result.TargetSupernodeAccount, epochID, result.TicketId, reporterAccount), updated)
	return nil
}

// HasIndependentReporterPassInWindowForTest exposes the indexed lookup for CP-R3 B-F3 overflow coverage.
var HasIndependentReporterPassInWindowForTest = func(k Keeper, ctx sdk.Context, ticketID string, targetAccount string, excludeReporter string, startEpoch uint64, endEpoch uint64) (bool, error) {
	return k.hasIndependentReporterPassInWindow(ctx, ticketID, targetAccount, excludeReporter, startEpoch, endEpoch)
}

// HasCleanRecheckInWindowForTest exposes the indexed lookup for CP-R3 B-F3 overflow coverage.
var HasCleanRecheckInWindowForTest = func(k Keeper, ctx sdk.Context, ticketID string, targetAccount string, startEpoch uint64, endEpoch uint64) (bool, error) {
	return k.hasCleanRecheckInWindow(ctx, ticketID, targetAccount, startEpoch, endEpoch)
}
