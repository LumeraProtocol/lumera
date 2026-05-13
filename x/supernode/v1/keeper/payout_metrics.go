package keeper

import sdk "github.com/cosmos/cosmos-sdk/types"

// GetLatestCascadeBytesForPayout returns the latest audit-sourced cascade bytes for a supernode.
func (k Keeper) GetLatestCascadeBytesForPayout(ctx sdk.Context, supernodeAccount string) (float64, int64, bool) {
	return k.getLatestCascadeBytesFromAudit(ctx, supernodeAccount)
}
