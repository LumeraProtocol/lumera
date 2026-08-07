package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// MigrateSupernode updates the SupernodeAccount field if legacyAddr is a supernode.
// Also records the migration in PrevSupernodeAccounts history.
func (k Keeper) MigrateSupernode(ctx sdk.Context, legacyAddr, newAddr sdk.AccAddress) error {
	sn, found, err := k.supernodeKeeper.StrictGetSuperNodeByAccount(ctx, legacyAddr.String())
	if err != nil {
		return fmt.Errorf("resolve source supernode ownership: %w", err)
	}
	return k.migrateValidatedSupernode(ctx, newAddr, sn, found)
}

// migrateValidatedSupernode mutates the exact record returned by the strict
// pre-mutation ownership lookup performed by ClaimLegacyAccount.
func (k Keeper) migrateValidatedSupernode(ctx sdk.Context, newAddr sdk.AccAddress, sn sntypes.SuperNode, found bool) error {
	if !found {
		return nil
	}

	// Update the supernode account field to new address.
	sn.SupernodeAccount = newAddr.String()

	// Preserve the existing account timeline verbatim. Migration changes the
	// effective account, so append exactly one transition at this block height.
	// Record the migration as a new account-history entry.
	sn.PrevSupernodeAccounts = append(sn.PrevSupernodeAccounts, &sntypes.SupernodeAccountHistory{
		Account: newAddr.String(),
		Height:  ctx.BlockHeight(),
	})

	return k.supernodeKeeper.SetSuperNode(ctx, sn)
}
