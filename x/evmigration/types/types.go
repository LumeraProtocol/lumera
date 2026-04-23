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
	if err := msg.LegacyProof.ValidateBasic(SideLegacy); err != nil {
		return err
	}
	if err := msg.NewProof.ValidateBasic(SideNew); err != nil {
		return err
	}
	return nil
}

// MigrationNewAddress returns the destination address used by the custom CLI flow.
func (msg *MsgClaimLegacyAccount) MigrationNewAddress() string { return msg.NewAddress }

// MigrationLegacyAddress returns the legacy source address used by the custom CLI flow.
func (msg *MsgClaimLegacyAccount) MigrationLegacyAddress() string { return msg.LegacyAddress }

// MigrationSetNewProof attaches the destination-account proof derived by the custom CLI.
// The raw EVM signature is wrapped in a SingleKeyProof; VerifyNewSignature recovers
// the signer's public key directly from the signature bytes without needing PubKey.
// SigFormat_SIG_FORMAT_CLI matches what the one-shot CLI produces via
// Keyring.Sign with SIGN_MODE_LEGACY_AMINO_JSON — the eth keyring signs
// Keccak256(payload) directly (no EIP-191 envelope). This method is deleted
// in Task 13; until then, the format must match the actual signing envelope
// so Tasks 10/11's format-aware verifier accepts the adapter output.
func (msg *MsgClaimLegacyAccount) MigrationSetNewProof(signature []byte) {
	msg.NewProof = MigrationProof{Proof: &MigrationProof_Single{Single: &SingleKeyProof{
		Signature: signature,
		SigFormat: SigFormat_SIG_FORMAT_CLI,
	}}}
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
	if err := msg.LegacyProof.ValidateBasic(SideLegacy); err != nil {
		return err
	}
	if err := msg.NewProof.ValidateBasic(SideNew); err != nil {
		return err
	}
	return nil
}

// MigrationNewAddress returns the destination address used by the custom CLI flow.
func (msg *MsgMigrateValidator) MigrationNewAddress() string { return msg.NewAddress }

// MigrationLegacyAddress returns the legacy source address used by the custom CLI flow.
func (msg *MsgMigrateValidator) MigrationLegacyAddress() string { return msg.LegacyAddress }

// MigrationSetNewProof attaches the destination-account proof derived by the custom CLI.
// See the MsgClaimLegacyAccount counterpart above for the SigFormat rationale.
func (msg *MsgMigrateValidator) MigrationSetNewProof(signature []byte) {
	msg.NewProof = MigrationProof{Proof: &MigrationProof_Single{Single: &SingleKeyProof{
		Signature: signature,
		SigFormat: SigFormat_SIG_FORMAT_CLI,
	}}}
}
