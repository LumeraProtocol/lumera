package keeper

import (
	"context"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ types.StakingHooks = Hooks{}

type Hooks struct {
	k Keeper
}

func (k Keeper) Hooks() Hooks {
	return Hooks{k}
}

// Required Hooks

// AfterValidatorBonded: called AFTER a validator transitions from unbonded -> bonded (joins active set).
func (h Hooks) AfterValidatorBonded(ctx context.Context, consAddr sdk.ConsAddress, valAddr sdk.ValAddress) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	//early return if super node is not registered
	_, found := h.k.QuerySuperNode(sdkCtx, valAddr)
	if !found {
		return nil
	}

	// Check if validator meets supernode requirements
	if h.k.IsEligibleAndNotJailedValidator(sdkCtx, valAddr) {
		// If not already active, enable (no-op if currently disabled)
		if !h.k.IsSuperNodeActive(sdkCtx, valAddr) {
			if err := h.k.SetSuperNodeActive(sdkCtx, valAddr); err != nil {
				return errorsmod.Wrap(err, "failed to set supernode active after validator bonded")
			}
		}
	} else {
		// Not eligible: transition to STOPPED (hooks never set DISABLED)
		if h.k.IsSuperNodeActive(sdkCtx, valAddr) {
			if err := h.k.SetSuperNodeStopped(sdkCtx, valAddr); err != nil {
				return errorsmod.Wrap(err, "failed to stop supernode after validator bonded")
			}
		}
	}

	return nil
}

// AfterValidatorBeginUnbonding: called AFTER a validator transitions from bonded -> unbonding.
func (h Hooks) AfterValidatorBeginUnbonding(ctx context.Context, consAddr sdk.ConsAddress, valAddr sdk.ValAddress) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	//early return if super node is not registered
	_, found := h.k.QuerySuperNode(sdkCtx, valAddr)
	if !found {
		return nil
	}

	if h.k.IsSuperNodeActive(sdkCtx, valAddr) && !h.k.IsEligibleAndNotJailedValidator(sdkCtx, valAddr) {
		if err := h.k.SetSuperNodeStopped(sdkCtx, valAddr); err != nil {
			return errorsmod.Wrap(err, "failed to stop supernode after validator begin unbonding")
		}
	}

	return nil
}

// AfterDelegationModified: called AFTER delegations or redelegations update the validator's stake.
func (h Hooks) AfterDelegationModified(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	//early return if super node is not registered
	_, found := h.k.QuerySuperNode(sdkCtx, valAddr)
	if !found {
		return nil
	}

	// If it meets requirements (stake above min), enable if not active
	if h.k.IsEligibleAndNotJailedValidator(sdkCtx, valAddr) {
		if !h.k.IsSuperNodeActive(sdkCtx, valAddr) {
			if err := h.k.SetSuperNodeActive(sdkCtx, valAddr); err != nil {
				return errorsmod.Wrap(err, "failed to set supernode active after delegation modified")
			}
		}
	} else {
		// If it no longer meets requirements, stop if currently active
		if h.k.IsSuperNodeActive(sdkCtx, valAddr) {
			if err := h.k.SetSuperNodeStopped(sdkCtx, valAddr); err != nil {
				return errorsmod.Wrap(err, "failed to stop supernode after delegation modified")
			}
		}
	}

	return nil
}

// AfterValidatorRemoved: called AFTER a validator is completely removed.
func (h Hooks) AfterValidatorRemoved(ctx context.Context, consAddr sdk.ConsAddress, valAddr sdk.ValAddress) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	//early return if super node is not registered
	_, found := h.k.QuerySuperNode(sdkCtx, valAddr)
	if !found {
		return nil
	}

	// If still active, stop it; hooks must not set DISABLED
	if h.k.IsSuperNodeActive(sdkCtx, valAddr) {
		if err := h.k.SetSuperNodeStopped(sdkCtx, valAddr); err != nil {
			return errorsmod.Wrap(err, "failed to stop supernode after validator removed")
		}
	}
	return nil
}

// Hooks we do NOT use: no-ops

func (h Hooks) AfterValidatorCreated(ctx context.Context, valAddr sdk.ValAddress) error {
	return nil
}

func (h Hooks) BeforeValidatorModified(ctx context.Context, valAddr sdk.ValAddress) error {
	return nil
}

func (h Hooks) BeforeDelegationCreated(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) error {
	return nil
}

func (h Hooks) BeforeDelegationSharesModified(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) error {
	return nil
}

func (h Hooks) BeforeDelegationRemoved(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) error {
	return nil
}

func (h Hooks) BeforeValidatorSlashed(ctx context.Context, valAddr sdk.ValAddress, fraction math.LegacyDec) error {
	return nil
}

func (h Hooks) AfterUnbondingInitiated(ctx context.Context, id uint64) error {
	return nil
}

func (h Hooks) AfterConsensusPubKeyUpdate(ctx context.Context, oldPubKey, newPubKey cryptotypes.PubKey, fee sdk.Coin) error {
	return nil
}
