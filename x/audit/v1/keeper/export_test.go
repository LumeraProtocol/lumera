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
