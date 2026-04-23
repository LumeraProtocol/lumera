package keeper

import (
	"fmt"

	kmultisig "github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ethsecp256k1 "github.com/cosmos/evm/crypto/ethsecp256k1"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
	"github.com/LumeraProtocol/lumera/x/evmigration/types/sigverify"
)

const (
	migrationPayloadKindClaim     = "claim"
	migrationPayloadKindValidator = "validator"
)

func migrationPayload(chainID string, evmChainID uint64, kind string, legacyAddr, newAddr sdk.AccAddress) []byte {
	return []byte(fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s", chainID, evmChainID, kind, legacyAddr.String(), newAddr.String()))
}

// verifySingleKeyProofSide validates a SingleKeyProof given an explicit sub-key
// type. Side is implied by keyType (SubKeyTypeCosmosSecp256k1 → legacy side,
// SubKeyTypeEthSecp256k1 → new side). The caller is expected to pass the
// correct boundAddr for the side (legacyAddr for Cosmos, newAddr for eth).
func verifySingleKeyProofSide(payload []byte, boundAddr sdk.AccAddress, p *types.SingleKeyProof, keyType sigverify.SubKeyType) error {
	if len(p.PubKey) != secp256k1.PubKeySize {
		return types.ErrInvalidMigrationPubKey.Wrapf("expected %d bytes, got %d", secp256k1.PubKeySize, len(p.PubKey))
	}
	switch keyType {
	case sigverify.SubKeyTypeCosmosSecp256k1:
		pk := &secp256k1.PubKey{Key: p.PubKey}
		derived := sdk.AccAddress(pk.Address())
		if !derived.Equals(boundAddr) {
			return types.ErrPubKeyAddressMismatch.Wrapf("pubkey derives to %s, expected %s", derived, boundAddr)
		}
		return sigverify.VerifyCosmosSecp256k1(pk, boundAddr, payload, p.Signature, p.SigFormat)
	case sigverify.SubKeyTypeEthSecp256k1:
		pk := &ethsecp256k1.PubKey{Key: p.PubKey}
		derived := sdk.AccAddress(pk.Address())
		if !derived.Equals(boundAddr) {
			return types.ErrPubKeyAddressMismatch.Wrapf("pubkey derives to %s, expected %s", derived, boundAddr)
		}
		return sigverify.VerifyEthSecp256k1(pk, boundAddr, payload, p.Signature, p.SigFormat)
	default:
		return types.ErrInvalidMigrationProof.Wrap("unknown sub-key type")
	}
}

// verifyMultisigProofSide reconstructs the LegacyAminoPubKey over the given
// SubKeyType, asserts it derives to boundAddr, then verifies each sub-signature
// using the matching per-sub-key helper.
func verifyMultisigProofSide(payload []byte, boundAddr sdk.AccAddress, m *types.MultisigProof, keyType sigverify.SubKeyType) error {
	subPubKeys := make([]cryptotypes.PubKey, len(m.SubPubKeys))
	for i, raw := range m.SubPubKeys {
		if len(raw) != secp256k1.PubKeySize {
			return types.ErrInvalidMigrationPubKey.Wrapf("sub_pub_keys[%d]: expected %d bytes, got %d", i, secp256k1.PubKeySize, len(raw))
		}
		switch keyType {
		case sigverify.SubKeyTypeCosmosSecp256k1:
			subPubKeys[i] = &secp256k1.PubKey{Key: raw}
		case sigverify.SubKeyTypeEthSecp256k1:
			subPubKeys[i] = &ethsecp256k1.PubKey{Key: raw}
		default:
			return types.ErrInvalidMigrationProof.Wrap("unknown sub-key type")
		}
	}
	multiPK := kmultisig.NewLegacyAminoPubKey(int(m.Threshold), subPubKeys)
	derived := sdk.AccAddress(multiPK.Address())
	if !derived.Equals(boundAddr) {
		return types.ErrPubKeyAddressMismatch.Wrapf("multisig pubkey derives to %s, expected %s", derived, boundAddr)
	}
	for i, idx := range m.SignerIndices {
		if int(idx) >= len(subPubKeys) {
			return types.ErrInvalidMigrationProof.Wrapf("signer_indices[%d]=%d out of range", i, idx)
		}
		switch pk := subPubKeys[idx].(type) {
		case *secp256k1.PubKey:
			signerAddr := sdk.AccAddress(pk.Address())
			if err := sigverify.VerifyCosmosSecp256k1(pk, signerAddr, payload, m.SubSignatures[i], m.SigFormat); err != nil {
				return types.ErrInvalidMigrationSignature.Wrapf("sub-sig %d (signer %s) invalid: %s", i, signerAddr, err)
			}
		case *ethsecp256k1.PubKey:
			signerAddr := sdk.AccAddress(pk.Address())
			if err := sigverify.VerifyEthSecp256k1(pk, signerAddr, payload, m.SubSignatures[i], m.SigFormat); err != nil {
				return types.ErrInvalidMigrationSignature.Wrapf("sub-sig %d (signer %s) invalid: %s", i, signerAddr, err)
			}
		default:
			// Provably unreachable today — the construction loop above only produces
			// *secp256k1.PubKey or *ethsecp256k1.PubKey entries. This default arm is
			// a permanent safety hedge: if a future task adds a third sub-key type
			// to the construction switch and forgets to update this verification
			// switch, we return an error instead of silently skipping sig check.
			return types.ErrInvalidMigrationPubKey.Wrapf("unexpected sub-key type %T at index %d (should be unreachable)", pk, idx)
		}
	}
	return nil
}

// VerifyMigrationProof verifies a migration proof against the canonical payload.
// Parameterized by sigverify.SubKeyType: legacy side passes
// sigverify.SubKeyTypeCosmosSecp256k1 and boundAddr=legacyAddr; new side passes
// sigverify.SubKeyTypeEthSecp256k1 and boundAddr=newAddr.
//
// Param-dependent limits (MaxMultisigSubKeys) must be enforced by the caller
// via proof.ValidateParams(params.MaxMultisigSubKeys) before invoking this
// function, since VerifyMigrationProof does not have access to keeper state.
func VerifyMigrationProof(
	chainID string, evmChainID uint64, kind string,
	legacyAddr, newAddr, boundAddr sdk.AccAddress,
	proof *types.MigrationProof,
	keyType sigverify.SubKeyType,
) error {
	if proof == nil {
		return types.ErrInvalidMigrationProof.Wrap("proof required")
	}
	side := types.SideLegacy
	if keyType == sigverify.SubKeyTypeEthSecp256k1 {
		side = types.SideNew
	}
	if err := proof.ValidateBasic(side); err != nil {
		return err
	}
	payload := migrationPayload(chainID, evmChainID, kind, legacyAddr, newAddr)
	switch p := proof.Proof.(type) {
	case *types.MigrationProof_Single:
		return verifySingleKeyProofSide(payload, boundAddr, p.Single, keyType)
	case *types.MigrationProof_Multisig:
		return verifyMultisigProofSide(payload, boundAddr, p.Multisig, keyType)
	default:
		return types.ErrInvalidMigrationProof.Wrap("no proof set")
	}
}
