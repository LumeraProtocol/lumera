package types

import (
	errorsmod "cosmossdk.io/errors"
)

// ValidateBasic performs stateless validation of a LegacyProof.
// Governance-controlled limits (MaxMultisigSubKeys) are checked via
// ValidateParams, called from the msg server after loading params.
func (p *LegacyProof) ValidateBasic() error {
	if p == nil {
		return ErrInvalidLegacyProof.Wrap("legacy_proof required")
	}
	switch inner := p.Proof.(type) {
	case *LegacyProof_Single:
		return SingleKeyProofValidateBasic(inner.Single)
	case *LegacyProof_Multisig:
		return MultisigProofValidateBasic(inner.Multisig)
	default:
		return ErrInvalidLegacyProof.Wrap("legacy_proof oneof not set")
	}
}

// ValidateParams performs param-dependent validation. Must be called by the
// msg server after Params are loaded from state.
func (p *LegacyProof) ValidateParams(maxSubKeys uint32) error {
	if p == nil {
		return ErrInvalidLegacyProof.Wrap("legacy_proof required")
	}
	if m, ok := p.Proof.(*LegacyProof_Multisig); ok {
		return MultisigProofValidateParams(m.Multisig, maxSubKeys)
	}
	return nil
}

// SingleKeyProofValidateBasic validates a SingleKeyProof's static invariants.
func SingleKeyProofValidateBasic(s *SingleKeyProof) error {
	if s == nil {
		return ErrInvalidLegacyProof.Wrap("single proof nil")
	}
	if len(s.PubKey) != 33 {
		return ErrInvalidLegacyPubKey.Wrap("pub_key must be 33 bytes")
	}
	if len(s.Signature) == 0 {
		return ErrInvalidLegacySignature.Wrap("signature required")
	}
	if s.SigFormat == SigFormat_SIG_FORMAT_UNSPECIFIED {
		return ErrInvalidLegacyProof.Wrap("sig_format required")
	}
	return nil
}

// MultisigProofValidateBasic validates a MultisigProof's static invariants
// (length, ordering, indices). Size cap is enforced separately by
// MultisigProofValidateParams.
func MultisigProofValidateBasic(m *MultisigProof) error {
	if m == nil {
		return ErrInvalidLegacyProof.Wrap("multisig proof nil")
	}
	n := uint32(len(m.SubPubKeys))
	if n == 0 {
		return ErrInvalidLegacyProof.Wrap("sub_pub_keys empty")
	}
	if m.Threshold < 1 || m.Threshold > n {
		return errorsmod.Wrapf(ErrInvalidLegacyProof, "invalid threshold K=%d for N=%d", m.Threshold, n)
	}
	if uint32(len(m.SignerIndices)) != m.Threshold {
		return errorsmod.Wrapf(ErrInvalidLegacyProof,
			"expected exactly K=%d signer_indices, got %d", m.Threshold, len(m.SignerIndices))
	}
	if len(m.SubSignatures) != len(m.SignerIndices) {
		return ErrInvalidLegacyProof.Wrap("sub_signatures length mismatch")
	}
	for i := 1; i < len(m.SignerIndices); i++ {
		if m.SignerIndices[i] <= m.SignerIndices[i-1] {
			return ErrInvalidLegacyProof.Wrap("signer_indices must be strictly ascending")
		}
	}
	for i, idx := range m.SignerIndices {
		if idx >= n {
			return errorsmod.Wrapf(ErrInvalidLegacyProof,
				"signer_indices[%d]=%d >= N=%d", i, idx, n)
		}
	}
	for i, k := range m.SubPubKeys {
		if len(k) != 33 {
			return errorsmod.Wrapf(ErrInvalidLegacyPubKey,
				"sub_pub_keys[%d] must be 33 bytes", i)
		}
	}
	if m.SigFormat == SigFormat_SIG_FORMAT_UNSPECIFIED {
		return ErrInvalidLegacyProof.Wrap("sig_format required")
	}
	return nil
}

// MultisigProofValidateParams enforces the governance-adjustable size cap.
func MultisigProofValidateParams(m *MultisigProof, maxSubKeys uint32) error {
	if m == nil {
		return nil
	}
	if uint32(len(m.SubPubKeys)) > maxSubKeys {
		return errorsmod.Wrapf(ErrInvalidLegacyProof,
			"multisig N=%d exceeds max %d", len(m.SubPubKeys), maxSubKeys)
	}
	return nil
}
