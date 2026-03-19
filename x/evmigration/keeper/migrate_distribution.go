package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
)

// MigrateDistribution withdraws all pending delegation rewards for legacyAddr,
// materializing them into the legacy bank balance before balances are moved.
func (k Keeper) MigrateDistribution(ctx sdk.Context, legacyAddr sdk.AccAddress) error {
	// Ensure the withdraw address points to legacyAddr itself so that
	// WithdrawDelegationRewards deposits rewards into the legacy bank balance
	// (which MigrateBank will transfer later). Without this, rewards would go
	// to a third-party withdraw address which, if it was a previously-migrated
	// legacy address, would deposit coins into a dead account.
	if err := k.redirectWithdrawAddrIfMigrated(ctx, legacyAddr); err != nil {
		return err
	}

	// Get all delegations for the legacy address.
	delegations, err := k.stakingKeeper.GetDelegatorDelegations(ctx, legacyAddr, ^uint16(0))
	if err != nil {
		return err
	}

	// Withdraw rewards for each delegation.
	for _, del := range delegations {
		valAddr, err := sdk.ValAddressFromBech32(del.ValidatorAddress)
		if err != nil {
			return err
		}
		if err := k.ensureDelegatorStartingInfoReferenceCount(ctx, valAddr, legacyAddr); err != nil {
			return err
		}
		// WithdrawDelegationRewards sends rewards to the delegator's withdraw
		// address which we ensured points to legacyAddr above.
		if _, err := k.distributionKeeper.WithdrawDelegationRewards(ctx, legacyAddr, valAddr); err != nil {
			return err
		}
	}

	return nil
}

// redirectWithdrawAddrIfMigrated checks if legacyAddr's distribution withdraw
// address is a previously-migrated legacy address. If so, it resets the
// withdraw address to legacyAddr itself so that subsequent reward withdrawals
// deposit into the account being migrated rather than a dead legacy address.
func (k Keeper) redirectWithdrawAddrIfMigrated(ctx sdk.Context, legacyAddr sdk.AccAddress) error {
	withdrawAddr, err := k.distributionKeeper.GetDelegatorWithdrawAddr(ctx, legacyAddr)
	if err != nil {
		return nil // No custom withdraw address — default (self) is fine.
	}

	// If already pointing to self, nothing to do.
	if withdrawAddr.Equals(legacyAddr) {
		return nil
	}

	// Check if the third-party withdraw address was already migrated.
	has, err := k.MigrationRecords.Has(ctx, withdrawAddr.String())
	if err != nil || !has {
		return nil // Not migrated — leave the third-party address as-is.
	}

	// The withdraw address is a dead legacy address. Temporarily redirect
	// to self so rewards land in legacyAddr's bank balance for transfer.
	return k.distributionKeeper.SetDelegatorWithdrawAddr(ctx, legacyAddr, legacyAddr)
}

// temporaryRedirectWithdrawAddr checks if addr's withdraw address points to an
// already-migrated legacy address. If so, it redirects to self and returns the
// original address + restored=true so the caller can restore it after the
// withdrawal. This avoids the permanent clobbering that redirectWithdrawAddrIfMigrated
// would cause for delegators whose own migration hasn't happened yet.
func (k Keeper) temporaryRedirectWithdrawAddr(ctx sdk.Context, addr sdk.AccAddress) (origWD sdk.AccAddress, restored bool, err error) {
	withdrawAddr, err := k.distributionKeeper.GetDelegatorWithdrawAddr(ctx, addr)
	if err != nil {
		return nil, false, nil // No custom withdraw address — default (self) is fine.
	}

	if withdrawAddr.Equals(addr) {
		return nil, false, nil
	}

	has, err := k.MigrationRecords.Has(ctx, withdrawAddr.String())
	if err != nil || !has {
		return nil, false, nil // Not migrated — leave as-is.
	}

	// Temporarily redirect to self.
	if err := k.distributionKeeper.SetDelegatorWithdrawAddr(ctx, addr, addr); err != nil {
		return nil, false, err
	}
	return withdrawAddr, true, nil
}

func (k Keeper) ensureDelegatorStartingInfoReferenceCount(ctx sdk.Context, valAddr sdk.ValAddress, delAddr sdk.AccAddress) error {
	startingInfo, err := k.distributionKeeper.GetDelegatorStartingInfo(ctx, valAddr, delAddr)
	if err != nil {
		return nil
	}
	return k.adjustHistoricalRewardsReferenceCount(ctx, valAddr, startingInfo.PreviousPeriod, 1, true)
}

func (k Keeper) incrementHistoricalRewardsReferenceCount(ctx sdk.Context, valAddr sdk.ValAddress, period uint64) error {
	return k.adjustHistoricalRewardsReferenceCount(ctx, valAddr, period, 1, false)
}

// resetHistoricalRewardsReferenceCount sets the reference count to 1 (base only),
// clearing stale delegator references before re-creating delegations.
func (k Keeper) resetHistoricalRewardsReferenceCount(ctx sdk.Context, valAddr sdk.ValAddress, period uint64) error {
	var (
		found      bool
		historical distrtypes.ValidatorHistoricalRewards
	)

	k.distributionKeeper.IterateValidatorHistoricalRewards(ctx, func(val sdk.ValAddress, p uint64, rewards distrtypes.ValidatorHistoricalRewards) (stop bool) {
		if val.Equals(valAddr) && p == period {
			found = true
			historical = rewards
			return true
		}
		return false
	})

	if !found {
		return fmt.Errorf("validator historical rewards not found for %s period %d", valAddr.String(), period)
	}

	historical.ReferenceCount = 1
	return k.distributionKeeper.SetValidatorHistoricalRewards(ctx, valAddr, period, historical)
}

func (k Keeper) adjustHistoricalRewardsReferenceCount(ctx sdk.Context, valAddr sdk.ValAddress, period uint64, delta int64, repairZero bool) error {
	var (
		found      bool
		historical distrtypes.ValidatorHistoricalRewards
	)

	k.distributionKeeper.IterateValidatorHistoricalRewards(ctx, func(val sdk.ValAddress, p uint64, rewards distrtypes.ValidatorHistoricalRewards) (stop bool) {
		if val.Equals(valAddr) && p == period {
			found = true
			historical = rewards
			return true
		}
		return false
	})

	if !found {
		return fmt.Errorf("validator historical rewards not found for %s period %d", valAddr.String(), period)
	}

	if repairZero && historical.ReferenceCount > 0 {
		return nil
	}

	next := int64(historical.ReferenceCount) + delta
	if repairZero && historical.ReferenceCount == 0 && delta > 0 {
		next = 1
	}
	if next < 0 {
		return fmt.Errorf("negative historical rewards reference count for %s period %d", valAddr.String(), period)
	}

	historical.ReferenceCount = uint32(next)
	return k.distributionKeeper.SetValidatorHistoricalRewards(ctx, valAddr, period, historical)
}
