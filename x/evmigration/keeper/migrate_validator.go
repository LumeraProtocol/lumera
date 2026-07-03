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
// Note: MigrateValidatorRecord intentionally leaves the old validator KV row
// untouched — RemoveValidator rejects bonded validators and its
// AfterValidatorRemoved hook destroys distribution state mid-migration.
// The final delete is performed by DeleteValidatorRecordNoHooks at the end
// of MigrateValidator, after all state has been re-homed under newValAddr.
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
//
// The delegations, unbonding delegations, and redelegations are supplied by the caller
// (MigrateValidator), which already read them for the pre-migration
// MaxValidatorDelegations count check. Threading them in avoids a second O(N)
// scan of the same staking records on the hot path — for a validator with
// thousands of delegations that redundant read dominated the block time. Passing
// redelegations also avoids a second validator-scoped index scan. Tests may pass
// nil redelegations to exercise the internal scoped scan directly.
func (k Keeper) MigrateValidatorDelegations(
	ctx sdk.Context,
	oldValAddr, newValAddr sdk.ValAddress,
	delegations []stakingtypes.Delegation,
	ubds []stakingtypes.UnbondingDelegation,
	reds []stakingtypes.Redelegation,
) error {
	// All delegations reference the same period (currentRewards.Period - 1). Its
	// reference count becomes base(1) + one per re-keyed delegation. Set it in a
	// single write here instead of resetting to 1 and incrementing once per
	// delegation, which cost N+1 full-chain scans of ValidatorHistoricalRewards.
	var (
		targetPeriod uint64
		validator    stakingtypes.Validator
	)
	if len(delegations) > 0 {
		currentRewards, err := k.distributionKeeper.GetValidatorCurrentRewards(ctx, newValAddr)
		if err != nil {
			return err
		}
		targetPeriod = currentRewards.Period - 1
		if err := k.setHistoricalRewardsReferenceCount(ctx, newValAddr, targetPeriod, uint32(1+len(delegations))); err != nil {
			return err
		}
		// Needed to convert delegation shares to token stake below. The validator
		// record was re-keyed to newValAddr in step V2, so read it from there.
		validator, err = k.stakingKeeper.GetValidator(ctx, newValAddr)
		if err != nil {
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

		// Initialize fresh distribution starting info for (newValAddr, delegator).
		// The old starting info was deleted above, so we always construct new info.
		// The historical rewards reference count for targetPeriod was already set
		// to base(1) + len(delegations) in one write above.
		//
		// Stake must be the delegation's TOKEN value (shares × exchange rate,
		// truncated), matching the SDK's initializeDelegation. Storing raw shares
		// overstates the stake for any validator that has ever been slashed
		// (tokens < shares), and calculateDelegationRewards then panics with
		// "calculated final stake ... greater than current stake" on every
		// subsequent withdrawal or share-modifying tx from a migrated delegator.
		startingInfo := distrtypes.DelegatorStartingInfo{
			PreviousPeriod: targetPeriod,
			Height:         uint64(ctx.BlockHeight()),
			Stake:          validator.TokensFromSharesTruncated(del.Shares),
		}
		if err := k.distributionKeeper.SetDelegatorStartingInfo(ctx, newValAddr, delAddr, startingInfo); err != nil {
			return err
		}
	}

	// Re-key unbonding delegations. (ubds supplied by the caller.)
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

	// Re-key redelegations where oldValAddr appears as either source or
	// destination validator. Existing in-flight redelegations must continue to
	// point at the migrated validator record after operator migration.
	if reds == nil {
		var err error
		reds, err = k.redelegationsForValidator(ctx, oldValAddr)
		if err != nil {
			return err
		}
	}

	for _, red := range reds {
		if err := k.stakingKeeper.RemoveRedelegation(ctx, red); err != nil {
			return err
		}

		newRed := stakingtypes.Redelegation{
			DelegatorAddress:    red.DelegatorAddress,
			ValidatorSrcAddress: red.ValidatorSrcAddress,
			ValidatorDstAddress: red.ValidatorDstAddress,
			Entries:             red.Entries,
		}
		if red.ValidatorSrcAddress == oldValAddr.String() {
			newRed.ValidatorSrcAddress = newValAddr.String()
		}
		if red.ValidatorDstAddress == oldValAddr.String() {
			newRed.ValidatorDstAddress = newValAddr.String()
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
	historicalRewards, err := k.validatorHistoricalRewards(ctx, oldValAddr)
	if err != nil {
		return err
	}
	k.distributionKeeper.DeleteValidatorHistoricalRewards(ctx, oldValAddr)
	for _, hr := range historicalRewards {
		if err := k.distributionKeeper.SetValidatorHistoricalRewards(ctx, newValAddr, hr.period, hr.rewards); err != nil {
			return err
		}
	}

	// ValidatorSlashEvents — collect all for oldValAddr, then re-key.
	slashEvents, err := k.validatorSlashEvents(ctx, oldValAddr)
	if err != nil {
		return err
	}
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

	// Remove the old primary record and secondary account index before writing
	// the re-keyed record under the new valoper. This avoids a false collision
	// when the supernode account was already migrated independently.
	k.supernodeKeeper.DeleteSuperNode(ctx, oldValAddr)

	// Update validator address to new valoper.
	sn.ValidatorAddress = newValAddr.String()

	// Only update SupernodeAccount (and its history) if it matches the
	// validator's legacy address — i.e. the validator was its own supernode
	// account. A supernode account that belongs to a different entity (or was
	// already migrated independently via ClaimLegacyAccount / supernode-setup)
	// is preserved, and its history is not touched.
	legacyAddrStr := legacyAddr.String()
	if sn.SupernodeAccount == legacyAddrStr {
		sn.SupernodeAccount = newAddr.String()

		// Rewrite existing history entries that reference the legacy address.
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
	}

	// Update validator address in embedded evidence records.
	oldValAddrStr := oldValAddr.String()
	for i := range sn.Evidence {
		if sn.Evidence[i].ValidatorAddress == oldValAddrStr {
			sn.Evidence[i].ValidatorAddress = newValAddr.String()
		}
	}

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
