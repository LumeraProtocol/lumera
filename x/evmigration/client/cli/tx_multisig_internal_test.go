package cli

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
)

func TestLoadPartialProof_RejectsPayloadFieldMismatch(t *testing.T) {
	t.Helper()

	pp := &PartialProof{
		Version:       partialProofVersion,
		Kind:          "claim",
		LegacyAddress: "lumera1legacy",
		NewAddress:    "lumera1new",
		ChainID:       "lumera-test-1",
		EVMChainID:    76857769,
		PayloadHex:    hex.EncodeToString([]byte("lumera-evm-migration:lumera-test-1:76857769:claim:lumera1legacy:lumera1other")),
		Single: &PartialSingle{
			PubKeyB64: "AAAA",
			SigFormat: "SIG_FORMAT_CLI",
		},
		PartialSigs: []PartialSubSignature{},
	}

	b, err := json.Marshal(pp)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "proof.json")
	require.NoError(t, os.WriteFile(path, b, 0o600))

	_, err = LoadPartialProof(path)
	require.ErrorContains(t, err, "payload_hex does not match")
}

func TestAssembleMultisigProof_PrefersValidSubsetWhenExtrasPresent(t *testing.T) {
	privs := []*secp256k1.PrivKey{
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
	}
	payload := []byte("lumera-evm-migration:lumera-test-1:76857769:claim:legacy:new")

	subPubKeys := make([]string, len(privs))
	partials := make([]PartialSubSignature, len(privs))
	for i, priv := range privs {
		subPubKeys[i] = base64.StdEncoding.EncodeToString(priv.PubKey().Bytes())
		hash := sha256.Sum256(payload)
		sig, err := priv.Sign(hash[:])
		require.NoError(t, err)
		partials[i] = PartialSubSignature{
			Index:        uint32(i),
			SignatureB64: base64.StdEncoding.EncodeToString(sig),
		}
	}

	// Corrupt the lowest-index signature. The combiner should skip it and keep
	// the other two valid signatures instead of blindly truncating to indices 0,1.
	sig0, err := base64.StdEncoding.DecodeString(partials[0].SignatureB64)
	require.NoError(t, err)
	sig0[0] ^= 0xFF
	partials[0].SignatureB64 = base64.StdEncoding.EncodeToString(sig0)

	proof, err := assembleMultisigProof(&PartialMultisig{
		Threshold:     2,
		SubPubKeysB64: subPubKeys,
		SigFormat:     "SIG_FORMAT_CLI",
	}, payload, partials)
	require.NoError(t, err)
	require.Equal(t, []uint32{1, 2}, proof.SignerIndices)
	require.Len(t, proof.SubSignatures, 2)
}
