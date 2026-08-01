package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// MigrateAuthz atomically re-keys grants where the account is granter/grantee
// and rewrites embedded StakeAuthorization validator allow/deny references.
// Production handlers use the same prebuilt fragment through retainedStatePlan.
func (k Keeper) MigrateAuthz(ctx sdk.Context, legacyAddr, newAddr sdk.AccAddress) error {
	params, err := k.Params.Get(ctx)
	if err != nil {
		return err
	}
	moves, err := k.buildAuthzPlan(ctx, legacyAddr, newAddr, params.MaxRetainedStateEntries)
	if err != nil {
		return err
	}
	cacheCtx, commit := ctx.CacheContext()
	plan := retainedStatePlan{authz: moves, legacyAddr: legacyAddr, newAddr: newAddr}
	if err := k.applyRetainedStatePlan(cacheCtx, plan); err != nil {
		return err
	}
	commit()
	return nil
}
