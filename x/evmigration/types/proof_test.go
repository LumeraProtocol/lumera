package types_test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

func TestSingleKeyProof_ValidateBasic(t *testing.T) {
	validPK := make([]byte, 33)
	validSig := make([]byte, 64)

	cases := []struct {
		name    string
		proof   *types.SingleKeyProof
		wantErr string
	}{
		{
			name:    "valid",
			proof:   &types.SingleKeyProof{PubKey: validPK, Signature: validSig, SigFormat: types.SigFormat_SIG_FORMAT_CLI},
			wantErr: "",
		},
		{
			name:    "wrong pubkey length",
			proof:   &types.SingleKeyProof{PubKey: make([]byte, 32), Signature: validSig, SigFormat: types.SigFormat_SIG_FORMAT_CLI},
			wantErr: "expected 33 bytes",
		},
		{
			name:    "empty signature",
			proof:   &types.SingleKeyProof{PubKey: validPK, Signature: nil, SigFormat: types.SigFormat_SIG_FORMAT_CLI},
			wantErr: "64 bytes",
		},
		{
			name:    "unspecified sig format",
			proof:   &types.SingleKeyProof{PubKey: validPK, Signature: validSig, SigFormat: types.SigFormat_SIG_FORMAT_UNSPECIFIED},
			wantErr: "sig_format unspecified",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := types.SingleKeyProofValidateBasic(tc.proof)
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}

func TestMultisigProof_ValidateBasic(t *testing.T) {
	makeKeys := func(n int) [][]byte {
		keys := make([][]byte, n)
		for i := range keys {
			keys[i] = make([]byte, 33)
			keys[i][0] = byte(i + 1)
		}
		return keys
	}
	validSig := make([]byte, 64)

	cases := []struct {
		name    string
		proof   *types.MultisigProof
		wantErr string
	}{
		{
			name: "valid 2-of-3",
			proof: &types.MultisigProof{
				Threshold:     2,
				SubPubKeys:    makeKeys(3),
				SignerIndices: []uint32{0, 2},
				SubSignatures: [][]byte{validSig, validSig},
				SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
			},
			wantErr: "",
		},
		{
			name: "empty sub_pub_keys",
			proof: &types.MultisigProof{
				Threshold:     1,
				SubPubKeys:    nil,
				SignerIndices: []uint32{0},
				SubSignatures: [][]byte{validSig},
				SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
			},
			wantErr: "sub_pub_keys empty",
		},
		{
			name: "threshold zero",
			proof: &types.MultisigProof{
				Threshold:     0,
				SubPubKeys:    makeKeys(3),
				SignerIndices: []uint32{},
				SubSignatures: [][]byte{},
				SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
			},
			wantErr: "invalid threshold",
		},
		{
			name: "threshold exceeds N",
			proof: &types.MultisigProof{
				Threshold:     4,
				SubPubKeys:    makeKeys(3),
				SignerIndices: []uint32{0, 1, 2},
				SubSignatures: [][]byte{validSig, validSig, validSig},
				SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
			},
			wantErr: "invalid threshold",
		},
		{
			name: "too few signer_indices",
			proof: &types.MultisigProof{
				Threshold:     2,
				SubPubKeys:    makeKeys(3),
				SignerIndices: []uint32{0},
				SubSignatures: [][]byte{validSig},
				SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
			},
			wantErr: "expected exactly K=2 signer_indices",
		},
		{
			name: "too many signer_indices",
			proof: &types.MultisigProof{
				Threshold:     2,
				SubPubKeys:    makeKeys(3),
				SignerIndices: []uint32{0, 1, 2},
				SubSignatures: [][]byte{validSig, validSig, validSig},
				SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
			},
			wantErr: "expected exactly K=2 signer_indices",
		},
		{
			name: "sub_signatures length mismatch",
			proof: &types.MultisigProof{
				Threshold:     2,
				SubPubKeys:    makeKeys(3),
				SignerIndices: []uint32{0, 1},
				SubSignatures: [][]byte{validSig},
				SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
			},
			wantErr: "sub_signatures length mismatch",
		},
		{
			name: "indices not ascending",
			proof: &types.MultisigProof{
				Threshold:     2,
				SubPubKeys:    makeKeys(3),
				SignerIndices: []uint32{2, 0},
				SubSignatures: [][]byte{validSig, validSig},
				SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
			},
			wantErr: "strictly ascending",
		},
		{
			name: "indices duplicate",
			proof: &types.MultisigProof{
				Threshold:     2,
				SubPubKeys:    makeKeys(3),
				SignerIndices: []uint32{1, 1},
				SubSignatures: [][]byte{validSig, validSig},
				SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
			},
			wantErr: "strictly ascending",
		},
		{
			name: "index out of range",
			proof: &types.MultisigProof{
				Threshold:     2,
				SubPubKeys:    makeKeys(3),
				SignerIndices: []uint32{0, 5},
				SubSignatures: [][]byte{validSig, validSig},
				SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
			},
			wantErr: ">= N=3",
		},
		{
			name: "sub pubkey wrong length",
			proof: &types.MultisigProof{
				Threshold:     2,
				SubPubKeys:    [][]byte{make([]byte, 33), make([]byte, 32), make([]byte, 33)},
				SignerIndices: []uint32{0, 1},
				SubSignatures: [][]byte{validSig, validSig},
				SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
			},
			wantErr: "expected 33 bytes",
		},
		{
			name: "unspecified sig format",
			proof: &types.MultisigProof{
				Threshold:     2,
				SubPubKeys:    makeKeys(3),
				SignerIndices: []uint32{0, 1},
				SubSignatures: [][]byte{validSig, validSig},
				SigFormat:     types.SigFormat_SIG_FORMAT_UNSPECIFIED,
			},
			wantErr: "sig_format required",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := types.MultisigProofValidateBasic(tc.proof)
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}

func TestMultisigProof_ValidateParams_SizeCap(t *testing.T) {
	makeKeys := func(n int) [][]byte {
		keys := make([][]byte, n)
		for i := range keys {
			keys[i] = make([]byte, 33)
			keys[i][0] = byte(i + 1)
		}
		return keys
	}
	validSig := make([]byte, 64)

	proof := &types.MultisigProof{
		Threshold:     1,
		SubPubKeys:    makeKeys(21),
		SignerIndices: []uint32{0},
		SubSignatures: [][]byte{validSig},
		SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
	}
	err := types.MultisigProofValidateParams(proof, 20)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds max 20")

	proof.SubPubKeys = makeKeys(20)
	require.NoError(t, types.MultisigProofValidateParams(proof, 20))
}

func TestMigrationProof_ValidateBasic_Dispatch(t *testing.T) {
	validPK := make([]byte, 33)
	validSig64 := make([]byte, 64)
	validSig65 := make([]byte, 65)

	t.Run("single legacy side", func(t *testing.T) {
		p := &types.MigrationProof{
			Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
				PubKey: validPK, Signature: validSig64, SigFormat: types.SigFormat_SIG_FORMAT_CLI,
			}},
		}
		require.NoError(t, p.ValidateBasic(types.SideLegacy))
	})
	t.Run("single new side", func(t *testing.T) {
		p := &types.MigrationProof{
			Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
				PubKey: validPK, Signature: validSig65, SigFormat: types.SigFormat_SIG_FORMAT_EIP191,
			}},
		}
		require.NoError(t, p.ValidateBasic(types.SideNew))
	})
	t.Run("multisig legacy side", func(t *testing.T) {
		subKeys := [][]byte{make([]byte, 33), make([]byte, 33)}
		subKeys[0][0] = 1
		subKeys[1][0] = 2
		p := &types.MigrationProof{
			Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
				Threshold:     1,
				SubPubKeys:    subKeys,
				SignerIndices: []uint32{0},
				SubSignatures: [][]byte{validSig64},
				SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
			}},
		}
		require.NoError(t, p.ValidateBasic(types.SideLegacy))
	})
	t.Run("neither set", func(t *testing.T) {
		p := &types.MigrationProof{}
		err := p.ValidateBasic(types.SideLegacy)
		require.Error(t, err)
		require.Contains(t, err.Error(), "oneof not set")
	})
	t.Run("nil proof", func(t *testing.T) {
		var p *types.MigrationProof
		err := p.ValidateBasic(types.SideLegacy)
		require.Error(t, err)
		require.Contains(t, err.Error(), "migration_proof required")
	})
}

func TestMigrationProof_ValidateBasic_SingleKey_EIP191_RejectedOnLegacySide(t *testing.T) {
	proof := &types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
		PubKey:    make([]byte, secp256k1.PubKeySize),
		Signature: make([]byte, 64), // legacy-correct length so the EIP191 rejection is what fires
		SigFormat: types.SigFormat_SIG_FORMAT_EIP191,
	}}}
	err := proof.ValidateBasic(types.SideLegacy)
	require.Error(t, err)
	require.ErrorContains(t, err, "EIP191")
}

func TestMigrationProof_ValidateBasic_SingleKey_EIP191_AcceptedOnNewSide(t *testing.T) {
	proof := &types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
		PubKey:    make([]byte, secp256k1.PubKeySize),
		Signature: make([]byte, 65),
		SigFormat: types.SigFormat_SIG_FORMAT_EIP191,
	}}}
	require.NoError(t, proof.ValidateBasic(types.SideNew))
}

func TestMigrationProof_ValidateBasic_SingleKey_RejectWrongSigLenPerSide(t *testing.T) {
	// Legacy side: 65-byte sig rejected.
	legacy := &types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
		PubKey:    make([]byte, secp256k1.PubKeySize),
		Signature: make([]byte, 65),
		SigFormat: types.SigFormat_SIG_FORMAT_CLI,
	}}}
	err := legacy.ValidateBasic(types.SideLegacy)
	require.Error(t, err)
	require.ErrorContains(t, err, "64 bytes")

	// New side: 64-byte sig rejected.
	newSide := &types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
		PubKey:    make([]byte, secp256k1.PubKeySize),
		Signature: make([]byte, 64),
		SigFormat: types.SigFormat_SIG_FORMAT_CLI,
	}}}
	err = newSide.ValidateBasic(types.SideNew)
	require.Error(t, err)
	require.ErrorContains(t, err, "65 bytes")
}

func TestMigrationProof_ValidateBasic_Multisig_EIP191_Rejected(t *testing.T) {
	proof := &types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
		Threshold:     1,
		SubPubKeys:    [][]byte{make([]byte, secp256k1.PubKeySize)},
		SignerIndices: []uint32{0},
		SubSignatures: [][]byte{make([]byte, 64)},
		SigFormat:     types.SigFormat_SIG_FORMAT_EIP191,
	}}}
	for _, side := range []types.Side{types.SideLegacy, types.SideNew} {
		err := proof.ValidateBasic(side)
		require.ErrorContains(t, err, "EIP191")
	}
}

func TestMigrationProof_ValidateBasic_Multisig_RejectWrongSubSigLenPerSide(t *testing.T) {
	legacy := &types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
		Threshold:     1,
		SubPubKeys:    [][]byte{make([]byte, secp256k1.PubKeySize)},
		SignerIndices: []uint32{0},
		SubSignatures: [][]byte{make([]byte, 65)}, // wrong for legacy
		SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
	}}}
	err := legacy.ValidateBasic(types.SideLegacy)
	require.Error(t, err)
	require.ErrorContains(t, err, "64 bytes")

	newSide := &types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
		Threshold:     1,
		SubPubKeys:    [][]byte{make([]byte, secp256k1.PubKeySize)},
		SignerIndices: []uint32{0},
		SubSignatures: [][]byte{make([]byte, 64)}, // wrong for new
		SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
	}}}
	err = newSide.ValidateBasic(types.SideNew)
	require.Error(t, err)
	require.ErrorContains(t, err, "65 bytes")
}

// TestMigrationProof_ValidateBasic_Multisig_AcceptNewSide locks in the positive
// Multisig+SideNew path: a well-formed multisig proof with 65-byte sub-signatures
// and CLI format must be accepted on SideNew.
func TestMigrationProof_ValidateBasic_Multisig_AcceptNewSide(t *testing.T) {
	proof := &types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
		Threshold:     1,
		SubPubKeys:    [][]byte{make([]byte, secp256k1.PubKeySize)},
		SignerIndices: []uint32{0},
		SubSignatures: [][]byte{make([]byte, 65)}, // correct for new side
		SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
	}}}
	require.NoError(t, proof.ValidateBasic(types.SideNew))
}

// TestMigrationProof_ValidateBasic_Multisig_NilMultisigInner locks in the nil
// guard inside MultisigProof.validateBasic. Without the guard, the dispatch at
// MigrationProof.ValidateBasic would hand a nil *MultisigProof into
// validateBasic, which then panics on m.SigFormat access.
func TestMigrationProof_ValidateBasic_Multisig_NilMultisigInner(t *testing.T) {
	proof := &types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: nil}}
	for _, side := range []types.Side{types.SideLegacy, types.SideNew} {
		err := proof.ValidateBasic(side)
		require.Error(t, err, "nil multisig must produce an error, not panic")
		require.ErrorContains(t, err, "multisig proof nil")
	}
}
