package types

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var (
	_ sdk.Msg = &MsgUpdateParams{}
	_ sdk.Msg = &MsgClaimLegacyAccount{}
	_ sdk.Msg = &MsgMigrateValidator{}
)

func (msg *MsgUpdateParams) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid authority address (%s)", err)
	}
	return msg.Params.Validate()
}

func (msg *MsgClaimLegacyAccount) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.NewAddress); err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid new_address (%s)", err)
	}
	if _, err := sdk.AccAddressFromBech32(msg.LegacyAddress); err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid legacy_address (%s)", err)
	}
	if msg.NewAddress == msg.LegacyAddress {
		return ErrSameAddress
	}
	if len(msg.LegacyPubKey) != 33 {
		return ErrInvalidLegacyPubKey.Wrap("compressed secp256k1 public key must be 33 bytes")
	}
	if len(msg.LegacySignature) == 0 {
		return ErrInvalidLegacySignature.Wrap("legacy_signature is required")
	}
	return nil
}

func (msg *MsgMigrateValidator) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.NewAddress); err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid new_address (%s)", err)
	}
	if _, err := sdk.AccAddressFromBech32(msg.LegacyAddress); err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid legacy_address (%s)", err)
	}
	if msg.NewAddress == msg.LegacyAddress {
		return ErrSameAddress
	}
	if len(msg.LegacyPubKey) != 33 {
		return ErrInvalidLegacyPubKey.Wrap("compressed secp256k1 public key must be 33 bytes")
	}
	if len(msg.LegacySignature) == 0 {
		return ErrInvalidLegacySignature.Wrap("legacy_signature is required")
	}
	return nil
}
