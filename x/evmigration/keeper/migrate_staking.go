package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// MigrateStaking atomically re-keys an account's staking state. The complete
// UBD/RED primary and maturity-queue plan is validated before the cache context
// receives its first write.
func (k Keeper) MigrateStaking(ctx sdk.Context, legacyAddr, newAddr, origWithdrawAddr sdk.AccAddress) error {
	cacheCtx, commit := ctx.CacheContext()
	delegations, ubds, reds, err := k.accountStakingRecords(cacheCtx, legacyAddr)
	if err != nil {
		return err
	}
	plan, err := k.buildStakingMigrationPlan(cacheCtx, delegations, ubds, reds, stakingAddressTransform{
		oldDelegator: legacyAddr,
		newDelegator: newAddr,
	})
	if err != nil {
		return err
	}
	if err := k.migrateAccountStakingWithPlan(cacheCtx, legacyAddr, newAddr, origWithdrawAddr, delegations, plan); err != nil {
		return err
	}
	commit()
	return nil
}

func (k Keeper) migrateAccountStakingWithPlan(
	ctx sdk.Context,
	legacyAddr, newAddr, origWithdrawAddr sdk.AccAddress,
	delegations []stakingtypes.Delegation,
	plan stakingMigrationPlan,
) error {
	// Validate all raw source/destination primaries and apply queue-backed records
	// before keeper calls mutate active delegation primaries.
	if err := k.applyStakingMigrationPlan(ctx, plan, false); err != nil {
		return err
	}
	if err := k.migrateActiveDelegations(ctx, legacyAddr, newAddr, delegations); err != nil {
		return err
	}
	return k.migrateWithdrawAddress(ctx, legacyAddr, newAddr, origWithdrawAddr)
}

// migrateActiveDelegations re-keys supplied active delegations and their
// distribution starting info. Supplying the preflight snapshot avoids a second
// unbounded keeper query after writes begin.
func (k Keeper) migrateActiveDelegations(ctx sdk.Context, legacyAddr, newAddr sdk.AccAddress, delegations []stakingtypes.Delegation) error {
	for _, del := range delegations {
		if del.DelegatorAddress != legacyAddr.String() {
			return fmt.Errorf("delegation snapshot contains unexpected delegator %s", del.DelegatorAddress)
		}
		valAddr, err := sdk.ValAddressFromBech32(del.ValidatorAddress)
		if err != nil {
			return err
		}

		if err := k.distributionKeeper.DeleteDelegatorStartingInfo(ctx, valAddr, legacyAddr); err != nil {
			return err
		}
		if err := k.stakingKeeper.RemoveDelegation(ctx, del); err != nil {
			return err
		}
		newDel := stakingtypes.NewDelegation(newAddr.String(), del.ValidatorAddress, del.Shares)
		if err := k.stakingKeeper.SetDelegation(ctx, newDel); err != nil {
			return err
		}

		currentRewards, err := k.distributionKeeper.GetValidatorCurrentRewards(ctx, valAddr)
		if err != nil {
			return err
		}
		previousPeriod := currentRewards.Period - 1
		val, err := k.stakingKeeper.GetValidator(ctx, valAddr)
		if err != nil {
			return err
		}
		startingInfo := distrtypes.DelegatorStartingInfo{
			Height:         uint64(ctx.BlockHeight()),
			PreviousPeriod: previousPeriod,
			Stake:          val.TokensFromSharesTruncated(del.Shares),
		}
		if err := k.incrementHistoricalRewardsReferenceCount(ctx, valAddr, previousPeriod); err != nil {
			return err
		}
		if err := k.distributionKeeper.SetDelegatorStartingInfo(ctx, valAddr, newAddr, startingInfo); err != nil {
			return err
		}
	}
	return nil
}

// migrateWithdrawAddress updates the delegator withdraw address. origWithdrawAddr
// is the withdraw address that was set before MigrateDistribution may have
// temporarily redirected it to self.
func (k Keeper) migrateWithdrawAddress(ctx sdk.Context, legacyAddr, newAddr, origWithdrawAddr sdk.AccAddress) error {
	if origWithdrawAddr == nil || origWithdrawAddr.Equals(legacyAddr) {
		return k.distributionKeeper.SetDelegatorWithdrawAddr(ctx, newAddr, newAddr)
	}

	resolvedAddr := origWithdrawAddr
	record, err := k.MigrationRecords.Get(ctx, origWithdrawAddr.String())
	if err == nil && record.NewAddress != "" {
		resolved, err := sdk.AccAddressFromBech32(record.NewAddress)
		if err == nil {
			resolvedAddr = resolved
		}
	}
	return k.distributionKeeper.SetDelegatorWithdrawAddr(ctx, newAddr, resolvedAddr)
}
