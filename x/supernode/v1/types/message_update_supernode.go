package types

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var _ sdk.Msg = &MsgUpdateSupernode{}

func NewMsgUpdateSupernode(creator string, validatorAddress string, ipAddress string, note string) *MsgUpdateSupernode {
	return &MsgUpdateSupernode{
		Creator:          creator,
		ValidatorAddress: validatorAddress,
		IpAddress:        ipAddress,
		Note:             note,
	}
}

func (msg *MsgUpdateSupernode) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}
	return nil
}
