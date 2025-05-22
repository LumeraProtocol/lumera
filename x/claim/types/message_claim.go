package types

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var _ sdk.Msg = &MsgClaim{}

func NewMsgClaim(creator string, oldAddress string, newAddress string, pubKey string, signature string) *MsgClaim {
	return &MsgClaim{
		Creator:    creator,
		OldAddress: oldAddress,
		NewAddress: newAddress,
		PubKey:     pubKey,
		Signature:  signature,
	}
}

func (msg *MsgClaim) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}
	return nil
}
