package keeper

import sdk "github.com/cosmos/cosmos-sdk/types"

const maxAuditEpochLookback uint64 = 16

// getLatestCascadeBytesFromAudit returns the latest available cascade bytes and report height
// from audit epoch reports for the given supernode account.
func (k Keeper) getLatestCascadeBytesFromAudit(ctx sdk.Context, supernodeAccount string) (float64, int64, bool) {
	if k.auditKeeper == nil || supernodeAccount == "" {
		return 0, 0, false
	}

	currentEpochID, _, _, err := k.auditKeeper.GetCurrentEpochInfo(ctx)
	if err != nil {
		k.Logger().Error("failed to derive current audit epoch", "err", err)
		return 0, 0, false
	}

	for offset := uint64(0); offset <= maxAuditEpochLookback && offset <= currentEpochID; offset++ {
		epochID := currentEpochID - offset
		report, found := k.auditKeeper.GetReport(ctx, epochID, supernodeAccount)
		if !found {
			continue
		}
		return report.HostReport.CascadeKademliaDbBytes, report.ReportHeight, true
	}

	return 0, 0, false
}

func isFreshByBlockHeight(currentHeight, reportHeight int64, maxBlocks uint64) bool {
	if reportHeight <= 0 {
		return false
	}
	if currentHeight < reportHeight {
		return false
	}
	if maxBlocks == 0 {
		return true
	}
	return uint64(currentHeight-reportHeight) <= maxBlocks
}
