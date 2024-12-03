package types

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var _ sdk.Msg = &MsgStartSupernode{}

func NewMsgStartSupernode(creator string, validatorAddress string, ipAddress string) *MsgStartSupernode {
	return &MsgStartSupernode{
		Creator:          creator,
		ValidatorAddress: validatorAddress,
		IpAddress:        ipAddress,
	}
}

func (msg *MsgStartSupernode) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}
	return nil
}
