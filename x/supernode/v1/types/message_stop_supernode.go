package types

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var _ sdk.Msg = &MsgStopSupernode{}

func NewMsgStopSupernode(creator string, validatorAddress string, reason string) *MsgStopSupernode {
	return &MsgStopSupernode{
		Creator:          creator,
		ValidatorAddress: validatorAddress,
		Reason:           reason,
	}
}

func (msg *MsgStopSupernode) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}
	return nil
}
