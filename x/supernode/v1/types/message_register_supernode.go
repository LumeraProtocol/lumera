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
	// Validate creator address (cosmos1...)
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}

	// Validate validator operator address (cosmosvaloper1...)
	_, err = sdk.ValAddressFromBech32(msg.ValidatorAddress)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid validator operator address (%s)", err)
	}

	if msg.IpAddress == "" {
		return errorsmod.Wrap(ErrEmptyIPAddress, "ip address cannot be empty")
	}

	if msg.Version == "" {
		return errorsmod.Wrap(ErrEmptyVersion, "version cannot be empty")
	}
	return nil
}
