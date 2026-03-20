package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// MigrateStaking re-keys all delegations, unbonding delegations, and redelegations
// from legacyAddr to newAddr. origWithdrawAddr is the withdraw address that was
// set *before* MigrateDistribution may have temporarily redirected it to self.
func (k Keeper) MigrateStaking(ctx sdk.Context, legacyAddr, newAddr, origWithdrawAddr sdk.AccAddress) error {
	// Active delegations.
	if err := k.migrateActiveDelegations(ctx, legacyAddr, newAddr); err != nil {
		return err
	}

	// Unbonding delegations.
	if err := k.migrateUnbondingDelegations(ctx, legacyAddr, newAddr); err != nil {
		return err
	}

	// Redelegations — we need to check all validators the legacy address has
	// redelegations from. Get delegator's redelegations by iterating all validators.
	if err := k.migrateRedelegations(ctx, legacyAddr, newAddr); err != nil {
		return err
	}

	// Migrate withdraw address using the original (pre-redirect) value.
	return k.migrateWithdrawAddress(ctx, legacyAddr, newAddr, origWithdrawAddr)
}

// migrateActiveDelegations re-keys all active delegations and their distribution
// starting info from legacyAddr to newAddr.
func (k Keeper) migrateActiveDelegations(ctx sdk.Context, legacyAddr, newAddr sdk.AccAddress) error {
	delegations, err := k.stakingKeeper.GetDelegatorDelegations(ctx, legacyAddr, ^uint16(0))
	if err != nil {
		return err
	}

	for _, del := range delegations {
		valAddr, err := sdk.ValAddressFromBech32(del.ValidatorAddress)
		if err != nil {
			return err
		}

		// Delete old distribution starting info.
		if err := k.distributionKeeper.DeleteDelegatorStartingInfo(ctx, valAddr, legacyAddr); err != nil {
			return err
		}

		// Remove old delegation.
		if err := k.stakingKeeper.RemoveDelegation(ctx, del); err != nil {
			return err
		}

		// Create new delegation with same shares.
		newDel := stakingtypes.NewDelegation(newAddr.String(), del.ValidatorAddress, del.Shares)
		if err := k.stakingKeeper.SetDelegation(ctx, newDel); err != nil {
			return err
		}

		// Initialize fresh distribution starting info for the new delegation.
		// The old starting info was deleted above, so we always create new info
		// anchored at the current block height and rewards period.
		currentRewards, err := k.distributionKeeper.GetValidatorCurrentRewards(ctx, valAddr)
		if err != nil {
			return err
		}
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		previousPeriod := currentRewards.Period - 1
		startingInfo := distrtypes.DelegatorStartingInfo{
			Height:         uint64(sdkCtx.BlockHeight()),
			PreviousPeriod: previousPeriod,
			Stake:          del.Shares,
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

// migrateUnbondingDelegations re-keys all unbonding delegations from legacyAddr
// to newAddr, including unbonding queue entries and UnbondingID indexes.
func (k Keeper) migrateUnbondingDelegations(ctx sdk.Context, legacyAddr, newAddr sdk.AccAddress) error {
	unbondings, err := k.stakingKeeper.GetUnbondingDelegations(ctx, legacyAddr, ^uint16(0))
	if err != nil {
		return err
	}

	for _, ubd := range unbondings {
		// Remove old unbonding delegation.
		// The full record is already loaded, so we do not need to rediscover it
		// through active delegations, which would miss validators that were fully
		// undelegated before migration.
		if err := k.stakingKeeper.RemoveUnbondingDelegation(ctx, ubd); err != nil {
			return err
		}

		// Create new with same entries but newAddr as delegator.
		newUbd := stakingtypes.UnbondingDelegation{
			DelegatorAddress: newAddr.String(),
			ValidatorAddress: ubd.ValidatorAddress,
			Entries:          ubd.Entries,
		}
		if err := k.stakingKeeper.SetUnbondingDelegation(ctx, newUbd); err != nil {
			return err
		}

		// Re-insert into unbonding queue and re-key UnbondingID indexes.
		for _, entry := range newUbd.Entries {
			if err := k.stakingKeeper.InsertUBDQueue(ctx, newUbd, entry.CompletionTime); err != nil {
				return err
			}
			if entry.UnbondingId > 0 {
				if err := k.stakingKeeper.SetUnbondingDelegationByUnbondingID(ctx, newUbd, entry.UnbondingId); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// migrateRedelegations re-keys all redelegations where legacyAddr is the
// delegator, including redelegation queue entries and UnbondingID indexes.
func (k Keeper) migrateRedelegations(ctx sdk.Context, legacyAddr, newAddr sdk.AccAddress) error {
	redelegations, err := k.stakingKeeper.GetRedelegations(ctx, legacyAddr, ^uint16(0))
	if err != nil {
		return err
	}

	for _, red := range redelegations {
		// Remove old redelegation.
		if err := k.stakingKeeper.RemoveRedelegation(ctx, red); err != nil {
			return err
		}

		// Create new with newAddr as delegator.
		newRed := stakingtypes.Redelegation{
			DelegatorAddress:    newAddr.String(),
			ValidatorSrcAddress: red.ValidatorSrcAddress,
			ValidatorDstAddress: red.ValidatorDstAddress,
			Entries:             red.Entries,
		}
		if err := k.stakingKeeper.SetRedelegation(ctx, newRed); err != nil {
			return err
		}

		// Re-insert into queue and re-key UnbondingID indexes.
		for _, entry := range newRed.Entries {
			if err := k.stakingKeeper.InsertRedelegationQueue(ctx, newRed, entry.CompletionTime); err != nil {
				return err
			}
			if entry.UnbondingId > 0 {
				if err := k.stakingKeeper.SetRedelegationByUnbondingID(ctx, newRed, entry.UnbondingId); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// migrateWithdrawAddress updates the delegator withdraw address. origWithdrawAddr
// is the withdraw address that was set before MigrateDistribution may have
// temporarily redirected it to self for safe reward withdrawal.
func (k Keeper) migrateWithdrawAddress(ctx sdk.Context, legacyAddr, newAddr, origWithdrawAddr sdk.AccAddress) error {
	// If the original withdraw address was self (legacy) or nil, update to new address.
	if origWithdrawAddr == nil || origWithdrawAddr.Equals(legacyAddr) {
		return k.distributionKeeper.SetDelegatorWithdrawAddr(ctx, newAddr, newAddr)
	}

	// Third-party withdraw address: if it was migrated, follow the record
	// to the new address so future rewards reach the right account.
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
