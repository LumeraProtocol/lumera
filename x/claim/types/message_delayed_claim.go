package types

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var _ sdk.Msg = &MsgDelayedClaim{}

func NewMsgDelayedClaim(creator string, oldAddress string, newAddress string, pubKey string, signature string, tier int32) *MsgDelayedClaim {
	return &MsgDelayedClaim{
		Creator:    creator,
		OldAddress: oldAddress,
		NewAddress: newAddress,
		PubKey:     pubKey,
		Signature:  signature,
		Tier:       tier,
	}
}

func (msg *MsgDelayedClaim) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}

	if msg.Tier < 1 || msg.Tier > 5 {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "invalid tier (%d)", msg.Tier)
	}
	return nil
}
