package keeper

import (
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
