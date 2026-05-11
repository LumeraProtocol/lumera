package keeper

import sdk "github.com/cosmos/cosmos-sdk/types"

// getLatestCascadeBytesFromAudit returns the latest available cascade bytes and report height
// for the given supernode account. In LEP-6 §12, CascadeKademliaDbBytes was removed from the
// audit module's HostReport; this function now reads from the supernode's own stored metrics.
func (k Keeper) getLatestCascadeBytesFromAudit(ctx sdk.Context, supernodeAccount string) (float64, int64, bool) {
	if supernodeAccount == "" {
		return 0, 0, false
	}
	sn, found, err := k.GetSuperNodeByAccount(ctx, supernodeAccount)
	if err != nil || !found {
		return 0, 0, false
	}
	valAddr, err := sdk.ValAddressFromBech32(sn.ValidatorAddress)
	if err != nil {
		return 0, 0, false
	}
	state, ok := k.GetMetricsState(ctx, valAddr)
	if !ok || state.Metrics == nil {
		return 0, 0, false
	}
	return state.Metrics.CascadeKademliaDbBytes, state.Height, true
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
