package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// MigrateSupernode updates the SupernodeAccount field if legacyAddr is a supernode.
// Also records the migration in PrevSupernodeAccounts history.
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

	// Update legacy address references in existing history entries.
	legacyAddrStr := legacyAddr.String()
	for i := range sn.PrevSupernodeAccounts {
		if sn.PrevSupernodeAccounts[i].Account == legacyAddrStr {
			sn.PrevSupernodeAccounts[i].Account = newAddr.String()
		}
	}

	// Record the migration as a new account-history entry.
	sn.PrevSupernodeAccounts = append(sn.PrevSupernodeAccounts, &sntypes.SupernodeAccountHistory{
		Account: newAddr.String(),
		Height:  ctx.BlockHeight(),
	})

	return k.supernodeKeeper.SetSuperNode(ctx, sn)
}
