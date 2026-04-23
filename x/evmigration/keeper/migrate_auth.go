package keeper

import (
	"bytes"
	"context"

	kmultisig "github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	ethsecp256k1 "github.com/cosmos/evm/crypto/ethsecp256k1"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// MigrateAuth migrates the x/auth account record from legacyAddr to newAddr.
// For vesting accounts, it preserves the vesting schedule by:
//  1. Reading and saving vesting parameters
//  2. Removing the legacy account (removes vesting lock so bank transfer can succeed)
//  3. After bank transfer (caller responsibility), creating a matching vesting account at newAddr
//
// destProof is the new-side MigrationProof from the message. When it contains a
// Multisig sub-proof, MigrateAuth reconstructs the multisig pubkey and persists it
// on the new account via SetPubKey. For Single-key and nil proofs the pubkey slot
// is left nil (consistent with pre-refactor behaviour).
//
// Returns vestingInfo if the legacy account was a vesting account (caller must call
// FinalizeVestingAccount after bank transfer), or nil for base accounts.
//
// PHASE 1 — ALL pre-mutation checks run first. Any rejection leaves the chain state
// untouched (no partial migration possible).
// PHASE 2 — state mutation (legacy removal, new-account materialization, SetPubKey).
func (k Keeper) MigrateAuth(
	ctx context.Context,
	legacyAddr, newAddr sdk.AccAddress,
	destProof *types.MigrationProof,
) (*VestingInfo, error) {
	// -------------------------------------------------------------------------
	// PHASE 1 — ALL PRE-MUTATION CHECKS. No state is written until they pass.
	// Any rejection here leaves the chain state untouched (no partial migration).
	// -------------------------------------------------------------------------

	// Phase-1 check A: stateless proof validation. Cheap; do it first so
	// malformed input doesn't trigger any state reads.
	if destProof != nil {
		if err := destProof.ValidateBasic(types.SideNew); err != nil {
			return nil, err
		}
	}

	// Phase-1 probe: fetch the pre-existing account at newAddr ONCE and cache
	// it. The single GetAccount(newAddr) call is reused during materialization
	// in Phase 2; since legacy removal doesn't affect newAddr, the probe value
	// stays accurate across the mutation boundary.
	existingNewAcc := k.accountKeeper.GetAccount(ctx, newAddr)

	// Phase-1 check B: destination-account type safety (for both single-key
	// AND multisig destinations). FinalizeVestingAccount in Phase 2 would
	// silently clobber pre-existing special-type state, so reject anything
	// other than fresh or plain *BaseAccount here.
	if existingNewAcc != nil {
		if _, ok := existingNewAcc.(sdk.ModuleAccountI); ok {
			return nil, types.ErrCannotMigrateModuleAccount.Wrapf(
				"destination %s is a module account; cannot migrate to a module address",
				newAddr,
			)
		}
		if _, ok := existingNewAcc.(*authtypes.BaseAccount); !ok {
			// Covers vesting accounts (Continuous/Delayed/Periodic/PermanentLocked),
			// any future smart-account / contract-account type, and any third-party
			// wrapper type the module hasn't been taught about.
			return nil, types.ErrPubKeyAddressMismatch.Wrapf(
				"destination %s has non-BaseAccount type %T; migration to existing special accounts (vesting, module, etc.) is not supported — choose a fresh destination",
				newAddr, existingNewAcc,
			)
		}
	}

	// Phase-1 check C: multisig-specific reconstruction, address binding, AND
	// pubkey-compatibility on any pre-existing BaseAccount. All of this runs
	// BEFORE any state mutation so a mismatch cannot leave the chain in a
	// partially-migrated state.
	var destMultiPK cryptotypes.PubKey
	if destProof != nil {
		if ms := destProof.GetMultisig(); ms != nil {
			subKeys := make([]cryptotypes.PubKey, len(ms.SubPubKeys))
			for i, raw := range ms.SubPubKeys {
				subKeys[i] = &ethsecp256k1.PubKey{Key: raw}
			}
			multiPK := kmultisig.NewLegacyAminoPubKey(int(ms.Threshold), subKeys)
			if !sdk.AccAddress(multiPK.Address()).Equals(newAddr) {
				return nil, types.ErrPubKeyAddressMismatch.Wrapf(
					"destination multisig pubkey derives to %s, expected %s",
					sdk.AccAddress(multiPK.Address()), newAddr,
				)
			}
			// Pubkey-compatibility on cached pre-existing account: if it
			// already has a pubkey, it must match. SDK 0.53.6's
			// BaseAccount.SetPubKey is an unconditional overwrite, so without
			// this check we'd silently replace a different legitimate pubkey
			// during Phase 2.
			if existingNewAcc != nil {
				if existingPK := existingNewAcc.GetPubKey(); existingPK != nil {
					if !bytes.Equal(existingPK.Bytes(), multiPK.Bytes()) {
						return nil, types.ErrPubKeyAddressMismatch.Wrapf(
							"destination account %s already has a different pubkey; refusing to overwrite with reconstructed multisig",
							newAddr,
						)
					}
					// existingPK == multiPK → idempotent re-run case; destMultiPK stays nil
					// so Phase 2 skips the redundant SetPubKey call.
				} else {
					destMultiPK = multiPK
				}
			} else {
				destMultiPK = multiPK
			}
		}
	}

	// -------------------------------------------------------------------------
	// PHASE 2 — STATE MUTATION. All pre-mutation checks have passed.
	// -------------------------------------------------------------------------

	// Fetch the legacy account.
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

	// Materialize newAcc. Reuse the cached Phase-1 probe — legacy removal did not
	// touch newAddr, so existingNewAcc is still an accurate view.
	var newAcc sdk.AccountI
	if existingNewAcc != nil {
		newAcc = existingNewAcc
	} else {
		newAcc = k.accountKeeper.NewAccountWithAddress(ctx, newAddr)
	}

	// Apply multisig pubkey. Phase-1 check C already proved that if newAcc
	// has a pubkey, it byte-equals destMultiPK (in which case destMultiPK is nil
	// — idempotent path); only call SetPubKey when the existing slot is nil
	// (fresh account OR funded-but-never-signed).
	if destMultiPK != nil {
		if err := newAcc.SetPubKey(destMultiPK); err != nil {
			return nil, err
		}
	}

	k.accountKeeper.SetAccount(ctx, newAcc)
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
