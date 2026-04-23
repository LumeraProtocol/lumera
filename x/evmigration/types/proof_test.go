package types_test

import (
	"testing"

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
			wantErr: "must be 33 bytes",
		},
		{
			name:    "empty signature",
			proof:   &types.SingleKeyProof{PubKey: validPK, Signature: nil, SigFormat: types.SigFormat_SIG_FORMAT_CLI},
			wantErr: "signature required",
		},
		{
			name:    "unspecified sig format",
			proof:   &types.SingleKeyProof{PubKey: validPK, Signature: validSig, SigFormat: types.SigFormat_SIG_FORMAT_UNSPECIFIED},
			wantErr: "sig_format required",
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
			wantErr: "must be 33 bytes",
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
	validSig := make([]byte, 64)

	t.Run("single", func(t *testing.T) {
		p := &types.MigrationProof{
			Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
				PubKey: validPK, Signature: validSig, SigFormat: types.SigFormat_SIG_FORMAT_CLI,
			}},
		}
		require.NoError(t, p.ValidateBasic())
	})
	t.Run("multisig", func(t *testing.T) {
		subKeys := [][]byte{make([]byte, 33), make([]byte, 33)}
		subKeys[0][0] = 1
		subKeys[1][0] = 2
		p := &types.MigrationProof{
			Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
				Threshold:     1,
				SubPubKeys:    subKeys,
				SignerIndices: []uint32{0},
				SubSignatures: [][]byte{validSig},
				SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
			}},
		}
		require.NoError(t, p.ValidateBasic())
	})
	t.Run("neither set", func(t *testing.T) {
		p := &types.MigrationProof{}
		err := p.ValidateBasic()
		require.Error(t, err)
		require.Contains(t, err.Error(), "oneof not set")
	})
	t.Run("nil proof", func(t *testing.T) {
		var p *types.MigrationProof
		err := p.ValidateBasic()
		require.Error(t, err)
		require.Contains(t, err.Error(), "legacy_proof required")
	})
}
