package types

import (
	errorsmod "cosmossdk.io/errors"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
)

// Side identifies which half of a migration a proof is proving.
type Side int

const (
	SideLegacy Side = iota + 1
	SideNew
)

// ValidateBasic checks the proof structure. side distinguishes legacy-side (Cosmos
// secp256k1, 64-byte sigs) from new-side (eth_secp256k1, 65-byte R||S||V sigs).
//
// Governance-controlled limits (MaxMultisigSubKeys) are checked via
// ValidateParams, called from the msg server after loading params.
func (p *MigrationProof) ValidateBasic(side Side) error {
	if p == nil {
		return ErrInvalidMigrationProof.Wrap("migration_proof required")
	}
	switch inner := p.Proof.(type) {
	case *MigrationProof_Single:
		return inner.Single.validateBasic(side)
	case *MigrationProof_Multisig:
		return inner.Multisig.validateBasic(side)
	default:
		return ErrInvalidMigrationProof.Wrap("migration_proof oneof not set")
	}
}

// ValidateParams performs param-dependent validation. Must be called by the
// msg server after Params are loaded from state.
func (p *MigrationProof) ValidateParams(maxSubKeys uint32) error {
	if p == nil {
		return ErrInvalidMigrationProof.Wrap("migration_proof required")
	}
	if m, ok := p.Proof.(*MigrationProof_Multisig); ok {
		return MultisigProofValidateParams(m.Multisig, maxSubKeys)
	}
	return nil
}

func (s *SingleKeyProof) validateBasic(side Side) error {
	if s == nil {
		return ErrInvalidMigrationProof.Wrap("single proof nil")
	}
	if len(s.PubKey) != secp256k1.PubKeySize {
		return ErrInvalidMigrationPubKey.Wrapf("expected %d bytes, got %d", secp256k1.PubKeySize, len(s.PubKey))
	}
	// Per-side signature length — see design §4.1 and §4.2 for rationale.
	// Legacy Cosmos secp256k1: 64 bytes raw R||S (Cosmos keyring has no V convention).
	// New eth_secp256k1: 65 bytes R||S||V (Cosmos EVM v0.6.0 keyring, go-ethereum
	// crypto.Sign, and Keplr/Leap personal_sign all produce 65; V is ignored by
	// the verifier but kept on-wire for Ethereum-native tooling consistency).
	if side == SideLegacy && len(s.Signature) != 64 {
		return ErrInvalidMigrationSignature.Wrapf("legacy Cosmos secp256k1 signature must be 64 bytes, got %d", len(s.Signature))
	}
	if side == SideNew && len(s.Signature) != 65 {
		return ErrInvalidMigrationSignature.Wrapf("new eth_secp256k1 signature must be 65 bytes (R||S||V), got %d", len(s.Signature))
	}
	if s.SigFormat == SigFormat_SIG_FORMAT_UNSPECIFIED {
		return ErrInvalidMigrationProof.Wrap("sig_format unspecified")
	}
	if s.SigFormat == SigFormat_SIG_FORMAT_EIP191 && side != SideNew {
		return ErrInvalidMigrationProof.Wrap("EIP191 is only valid for new-side single-key proofs")
	}
	return nil
}

// SingleKeyProofValidateBasic validates a SingleKeyProof's static invariants
// using legacy-side rules (Cosmos secp256k1, 64-byte signatures).
// Retained for backwards-compatible usage in tests and helpers.
func SingleKeyProofValidateBasic(s *SingleKeyProof) error {
	return s.validateBasic(SideLegacy)
}

func (m *MultisigProof) validateBasic(side Side) error {
	if m == nil {
		return ErrInvalidMigrationProof.Wrap("multisig proof nil")
	}
	if m.SigFormat == SigFormat_SIG_FORMAT_EIP191 {
		return ErrInvalidMigrationProof.Wrap("EIP191 is not valid for multisig proofs on either side")
	}
	if err := m.validateStructure(); err != nil {
		return err
	}
	// Length-check EVERY sub_pub_key (not just indexed ones) — LegacyAminoPubKey.Address()
	// consumes all N sub-keys during derivation.
	for i, raw := range m.SubPubKeys {
		if len(raw) != secp256k1.PubKeySize {
			return ErrInvalidMigrationPubKey.Wrapf("sub_pub_keys[%d]: expected %d bytes, got %d",
				i, secp256k1.PubKeySize, len(raw))
		}
	}
	// Per-side sub-signature length enforcement.
	expectedSigLen := 64
	sigLabel := "legacy Cosmos secp256k1 sub-signature"
	if side == SideNew {
		expectedSigLen = 65
		sigLabel = "new eth_secp256k1 sub-signature"
	}
	for i, sig := range m.SubSignatures {
		if len(sig) != expectedSigLen {
			return ErrInvalidMigrationSignature.Wrapf("%s[%d]: expected %d bytes, got %d",
				sigLabel, i, expectedSigLen, len(sig))
		}
	}
	return nil
}

// validateStructure checks the structural invariants of a MultisigProof that are
// independent of side (N>=1, threshold bounds, signer_indices exact-K + ascending
// + in-range, sub_signatures length matches, sig_format not unspecified).
func (m *MultisigProof) validateStructure() error {
	if m == nil {
		return ErrInvalidMigrationProof.Wrap("multisig proof nil")
	}
	n := uint32(len(m.SubPubKeys))
	if n == 0 {
		return ErrInvalidMigrationProof.Wrap("sub_pub_keys empty")
	}
	if m.Threshold < 1 || m.Threshold > n {
		return errorsmod.Wrapf(ErrInvalidMigrationProof, "invalid threshold K=%d for N=%d", m.Threshold, n)
	}
	if uint32(len(m.SignerIndices)) != m.Threshold {
		return errorsmod.Wrapf(ErrInvalidMigrationProof,
			"expected exactly K=%d signer_indices, got %d", m.Threshold, len(m.SignerIndices))
	}
	if len(m.SubSignatures) != len(m.SignerIndices) {
		return ErrInvalidMigrationProof.Wrap("sub_signatures length mismatch")
	}
	for i := 1; i < len(m.SignerIndices); i++ {
		if m.SignerIndices[i] <= m.SignerIndices[i-1] {
			return ErrInvalidMigrationProof.Wrap("signer_indices must be strictly ascending")
		}
	}
	for i, idx := range m.SignerIndices {
		if idx >= n {
			return errorsmod.Wrapf(ErrInvalidMigrationProof,
				"signer_indices[%d]=%d >= N=%d", i, idx, n)
		}
	}
	if m.SigFormat == SigFormat_SIG_FORMAT_UNSPECIFIED {
		return ErrInvalidMigrationProof.Wrap("sig_format required")
	}
	return nil
}

// MultisigProofValidateBasic validates a MultisigProof's static invariants
// using legacy-side rules (Cosmos secp256k1, 64-byte sub-signatures).
// Retained for backwards-compatible usage in tests and helpers.
// Size cap is enforced separately by MultisigProofValidateParams.
func MultisigProofValidateBasic(m *MultisigProof) error {
	return m.validateBasic(SideLegacy)
}

// MultisigProofValidateParams enforces the governance-adjustable size cap.
func MultisigProofValidateParams(m *MultisigProof, maxSubKeys uint32) error {
	if m == nil {
		return nil
	}
	if uint32(len(m.SubPubKeys)) > maxSubKeys {
		return errorsmod.Wrapf(ErrInvalidMigrationProof,
			"multisig N=%d exceeds max %d", len(m.SubPubKeys), maxSubKeys)
	}
	return nil
}
