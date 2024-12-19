package keeper

import (
	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
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

func (k msgServer) checkValidatorSupernodeEligibility(ctx sdk.Context, validator types.ValidatorI, valAddr string) error {
	isBonded := validator.IsBonded()
	if !isBonded {
		minStake := k.GetParams(ctx).MinimumStakeForSn
		stake := validator.GetTokens()
		minStakeInt := math.NewIntFromUint64(minStake)
		if stake.LT(minStakeInt) {
			return errorsmod.Wrapf(
				sdkerrors.ErrInvalidRequest,
				"validator %s is not in active set and does not meet minimum stake requirement. Required: %d, Got: %s",
				valAddr,
				minStake,
				stake,
			)
		}
	}
	return nil
}
