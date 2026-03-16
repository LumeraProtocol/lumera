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
	if len(msg.NewPubKey) != 33 {
		return ErrInvalidNewPubKey.Wrap("compressed eth_secp256k1 public key must be 33 bytes")
	}
	if len(msg.NewSignature) == 0 {
		return ErrInvalidNewSignature.Wrap("new_signature is required")
	}
	return nil
}

// MigrationNewAddress returns the destination address used by the custom CLI flow.
func (msg *MsgClaimLegacyAccount) MigrationNewAddress() string { return msg.NewAddress }

// MigrationLegacyAddress returns the legacy source address used by the custom CLI flow.
func (msg *MsgClaimLegacyAccount) MigrationLegacyAddress() string { return msg.LegacyAddress }

// MigrationSetNewProof attaches the destination-account proof derived by the custom CLI.
func (msg *MsgClaimLegacyAccount) MigrationSetNewProof(pubKey, signature []byte) {
	msg.NewPubKey = pubKey
	msg.NewSignature = signature
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
	if len(msg.NewPubKey) != 33 {
		return ErrInvalidNewPubKey.Wrap("compressed eth_secp256k1 public key must be 33 bytes")
	}
	if len(msg.NewSignature) == 0 {
		return ErrInvalidNewSignature.Wrap("new_signature is required")
	}
	return nil
}

// MigrationNewAddress returns the destination address used by the custom CLI flow.
func (msg *MsgMigrateValidator) MigrationNewAddress() string { return msg.NewAddress }

// MigrationLegacyAddress returns the legacy source address used by the custom CLI flow.
func (msg *MsgMigrateValidator) MigrationLegacyAddress() string { return msg.LegacyAddress }

// MigrationSetNewProof attaches the destination-account proof derived by the custom CLI.
func (msg *MsgMigrateValidator) MigrationSetNewProof(pubKey, signature []byte) {
	msg.NewPubKey = pubKey
	msg.NewSignature = signature
}
