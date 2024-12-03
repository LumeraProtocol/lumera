package types

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var _ sdk.Msg = &MsgRegisterSupernode{}

func NewMsgRegisterSupernode(creator string, validatorAddress string, ipAddress string, version string) *MsgRegisterSupernode {
	return &MsgRegisterSupernode{
		Creator:          creator,
		ValidatorAddress: validatorAddress,
		IpAddress:        ipAddress,
		Version:          version,
	}
}

func (msg *MsgRegisterSupernode) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}
	return nil
}
