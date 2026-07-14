package keeper

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
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

// repairLegacyRawShareStartingInfo repairs DelegatorStartingInfo rows written by
// the v1.20.0 evmigration implementation. That implementation stored delegation
// shares in Stake when re-keying an account; x/distribution requires the token
// value of those shares. The two values differ after a validator has been
// slashed, and reward withdrawal then panics in CalculateDelegationRewards.
//
// The repair is deliberately narrow: the validator must have a sub-1 exchange
// rate, the stored stake must exactly equal the delegation's raw shares (the
// v1.20.0 fingerprint), and replaying slash events after the starting height
// must still leave stake above the current token value. The corrected starting
// stake is scaled so that replaying those later slashes lands at currentStake;
// this preserves the SDK's reward-period accounting instead of simply clamping
// every period to the current value.
func (k Keeper) repairLegacyRawShareStartingInfo(
	ctx sdk.Context,
	val stakingtypes.Validator,
	del stakingtypes.Delegation,
	delAddr sdk.AccAddress,
) error {
	// Empty validator share state appears in keeper-level mocks and cannot be a
	// real active delegation. It also avoids a zero-denominator conversion.
	if val.DelegatorShares.IsNil() || !val.DelegatorShares.IsPositive() {
		return nil
	}

	currentStake := val.TokensFromShares(del.Shares)
	if !currentStake.LT(del.Shares) {
		return nil // never slashed (or otherwise not the v1.20.0 failure mode)
	}

	valAddr, err := sdk.ValAddressFromBech32(del.ValidatorAddress)
	if err != nil {
		return err
	}
	startingInfo, err := k.distributionKeeper.GetDelegatorStartingInfo(ctx, valAddr, delAddr)
	if err != nil {
		return fmt.Errorf("get delegator starting info: %w", err)
	}
	if !startingInfo.Stake.Equal(del.Shares) {
		return nil // not a row produced by the v1.20.0 raw-shares bug
	}

	finalStake := startingInfo.Stake
	slashEvents, err := k.validatorSlashEvents(ctx, valAddr)
	if err != nil {
		return fmt.Errorf("load validator slash events: %w", err)
	}
	for _, entry := range slashEvents {
		if entry.height < startingInfo.Height || entry.height > uint64(ctx.BlockHeight()) {
			continue
		}
		if entry.event.ValidatorPeriod <= startingInfo.PreviousPeriod {
			continue
		}
		finalStake = finalStake.MulTruncate(math.LegacyOneDec().Sub(entry.event.Fraction))
	}

	// Match the SDK's three-smallest-decimal rounding tolerance. Rows within
	// that margin do not panic and must not be rewritten.
	marginOfErr := math.LegacySmallestDec().MulInt64(3)
	if finalStake.LTE(currentStake.Add(marginOfErr)) {
		return nil
	}

	// Scale the starting value by the observed final/current discrepancy. When
	// post-start slash events exist, replaying them over this repaired value
	// retains their timing for reward calculation. Truncation is intentional:
	// the SDK requires calculated stake to be <= current stake.
	repairedStake := startingInfo.Stake.MulTruncate(currentStake.QuoTruncate(finalStake))
	startingInfo.Stake = repairedStake
	if err := k.distributionKeeper.SetDelegatorStartingInfo(ctx, valAddr, delAddr, startingInfo); err != nil {
		return fmt.Errorf("set repaired delegator starting info: %w", err)
	}

	ctx.Logger().Info(
		"repaired v1.20.0 raw-share delegator starting info",
		"validator", valAddr.String(),
		"delegator", delAddr.String(),
		"repaired_stake", repairedStake.String(),
		"current_stake", currentStake.String(),
	)
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

// setHistoricalRewardsReferenceCount overwrites the reference count of a single
// (valAddr, period) historical rewards row. Used to set the count in one write
// instead of a reset-then-increment-per-delegation loop, clearing stale
// delegator references in the process.
func (k Keeper) setHistoricalRewardsReferenceCount(ctx sdk.Context, valAddr sdk.ValAddress, period uint64, count uint32) error {
	historical, found, err := k.validatorHistoricalReward(ctx, valAddr, period)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("validator historical rewards not found for %s period %d", valAddr.String(), period)
	}

	historical.ReferenceCount = count
	return k.distributionKeeper.SetValidatorHistoricalRewards(ctx, valAddr, period, historical)
}

func (k Keeper) adjustHistoricalRewardsReferenceCount(ctx sdk.Context, valAddr sdk.ValAddress, period uint64, delta int64, repairZero bool) error {
	historical, found, err := k.validatorHistoricalReward(ctx, valAddr, period)
	if err != nil {
		return err
	}
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
