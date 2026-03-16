package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// MigrateValidatorRecord re-keys the validator record from oldValAddr to newValAddr.
// Updates power index, last validator power, and ConsAddr mapping.
//
// Note: the old validator KV entry at oldValAddr is left orphaned rather than
// deleted. RemoveValidator cannot be used because (a) it rejects bonded
// validators and (b) its AfterValidatorRemoved hook destroys distribution
// state needed for migration. The orphaned record is inert: its power index
// is removed, all delegations/distribution/ConsAddr point to newValAddr, and
// the migration-records check prevents re-migration.
func (k Keeper) MigrateValidatorRecord(ctx sdk.Context, oldValAddr, newValAddr sdk.ValAddress) error {
	val, err := k.stakingKeeper.GetValidator(ctx, oldValAddr)
	if err != nil {
		return err
	}

	// Remove old power index entry before modifying.
	if err := k.stakingKeeper.DeleteValidatorByPowerIndex(ctx, val); err != nil {
		return err
	}

	// Update operator address (must use valoper bech32 prefix).
	val.OperatorAddress = newValAddr.String()

	// Store new validator record at the new address key.
	if err := k.stakingKeeper.SetValidator(ctx, val); err != nil {
		return err
	}

	// Re-create power index for the new address.
	if err := k.stakingKeeper.SetValidatorByPowerIndex(ctx, val); err != nil {
		return err
	}

	// Re-key LastValidatorPower.
	power, err := k.stakingKeeper.GetLastValidatorPower(ctx, oldValAddr)
	if err == nil && power > 0 {
		if err := k.stakingKeeper.DeleteLastValidatorPower(ctx, oldValAddr); err != nil {
			return err
		}
		if err := k.stakingKeeper.SetLastValidatorPower(ctx, newValAddr, power); err != nil {
			return err
		}
	}

	// Re-key ValidatorByConsAddr mapping: ConsAddr → newValAddr.
	if err := k.stakingKeeper.SetValidatorByConsAddr(ctx, val); err != nil {
		return err
	}

	return nil
}

// MigrateValidatorDelegations re-keys all delegations pointing to oldValAddr
// to point to newValAddr. This affects ALL delegators, not just the operator.
func (k Keeper) MigrateValidatorDelegations(ctx sdk.Context, oldValAddr, newValAddr sdk.ValAddress) error {
	// Re-key active delegations.
	delegations, err := k.stakingKeeper.GetValidatorDelegations(ctx, oldValAddr)
	if err != nil {
		return err
	}

	// All delegations will reference the same period (currentRewards.Period - 1).
	// Reset its reference count to 1 (base) since old delegator references are stale
	// after re-keying distribution state.
	var targetPeriod uint64
	if len(delegations) > 0 {
		currentRewards, err := k.distributionKeeper.GetValidatorCurrentRewards(ctx, newValAddr)
		if err != nil {
			return err
		}
		targetPeriod = currentRewards.Period - 1
		if err := k.resetHistoricalRewardsReferenceCount(ctx, newValAddr, targetPeriod); err != nil {
			return err
		}
	}

	for _, del := range delegations {
		// Delete old distribution starting info.
		delAddr, err := sdk.AccAddressFromBech32(del.DelegatorAddress)
		if err != nil {
			return err
		}
		if err := k.distributionKeeper.DeleteDelegatorStartingInfo(ctx, oldValAddr, delAddr); err != nil {
			return err
		}

		// Remove old delegation.
		if err := k.stakingKeeper.RemoveDelegation(ctx, del); err != nil {
			return err
		}

		// Create new delegation pointing to newValAddr.
		newDel := stakingtypes.NewDelegation(del.DelegatorAddress, newValAddr.String(), del.Shares)
		if err := k.stakingKeeper.SetDelegation(ctx, newDel); err != nil {
			return err
		}

		// Initialize distribution starting info for (newValAddr, delegator).
		startingInfo, _ := k.distributionKeeper.GetDelegatorStartingInfo(ctx, oldValAddr, delAddr)
		startingInfo.PreviousPeriod = targetPeriod
		startingInfo.Height = uint64(ctx.BlockHeight())
		startingInfo.Stake = del.Shares
		if err := k.incrementHistoricalRewardsReferenceCount(ctx, newValAddr, targetPeriod); err != nil {
			return err
		}
		if err := k.distributionKeeper.SetDelegatorStartingInfo(ctx, newValAddr, delAddr, startingInfo); err != nil {
			return err
		}
	}

	// Re-key unbonding delegations.
	ubds, err := k.stakingKeeper.GetUnbondingDelegationsFromValidator(ctx, oldValAddr)
	if err != nil {
		return err
	}

	for _, ubd := range ubds {
		if err := k.stakingKeeper.RemoveUnbondingDelegation(ctx, ubd); err != nil {
			return err
		}

		newUbd := stakingtypes.UnbondingDelegation{
			DelegatorAddress: ubd.DelegatorAddress,
			ValidatorAddress: newValAddr.String(),
			Entries:          ubd.Entries,
		}
		if err := k.stakingKeeper.SetUnbondingDelegation(ctx, newUbd); err != nil {
			return err
		}

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

	// Re-key redelegations where oldValAddr is source.
	reds, err := k.stakingKeeper.GetRedelegationsFromSrcValidator(ctx, oldValAddr)
	if err != nil {
		return err
	}

	for _, red := range reds {
		if err := k.stakingKeeper.RemoveRedelegation(ctx, red); err != nil {
			return err
		}

		newRed := stakingtypes.Redelegation{
			DelegatorAddress:    red.DelegatorAddress,
			ValidatorSrcAddress: newValAddr.String(),
			ValidatorDstAddress: red.ValidatorDstAddress,
			Entries:             red.Entries,
		}
		if err := k.stakingKeeper.SetRedelegation(ctx, newRed); err != nil {
			return err
		}

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

// MigrateValidatorDistribution re-keys all distribution state keyed by ValAddr.
func (k Keeper) MigrateValidatorDistribution(ctx sdk.Context, oldValAddr, newValAddr sdk.ValAddress) error {
	// ValidatorCurrentRewards.
	currentRewards, err := k.distributionKeeper.GetValidatorCurrentRewards(ctx, oldValAddr)
	if err == nil {
		if err := k.distributionKeeper.DeleteValidatorCurrentRewards(ctx, oldValAddr); err != nil {
			return err
		}
		if err := k.distributionKeeper.SetValidatorCurrentRewards(ctx, newValAddr, currentRewards); err != nil {
			return err
		}
	}

	// ValidatorAccumulatedCommission.
	commission, err := k.distributionKeeper.GetValidatorAccumulatedCommission(ctx, oldValAddr)
	if err == nil {
		if err := k.distributionKeeper.DeleteValidatorAccumulatedCommission(ctx, oldValAddr); err != nil {
			return err
		}
		if err := k.distributionKeeper.SetValidatorAccumulatedCommission(ctx, newValAddr, commission); err != nil {
			return err
		}
	}

	// ValidatorOutstandingRewards.
	outstanding, err := k.distributionKeeper.GetValidatorOutstandingRewards(ctx, oldValAddr)
	if err == nil {
		if err := k.distributionKeeper.DeleteValidatorOutstandingRewards(ctx, oldValAddr); err != nil {
			return err
		}
		if err := k.distributionKeeper.SetValidatorOutstandingRewards(ctx, newValAddr, outstanding); err != nil {
			return err
		}
	}

	// ValidatorHistoricalRewards — collect all periods for oldValAddr, then re-key.
	type historicalEntry struct {
		period  uint64
		rewards distrtypes.ValidatorHistoricalRewards
	}
	var historicalRewards []historicalEntry
	k.distributionKeeper.IterateValidatorHistoricalRewards(ctx, func(val sdk.ValAddress, period uint64, rewards distrtypes.ValidatorHistoricalRewards) (stop bool) {
		if val.Equals(oldValAddr) {
			historicalRewards = append(historicalRewards, historicalEntry{period, rewards})
		}
		return false
	})
	k.distributionKeeper.DeleteValidatorHistoricalRewards(ctx, oldValAddr)
	for _, hr := range historicalRewards {
		if err := k.distributionKeeper.SetValidatorHistoricalRewards(ctx, newValAddr, hr.period, hr.rewards); err != nil {
			return err
		}
	}

	// ValidatorSlashEvents — collect all for oldValAddr, then re-key.
	type slashEntry struct {
		height uint64
		event  distrtypes.ValidatorSlashEvent
	}
	var slashEvents []slashEntry
	k.distributionKeeper.IterateValidatorSlashEvents(ctx, func(val sdk.ValAddress, height uint64, event distrtypes.ValidatorSlashEvent) (stop bool) {
		if val.Equals(oldValAddr) {
			slashEvents = append(slashEvents, slashEntry{height, event})
		}
		return false
	})
	k.distributionKeeper.DeleteValidatorSlashEvents(ctx, oldValAddr)
	for _, se := range slashEvents {
		if err := k.distributionKeeper.SetValidatorSlashEvent(ctx, newValAddr, se.height, se.event.ValidatorPeriod, se.event); err != nil {
			return err
		}
	}

	return nil
}

// MigrateValidatorSupernode re-keys the supernode record from oldValAddr to newValAddr.
// The supernode's account field is only updated when it matches the validator's
// legacy address (i.e. the validator was its own supernode account). If the
// supernode account is a separate entity (possibly already migrated independently),
// it is left unchanged.
func (k Keeper) MigrateValidatorSupernode(ctx sdk.Context, oldValAddr, newValAddr sdk.ValAddress, legacyAddr, newAddr sdk.AccAddress) error {
	sn, found := k.supernodeKeeper.QuerySuperNode(ctx, oldValAddr)
	if !found {
		return nil
	}

	// Update validator address to new valoper.
	sn.ValidatorAddress = sdk.ValAddress(newAddr).String()

	// Only update SupernodeAccount if it matches the validator's legacy address.
	// A supernode account that belongs to a different entity (or was already
	// migrated independently via ClaimLegacyAccount / supernode-setup) is preserved.
	legacyAddrStr := legacyAddr.String()
	if sn.SupernodeAccount == legacyAddrStr {
		sn.SupernodeAccount = newAddr.String()
	}

	// Update validator address in embedded evidence records.
	oldValAddrStr := oldValAddr.String()
	for i := range sn.Evidence {
		if sn.Evidence[i].ValidatorAddress == oldValAddrStr {
			sn.Evidence[i].ValidatorAddress = newValAddr.String()
		}
	}

	// Update account address in supernode account history only where it
	// matches the validator's legacy address.
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

	// Migrate metrics state: write under new key, delete old key.
	metrics, found := k.supernodeKeeper.GetMetricsState(ctx, oldValAddr)
	if found {
		metrics.ValidatorAddress = newValAddr.String()
		if err := k.supernodeKeeper.SetMetricsState(ctx, metrics); err != nil {
			return err
		}
		k.supernodeKeeper.DeleteMetricsState(ctx, oldValAddr)
	}

	return k.supernodeKeeper.SetSuperNode(ctx, sn)
}

// MigrateValidatorActions updates action records that reference legacyAddr
// in their SuperNodes field.
func (k Keeper) MigrateValidatorActions(ctx sdk.Context, legacyAddr, newAddr sdk.AccAddress) error {
	return k.MigrateActions(ctx, legacyAddr, newAddr)
}
