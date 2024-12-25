package keeper

import (
	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/staking/types"
)

func (k msgServer) verifyValidatorOperator(_ sdk.Context, valOperAddr sdk.ValAddress, creator string) error {
	creatorAddr, err := sdk.AccAddressFromBech32(creator)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address: %s", err)
	}

	valAccAddr := sdk.AccAddress(valOperAddr)
	if !creatorAddr.Equals(valAccAddr) {
		return errorsmod.Wrapf(sdkerrors.ErrUnauthorized,
			"creator account %s is not the validator operator account %s",
			creatorAddr, valAccAddr)
	}

	return nil
}

// CheckValidatorSupernodeEligibility ensures the validator has enough self-stake.
func (k msgServer) CheckValidatorSupernodeEligibility(ctx sdk.Context, validator types.ValidatorI, valAddr string) error {
	// If the validator is already bonded, skip the stake check
	if validator.IsBonded() {
		return nil
	}

	// 1. Get chain's configured minimum self-stake
	minStake := k.GetParams(ctx).MinimumStakeForSn
	minStakeInt := sdkmath.NewIntFromUint64(minStake)

	// 2. Convert operator address (valAddr) into types
	valOperatorAddr, err := sdk.ValAddressFromBech32(valAddr)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress,
			"invalid validator address: %s", valAddr)
	}
	valAccAddr := sdk.AccAddress(valOperatorAddr)

	// 3. Get self-delegation record
	selfDelegation, found := k.stakingKeeper.Delegation(ctx, valAccAddr, valOperatorAddr)
	if !found {
		return errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest,
			"validator %s has no self-delegation; cannot meet minimum self-stake requirement",
			valAddr,
		)
	}

	// 4. Guard: if validator's DelegatorShares == 0, we can't compute tokens from shares
	if validator.GetDelegatorShares().IsZero() {
		return errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest,
			"validator %s has zero delegator shares; no self-stake available",
			valAddr,
		)
	}

	// 5. Convert the self-delegation shares to actual tokens (decimal)
	selfDelegatedTokens := validator.TokensFromShares(selfDelegation.GetShares())

	// 6. Convert decimal -> integer
	selfDelegatedTokensInt := selfDelegatedTokens.TruncateInt()

	// 7. Compare two Ints: selfDelegatedTokensInt vs. minStakeInt
	if selfDelegatedTokensInt.LT(minStakeInt) {
		return errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest,
			"validator %s does not meet minimum self stake requirement. Required: %d, got: %s",
			valAddr,
			minStake,
			selfDelegatedTokensInt.String(),
		)
	}

	return nil
}
