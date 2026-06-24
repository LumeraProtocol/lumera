package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// MigrateBank transfers all coin balances from legacyAddr to newAddr.
// Must be called AFTER MigrateAuth removes any vesting lock.
func (k Keeper) MigrateBank(ctx sdk.Context, legacyAddr, newAddr sdk.AccAddress) error {
	balances := k.bankKeeper.GetAllBalances(ctx, legacyAddr)
	if balances.IsZero() {
		return nil
	}

	return k.bankKeeper.SendCoins(ctx, legacyAddr, newAddr, balances)
}
