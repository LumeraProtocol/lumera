package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// MigrateSupernode preserves the same continuity guarantees as the production
// account-migration handler. All plans are built before the first write.
func (k Keeper) MigrateSupernode(ctx sdk.Context, legacyAddr, newAddr sdk.AccAddress) error {
	cacheCtx, commit := ctx.CacheContext()
	if err := k.migrateSupernodeWithContinuity(cacheCtx, legacyAddr, newAddr); err != nil {
		return err
	}
	commit()
	return nil
}

func (k Keeper) migrateSupernodeWithContinuity(ctx sdk.Context, legacyAddr, newAddr sdk.AccAddress) error {
	sn, found, err := k.supernodeKeeper.StrictGetSuperNodeByAccount(ctx, legacyAddr.String())
	if err != nil {
		return fmt.Errorf("resolve source supernode ownership: %w", err)
	}
	if !found {
		return nil
	}
	if err := k.validateDestinationSupernodeOwnership(ctx, newAddr); err != nil {
		return err
	}
	auditPlan, err := k.auditKeeper.BuildCurrentAccountTransitionPlan(ctx, legacyAddr.String(), newAddr.String())
	if err != nil {
		return fmt.Errorf("build audit account transition: %w", err)
	}
	if err := k.auditKeeper.ApplyAccountTransitionPlan(ctx, auditPlan); err != nil {
		return fmt.Errorf("apply audit account transition: %w", err)
	}
	return k.migrateValidatedSupernode(ctx, newAddr, sn, true)
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
