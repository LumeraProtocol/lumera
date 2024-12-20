package keeper

import (
	"context"

	"cosmossdk.io/math"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// SupernodeHooks wrapper struct for staking hooks
type SupernodeHooks struct {
	k Keeper
}

var _ stakingtypes.StakingHooks = SupernodeHooks{}

// Create new supernode hooks instance
func NewSupernodeHooks(k Keeper) SupernodeHooks {
	return SupernodeHooks{k: k}
}

// Required hook implementation with no-op behavior for now
func (h SupernodeHooks) AfterValidatorCreated(ctx context.Context, valAddr sdk.ValAddress) error {
	return nil
}

func (h SupernodeHooks) BeforeValidatorModified(ctx context.Context, valAddr sdk.ValAddress) error {
	return nil
}

func (h SupernodeHooks) AfterValidatorRemoved(ctx context.Context, consAddr sdk.ConsAddress, valAddr sdk.ValAddress) error {
	return nil
}

func (h SupernodeHooks) AfterValidatorBonded(ctx context.Context, consAddr sdk.ConsAddress, valAddr sdk.ValAddress) error {
	return nil
}

func (h SupernodeHooks) AfterValidatorBeginUnbonding(ctx context.Context, consAddr sdk.ConsAddress, valAddr sdk.ValAddress) error {
	return nil
}

func (h SupernodeHooks) BeforeDelegationCreated(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) error {
	return nil
}

func (h SupernodeHooks) BeforeDelegationSharesModified(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) error {
	return nil
}

func (h SupernodeHooks) BeforeDelegationRemoved(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) error {
	return nil
}

func (h SupernodeHooks) AfterDelegationModified(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) error {
	return nil
}

func (h SupernodeHooks) BeforeValidatorSlashed(ctx context.Context, valAddr sdk.ValAddress, fraction math.LegacyDec) error {
	return nil
}

func (h SupernodeHooks) AfterUnbondingInitiated(ctx context.Context, id uint64) error {
	return nil
}

func (h SupernodeHooks) AfterConsensusPubKeyUpdate(ctx context.Context, oldPubKey, newPubKey cryptotypes.PubKey, fee sdk.Coin) error {
	return nil
}
