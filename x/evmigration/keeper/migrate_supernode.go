package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// MigrateSupernode updates the SupernodeAccount field if legacyAddr is a supernode.
func (k Keeper) MigrateSupernode(ctx sdk.Context, legacyAddr, newAddr sdk.AccAddress) error {
	sn, found, err := k.supernodeKeeper.GetSuperNodeByAccount(ctx, legacyAddr.String())
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	// Update the supernode account field to new address.
	sn.SupernodeAccount = newAddr.String()
	return k.supernodeKeeper.SetSuperNode(ctx, sn)
}
