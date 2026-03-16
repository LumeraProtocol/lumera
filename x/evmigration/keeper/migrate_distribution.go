package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
)

// MigrateDistribution withdraws all pending delegation rewards for legacyAddr,
// materializing them into the legacy bank balance before balances are moved.
func (k Keeper) MigrateDistribution(ctx sdk.Context, legacyAddr sdk.AccAddress) error {
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
		// WithdrawDelegationRewards sends rewards to the legacy bank balance.
		// Ignoring the returned coins — they're now in the bank balance.
		if _, err := k.distributionKeeper.WithdrawDelegationRewards(ctx, legacyAddr, valAddr); err != nil {
			return err
		}
	}

	return nil
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
