package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pastelnetwork/pastel/x/supernode/types"
)

var _ types.StakingHooks = Hooks{}

// Hooks wrapper struct for supernode keeper
type Hooks struct {
	k Keeper
}

// Return the supernode hooks
func (k Keeper) Hooks() Hooks {
	return Hooks{k}
}

// -----------------------------------------------------------------------------
// Required Hooks
// -----------------------------------------------------------------------------

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
		// If not already active, enable
		if !h.k.IsSuperNodeActive(sdkCtx, valAddr) {
			if err := h.k.EnableSuperNode(sdkCtx, valAddr); err != nil {
				return errorsmod.Wrap(err, "failed to enable supernode after validator bonded")
			}
		}
	} else {
		// If it doesn't meet requirements but was active, disable, just incase
		if h.k.IsSuperNodeActive(sdkCtx, valAddr) {
			if err := h.k.DisableSuperNode(sdkCtx, valAddr); err != nil {
				return errorsmod.Wrap(err, "failed to disable supernode after validator bonded")
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

	if h.k.IsSuperNodeActive(sdkCtx, valAddr) {
		if err := h.k.DisableSuperNode(sdkCtx, valAddr); err != nil {
			return errorsmod.Wrap(err, "failed to disable supernode after validator begin unbonding")
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
			if err := h.k.EnableSuperNode(sdkCtx, valAddr); err != nil {
				return errorsmod.Wrap(err, "failed to enable supernode after delegation modified")
			}
		}
	} else {
		// If it no longer meets requirements, disable if currently active
		if h.k.IsSuperNodeActive(sdkCtx, valAddr) {
			if err := h.k.DisableSuperNode(sdkCtx, valAddr); err != nil {
				return errorsmod.Wrap(err, "failed to disable supernode after delegation modified")
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

	// Additional Check, though supernode is already disabled before this hooks is called
	if h.k.IsSuperNodeActive(sdkCtx, valAddr) {
		if err := h.k.DisableSuperNode(sdkCtx, valAddr); err != nil {
			return errorsmod.Wrap(err, "failed to disable supernode after validator removed")
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// Hooks we do NOT use: no-ops
// -----------------------------------------------------------------------------

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
