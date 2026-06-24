package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// DeleteValidatorRecordNoHooks removes ONLY the main validator KV entry at
// ValidatorsKey(oldValAddr) from x/staking's store. It does NOT:
//   - call RemoveValidator
//   - fire any staking hook (notably AfterValidatorRemoved, which would destroy
//     distribution state we still need)
//   - touch ValidatorByConsAddr
//   - touch the power index
//   - touch LastValidatorPower
//   - touch distribution / unbonding / redelegation state
//
// This is the narrow missing step needed after MigrateValidatorRecord +
// MigrateValidatorDistribution + MigrateValidatorDelegations have re-homed
// all validator state under newValAddr: without this delete, staking's main
// validator store still has a dead row at oldValAddr that surfaces in
// IterateValidators / GetAllValidators / genesis export.
//
// UNSAFE / MIGRATION-ONLY. Call only after all other re-keying in
// MigrateValidator has succeeded, just before finalizeMigration.
//
// Preconditions (asserted):
//   - stakingStoreHandle.svc has been wired (via SetStakingStoreService).
//   - newValAddr already has a validator record (re-keying succeeded).
//
// On any precondition failure it returns an error and writes nothing.
func (k Keeper) DeleteValidatorRecordNoHooks(ctx sdk.Context, oldValAddr, newValAddr sdk.ValAddress) error {
	if k.stakingStoreHandle == nil || k.stakingStoreHandle.svc == nil {
		return fmt.Errorf("DeleteValidatorRecordNoHooks: staking store service not wired; call SetStakingStoreService from app.go")
	}

	// Assert the new row already exists — if not, we'd leave the chain with
	// zero validator records for the original operator.
	if _, err := k.stakingKeeper.GetValidator(ctx, newValAddr); err != nil {
		return fmt.Errorf("DeleteValidatorRecordNoHooks: new validator record at %s not found (re-keying must precede delete): %w", newValAddr, err)
	}

	store := k.stakingStoreHandle.svc.OpenKVStore(ctx)
	return store.Delete(stakingtypes.GetValidatorKey(oldValAddr))
}
