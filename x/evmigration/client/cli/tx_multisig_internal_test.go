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

	"github.com/LumeraProtocol/lumera/x/evmigration/types/sigverify"
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
		// PayloadHex contains "lumera1other" not "lumera1new" — mismatch.
		PayloadHex: hex.EncodeToString([]byte("lumera-evm-migration:lumera-test-1:76857769:claim:lumera1legacy:lumera1other")),
		Legacy: &SideSpec{
			PubKey:    "AAAA",
			SigFormat: "SIG_FORMAT_CLI",
		},
		New: &SideSpec{
			PubKey:    "BBBB",
			SigFormat: "SIG_FORMAT_CLI",
		},
		PartialLegacySignatures: []PartialSignature{},
		PartialNewSignatures:    []PartialSignature{},
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
	partials := make([]PartialSignature, len(privs))
	for i, priv := range privs {
		subPubKeys[i] = base64.StdEncoding.EncodeToString(priv.PubKey().Bytes())
		hash := sha256.Sum256(payload)
		sig, err := priv.Sign(hash[:])
		require.NoError(t, err)
		partials[i] = PartialSignature{
			Index:     uint32(i),
			Signature: base64.StdEncoding.EncodeToString(sig),
		}
	}

	// Corrupt the lowest-index signature. The combiner should skip it and keep
	// the other two valid signatures instead of blindly truncating to indices 0,1.
	sig0, err := base64.StdEncoding.DecodeString(partials[0].Signature)
	require.NoError(t, err)
	sig0[0] ^= 0xFF
	partials[0].Signature = base64.StdEncoding.EncodeToString(sig0)

	ss := &SideSpec{
		Threshold:  2,
		SubPubKeys: subPubKeys,
		SigFormat:  "SIG_FORMAT_CLI",
	}
	proof, err := assembleMultisigProof(ss, payload, partials)
	require.NoError(t, err)
	require.Equal(t, []uint32{1, 2}, proof.SignerIndices)
	require.Len(t, proof.SubSignatures, 2)
}

// ---------- upsertSig tests ----------

// TestUpsertSig_ReplacesAtSameIndex verifies that upserting an entry with an
// existing index replaces it in-place (no duplication).
func TestUpsertSig_ReplacesAtSameIndex(t *testing.T) {
	existing := []PartialSignature{
		{Index: 1, Signature: "old1"},
		{Index: 3, Signature: "old3"},
	}
	result := upsertSig(existing, PartialSignature{Index: 3, Signature: "new3"})
	require.Len(t, result, 2)
	// Index 3 must have the updated signature.
	var found bool
	for _, s := range result {
		if s.Index == 3 {
			require.Equal(t, "new3", s.Signature)
			found = true
		}
	}
	require.True(t, found, "index 3 must be present after upsert")
}

// TestUpsertSig_AppendsNewIndex verifies that upserting a new index appends it.
func TestUpsertSig_AppendsNewIndex(t *testing.T) {
	existing := []PartialSignature{
		{Index: 1, Signature: "sig1"},
		{Index: 3, Signature: "sig3"},
	}
	result := upsertSig(existing, PartialSignature{Index: 5, Signature: "sig5"})
	require.Len(t, result, 3)
	require.Equal(t, uint32(5), result[2].Index)
	require.Equal(t, "sig5", result[2].Signature)
}

// TestUpsertSig_IdempotentTwice verifies that running upsert twice with the
// same index leaves exactly one entry at that index.
func TestUpsertSig_IdempotentTwice(t *testing.T) {
	existing := []PartialSignature{{Index: 2, Signature: "first"}}
	after1 := upsertSig(existing, PartialSignature{Index: 2, Signature: "second"})
	after2 := upsertSig(after1, PartialSignature{Index: 2, Signature: "third"})
	var count int
	for _, s := range after2 {
		if s.Index == 2 {
			count++
			require.Equal(t, "third", s.Signature)
		}
	}
	require.Equal(t, 1, count, "upsert must not duplicate entries")
}

// ---------- legacySigningInput tests ----------

func TestLegacySigningInput_CLI(t *testing.T) {
	payload := []byte("test-payload")
	out, err := legacySigningInput(payload, "SIG_FORMAT_CLI", "lumera1abc")
	require.NoError(t, err)
	h := sha256.Sum256(payload)
	require.Equal(t, h[:], out)
}

func TestLegacySigningInput_ADR036(t *testing.T) {
	payload := []byte("test-payload")
	signer := "lumera1abc"
	out, err := legacySigningInput(payload, "SIG_FORMAT_ADR036", signer)
	require.NoError(t, err)
	require.Contains(t, string(out), signer)
	require.Contains(t, string(out), "sign/MsgSignData")
}

func TestLegacySigningInput_EIP191_Errors(t *testing.T) {
	_, err := legacySigningInput([]byte("p"), "SIG_FORMAT_EIP191", "lumera1abc")
	require.ErrorContains(t, err, "not valid on the legacy side")
}

func TestLegacySigningInput_Unknown_Errors(t *testing.T) {
	_, err := legacySigningInput([]byte("p"), "SIG_FORMAT_UNKNOWN", "lumera1abc")
	require.ErrorContains(t, err, "unsupported legacy sig_format")
}

// ---------- newSigningInput tests ----------

func TestNewSigningInput_CLI_PassesPayloadThrough(t *testing.T) {
	payload := []byte("test-payload")
	out, err := newSigningInput(payload, "SIG_FORMAT_CLI", "lumera1abc")
	require.NoError(t, err)
	// eth keyring applies Keccak256 internally — we must pass raw payload.
	require.Equal(t, payload, out)
}

func TestNewSigningInput_EIP191_WrapsPayload(t *testing.T) {
	payload := []byte("test-payload")
	out, err := newSigningInput(payload, "SIG_FORMAT_EIP191", "lumera1abc")
	require.NoError(t, err)
	require.Contains(t, string(out), "\x19Ethereum Signed Message:")
}

func TestNewSigningInput_ADR036(t *testing.T) {
	payload := []byte("test-payload")
	signer := "lumera1abc"
	out, err := newSigningInput(payload, "SIG_FORMAT_ADR036", signer)
	require.NoError(t, err)
	require.Contains(t, string(out), signer)
	require.Contains(t, string(out), "sign/MsgSignData")
}

func TestNewSigningInput_Unknown_Errors(t *testing.T) {
	_, err := newSigningInput([]byte("p"), "SIG_FORMAT_UNKNOWN", "lumera1abc")
	require.ErrorContains(t, err, "unsupported new sig_format")
}

// ---------- findSubKeyIndex tests ----------

// TestFindSubKeyIndex_WrongTypeCosmosForNew verifies that supplying a Cosmos
// secp256k1 key via --new-key (expected eth_secp256k1) produces a type error.
func TestFindSubKeyIndex_WrongTypeCosmosForNew(t *testing.T) {
	kr := newTestKeyring(t)
	cosmosRec := addLegacyKey(t, kr, "cosmos-key", testMnemonic)
	pk, err := cosmosRec.GetPubKey()
	require.NoError(t, err)

	spec := &SideSpec{
		PubKey:    base64.StdEncoding.EncodeToString(pk.Bytes()),
		SigFormat: "SIG_FORMAT_CLI",
	}
	ctx := clientCtxWithKeyring(kr)
	_, err = findSubKeyIndex(ctx, "cosmos-key", spec, sigverify.SubKeyTypeEthSecp256k1)
	require.ErrorContains(t, err, "expected eth_secp256k1")
}

// TestFindSubKeyIndex_WrongTypeEthForLegacy verifies that supplying an
// eth_secp256k1 key via --from (expected Cosmos secp256k1) produces a type error.
func TestFindSubKeyIndex_WrongTypeEthForLegacy(t *testing.T) {
	kr := newTestKeyring(t)
	ethRec := addEVMKey(t, kr, "eth-key", testMnemonic)
	pk, err := ethRec.GetPubKey()
	require.NoError(t, err)

	spec := &SideSpec{
		PubKey:    base64.StdEncoding.EncodeToString(pk.Bytes()),
		SigFormat: "SIG_FORMAT_CLI",
	}
	ctx := clientCtxWithKeyring(kr)
	_, err = findSubKeyIndex(ctx, "eth-key", spec, sigverify.SubKeyTypeCosmosSecp256k1)
	require.ErrorContains(t, err, "expected Cosmos secp256k1")
}

// TestFindSubKeyIndex_SingleKeyMatch verifies that a matching single-key spec
// returns index 0 without error.
func TestFindSubKeyIndex_SingleKeyMatch(t *testing.T) {
	kr := newTestKeyring(t)
	rec := addLegacyKey(t, kr, "legacy-key", testMnemonic)
	pk, err := rec.GetPubKey()
	require.NoError(t, err)

	spec := &SideSpec{
		PubKey:    base64.StdEncoding.EncodeToString(pk.Bytes()),
		SigFormat: "SIG_FORMAT_CLI",
	}
	ctx := clientCtxWithKeyring(kr)
	idx, err := findSubKeyIndex(ctx, "legacy-key", spec, sigverify.SubKeyTypeCosmosSecp256k1)
	require.NoError(t, err)
	require.Equal(t, uint32(0), idx)
}

// TestFindSubKeyIndex_MultisigMatch verifies that a key matching the second
// entry in a multisig spec returns index 1.
func TestFindSubKeyIndex_MultisigMatch(t *testing.T) {
	kr := newTestKeyring(t)
	_ = addLegacyKey(t, kr, "key0", testMnemonic)

	// Generate a distinct key for index 1.
	priv1 := secp256k1.GenPrivKey()
	_, err := kr.SaveOfflineKey("key1", priv1.PubKey())
	require.NoError(t, err)

	spec := &SideSpec{
		Threshold: 1,
		SubPubKeys: []string{
			base64.StdEncoding.EncodeToString(secp256k1.GenPrivKey().PubKey().Bytes()),
			base64.StdEncoding.EncodeToString(priv1.PubKey().Bytes()),
		},
		SigFormat: "SIG_FORMAT_CLI",
	}
	ctx := clientCtxWithKeyring(kr)
	idx, err := findSubKeyIndex(ctx, "key1", spec, sigverify.SubKeyTypeCosmosSecp256k1)
	require.NoError(t, err)
	require.Equal(t, uint32(1), idx)
}

// TestFindSubKeyIndex_KeyNotFound verifies that a missing keyring name errors.
func TestFindSubKeyIndex_KeyNotFound(t *testing.T) {
	kr := newTestKeyring(t)
	spec := &SideSpec{PubKey: "AAAA", SigFormat: "SIG_FORMAT_CLI"}
	ctx := clientCtxWithKeyring(kr)
	_, err := findSubKeyIndex(ctx, "nonexistent", spec, sigverify.SubKeyTypeCosmosSecp256k1)
	require.ErrorContains(t, err, "not found in keyring")
}
