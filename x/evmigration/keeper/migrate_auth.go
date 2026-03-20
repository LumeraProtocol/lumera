package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// MigrateAuth migrates the x/auth account record from legacyAddr to newAddr.
// For vesting accounts, it preserves the vesting schedule by:
//  1. Reading and saving vesting parameters
//  2. Removing the legacy account (removes vesting lock so bank transfer can succeed)
//  3. After bank transfer (caller responsibility), creating a matching vesting account at newAddr
//
// Returns vestingInfo if the legacy account was a vesting account (caller must call
// FinalizeVestingAccount after bank transfer), or nil for base accounts.
func (k Keeper) MigrateAuth(ctx context.Context, legacyAddr, newAddr sdk.AccAddress) (*VestingInfo, error) {
	legacyAcc := k.accountKeeper.GetAccount(ctx, legacyAddr)
	if legacyAcc == nil {
		return nil, types.ErrLegacyAccountNotFound
	}

	// Check for module accounts — these cannot be migrated.
	if _, ok := legacyAcc.(sdk.ModuleAccountI); ok {
		return nil, types.ErrCannotMigrateModuleAccount
	}

	var vi *VestingInfo

	switch acc := legacyAcc.(type) {
	case *vestingtypes.ContinuousVestingAccount:
		vi = &VestingInfo{
			Type:             VestingTypeContinuous,
			OriginalVesting:  acc.OriginalVesting,
			DelegatedFree:    acc.DelegatedFree,
			DelegatedVesting: acc.DelegatedVesting,
			EndTime:          acc.EndTime,
			StartTime:        acc.StartTime,
		}
	case *vestingtypes.DelayedVestingAccount:
		vi = &VestingInfo{
			Type:             VestingTypeDelayed,
			OriginalVesting:  acc.OriginalVesting,
			DelegatedFree:    acc.DelegatedFree,
			DelegatedVesting: acc.DelegatedVesting,
			EndTime:          acc.EndTime,
		}
	case *vestingtypes.PeriodicVestingAccount:
		vi = &VestingInfo{
			Type:             VestingTypePeriodic,
			OriginalVesting:  acc.OriginalVesting,
			DelegatedFree:    acc.DelegatedFree,
			DelegatedVesting: acc.DelegatedVesting,
			EndTime:          acc.EndTime,
			StartTime:        acc.StartTime,
			Periods:          acc.VestingPeriods,
		}
	case *vestingtypes.PermanentLockedAccount:
		vi = &VestingInfo{
			Type:             VestingTypePermanentLocked,
			OriginalVesting:  acc.OriginalVesting,
			DelegatedFree:    acc.DelegatedFree,
			DelegatedVesting: acc.DelegatedVesting,
			EndTime:          acc.EndTime,
		}
	}

	// Remove legacy account. For vesting accounts, this removes the vesting lock
	// so that the subsequent bank SendCoins can transfer all coins including locked ones.
	k.accountKeeper.RemoveAccount(ctx, legacyAcc)

	// Ensure the new address has an account record.
	newAcc := k.accountKeeper.GetAccount(ctx, newAddr)
	if newAcc == nil {
		newAcc = k.accountKeeper.NewAccountWithAddress(ctx, newAddr)
		k.accountKeeper.SetAccount(ctx, newAcc)
	}

	return vi, nil
}

// FinalizeVestingAccount creates a matching vesting account at newAddr after
// bank balances have been transferred. Must be called only if MigrateAuth returned
// non-nil VestingInfo.
func (k Keeper) FinalizeVestingAccount(ctx context.Context, newAddr sdk.AccAddress, vi *VestingInfo) error {
	newAcc := k.accountKeeper.GetAccount(ctx, newAddr)
	if newAcc == nil {
		return types.ErrLegacyAccountNotFound.Wrap("new account not found after bank transfer")
	}

	baseAcc, ok := newAcc.(*authtypes.BaseAccount)
	if !ok {
		// Account might already be a special type (e.g., from a previous receive).
		// Extract the base account.
		baseAcc = authtypes.NewBaseAccount(newAddr, newAcc.GetPubKey(), newAcc.GetAccountNumber(), newAcc.GetSequence())
	}

	var vestingAcc sdk.AccountI

	switch vi.Type {
	case VestingTypeContinuous:
		bva, err := vestingtypes.NewBaseVestingAccount(baseAcc, vi.OriginalVesting, vi.EndTime)
		if err != nil {
			return err
		}
		vestingAcc = vestingtypes.NewContinuousVestingAccountRaw(bva, vi.StartTime)
	case VestingTypeDelayed:
		bva, err := vestingtypes.NewBaseVestingAccount(baseAcc, vi.OriginalVesting, vi.EndTime)
		if err != nil {
			return err
		}
		vestingAcc = vestingtypes.NewDelayedVestingAccountRaw(bva)
	case VestingTypePeriodic:
		bva, err := vestingtypes.NewBaseVestingAccount(baseAcc, vi.OriginalVesting, vi.EndTime)
		if err != nil {
			return err
		}
		vestingAcc = vestingtypes.NewPeriodicVestingAccountRaw(bva, vi.StartTime, vi.Periods)
	case VestingTypePermanentLocked:
		pla, err := vestingtypes.NewPermanentLockedAccount(baseAcc, vi.OriginalVesting)
		if err != nil {
			return err
		}
		vestingAcc = pla
	}

	// Preserve delegated vesting/free tracking so spendable vesting semantics
	// remain unchanged after migration (important when the legacy vesting account
	// had active delegations).
	switch acc := vestingAcc.(type) {
	case *vestingtypes.ContinuousVestingAccount:
		acc.DelegatedFree = vi.DelegatedFree
		acc.DelegatedVesting = vi.DelegatedVesting
	case *vestingtypes.DelayedVestingAccount:
		acc.DelegatedFree = vi.DelegatedFree
		acc.DelegatedVesting = vi.DelegatedVesting
	case *vestingtypes.PeriodicVestingAccount:
		acc.DelegatedFree = vi.DelegatedFree
		acc.DelegatedVesting = vi.DelegatedVesting
	case *vestingtypes.PermanentLockedAccount:
		acc.DelegatedFree = vi.DelegatedFree
		acc.DelegatedVesting = vi.DelegatedVesting
	}

	if vestingAcc != nil {
		k.accountKeeper.SetAccount(ctx, vestingAcc)
	}

	return nil
}

// VestingType identifies the type of vesting account.
type VestingType int

const (
	VestingTypeContinuous VestingType = iota + 1
	VestingTypeDelayed
	VestingTypePeriodic
	VestingTypePermanentLocked
)

// VestingInfo holds the vesting parameters extracted from a legacy vesting account
// so they can be re-applied to the new address after bank transfer.
type VestingInfo struct {
	Type             VestingType
	OriginalVesting  sdk.Coins
	DelegatedFree    sdk.Coins
	DelegatedVesting sdk.Coins
	EndTime          int64
	StartTime        int64
	Periods          vestingtypes.Periods
}
