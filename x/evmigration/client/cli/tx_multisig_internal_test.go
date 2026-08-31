package cli

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	kmultisig "github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ethsecp256k1 "github.com/cosmos/evm/crypto/ethsecp256k1"
	evmcryptotypes "github.com/cosmos/evm/crypto/ethsecp256k1"

	"github.com/LumeraProtocol/lumera/x/evmigration/keeper"
	"github.com/LumeraProtocol/lumera/x/evmigration/types"
	"github.com/LumeraProtocol/lumera/x/evmigration/types/sigverify"
)

// cosmosSign produces a SIG_FORMAT_CLI Cosmos secp256k1 signature over payload.
func cosmosSign(t *testing.T, priv *secp256k1.PrivKey, payload []byte) []byte {
	t.Helper()
	hash := sha256.Sum256(payload)
	sig, err := priv.Sign(hash[:])
	require.NoError(t, err)
	return sig
}

// ethSign produces a SIG_FORMAT_CLI eth_secp256k1 signature over payload
// (65-byte R||S||V; the eth keyring applies Keccak256 internally).
func ethSign(t *testing.T, priv *ethsecp256k1.PrivKey, payload []byte) []byte {
	t.Helper()
	sig, err := priv.Sign(payload)
	require.NoError(t, err)
	return sig
}

// ---------- buildProofFromPartial — single-key Cosmos side ----------

func TestBuildProofFromPartial_SingleKey_Valid(t *testing.T) {
	priv := secp256k1.GenPrivKey()
	pk := priv.PubKey().(*secp256k1.PubKey)
	payload := []byte("lumera-evm-migration:lumera-test-1:76857769:claim:legacy:new")
	sig := cosmosSign(t, priv, payload)

	side := &SideSpec{
		PubKey:    base64.StdEncoding.EncodeToString(pk.Bytes()),
		SigFormat: "SIG_FORMAT_CLI",
	}
	sigs := []PartialSignature{{Index: 0, Signature: base64.StdEncoding.EncodeToString(sig)}}

	proof, err := buildProofFromPartial(side, sigs, payload, sigverify.SubKeyTypeCosmosSecp256k1, "legacy", io.Discard)
	require.NoError(t, err)
	sp := proof.GetSingle()
	require.NotNil(t, sp)
	require.Equal(t, pk.Bytes(), sp.PubKey)
	require.Equal(t, sig, sp.Signature)
	require.Equal(t, types.SigFormat_SIG_FORMAT_CLI, sp.SigFormat)
}

func TestBuildProofFromPartial_SingleKey_MissingSig(t *testing.T) {
	priv := secp256k1.GenPrivKey()
	pk := priv.PubKey().(*secp256k1.PubKey)
	side := &SideSpec{
		PubKey:    base64.StdEncoding.EncodeToString(pk.Bytes()),
		SigFormat: "SIG_FORMAT_CLI",
	}
	_, err := buildProofFromPartial(side, nil, []byte("payload"), sigverify.SubKeyTypeCosmosSecp256k1, "legacy", io.Discard)
	require.ErrorContains(t, err, "no partial signature")
}

func TestBuildProofFromPartial_SingleKey_TooMany(t *testing.T) {
	priv := secp256k1.GenPrivKey()
	pk := priv.PubKey().(*secp256k1.PubKey)
	side := &SideSpec{
		PubKey:    base64.StdEncoding.EncodeToString(pk.Bytes()),
		SigFormat: "SIG_FORMAT_CLI",
	}
	sigs := []PartialSignature{
		{Index: 0, Signature: "AAAA"},
		{Index: 0, Signature: "BBBB"},
	}
	_, err := buildProofFromPartial(side, sigs, []byte("payload"), sigverify.SubKeyTypeCosmosSecp256k1, "legacy", io.Discard)
	require.ErrorContains(t, err, "single-key side expects exactly one at index 0")
}

func TestBuildProofFromPartial_SingleKey_WrongIndex(t *testing.T) {
	priv := secp256k1.GenPrivKey()
	pk := priv.PubKey().(*secp256k1.PubKey)
	side := &SideSpec{
		PubKey:    base64.StdEncoding.EncodeToString(pk.Bytes()),
		SigFormat: "SIG_FORMAT_CLI",
	}
	sigs := []PartialSignature{{Index: 1, Signature: "AAAA"}}
	_, err := buildProofFromPartial(side, sigs, []byte("payload"), sigverify.SubKeyTypeCosmosSecp256k1, "legacy", io.Discard)
	require.ErrorContains(t, err, "index=1")
	require.ErrorContains(t, err, "expects index=0")
}

// TestBuildProofFromPartial_SingleKey_InvalidSig_Aborts exercises the single-key
// crypto-failure branch at tx_multisig.go:288. Unlike multisig (drop-and-warn),
// the single-key side has no fallback signer, so a bad signature must abort.
func TestBuildProofFromPartial_SingleKey_InvalidSig_Aborts(t *testing.T) {
	priv := secp256k1.GenPrivKey()
	pk := priv.PubKey().(*secp256k1.PubKey)
	payload := []byte("lumera-evm-migration:lumera-test-1:76857769:claim:legacy:new")

	// 64 zero bytes is a well-formed-looking but cryptographically invalid
	// Cosmos secp256k1 signature — passes length checks, fails verification.
	garbageSig := make([]byte, 64)

	side := &SideSpec{
		PubKey:    base64.StdEncoding.EncodeToString(pk.Bytes()),
		SigFormat: "SIG_FORMAT_CLI",
	}
	sigs := []PartialSignature{{Index: 0, Signature: base64.StdEncoding.EncodeToString(garbageSig)}}

	_, err := buildProofFromPartial(side, sigs, payload, sigverify.SubKeyTypeCosmosSecp256k1, "legacy", io.Discard)
	require.Error(t, err)
	require.ErrorContains(t, err, "single-key partial signature invalid")
}

func TestValidateMatchingMultisigSignerIndices_AllowsMatching(t *testing.T) {
	err := validateMatchingMultisigSignerIndices("alice-legacy", "alice-evm", false, false, 1, 1)
	require.NoError(t, err)
}

func TestValidateMatchingMultisigSignerIndices_RejectsMismatch(t *testing.T) {
	err := validateMatchingMultisigSignerIndices("alice-legacy", "alice-evm", false, false, 0, 3)
	require.ErrorContains(t, err, "legacy key \"alice-legacy\" is signer index 0")
	require.ErrorContains(t, err, "new key \"alice-evm\" is signer index 3")
	require.ErrorContains(t, err, "same signer position")
}

func TestValidateMatchingMultisigSignerIndices_SkipsSingleKeySide(t *testing.T) {
	require.NoError(t, validateMatchingMultisigSignerIndices("legacy", "new", true, false, 0, 2))
	require.NoError(t, validateMatchingMultisigSignerIndices("legacy", "new", false, true, 0, 2))
	require.NoError(t, validateMatchingMultisigSignerIndices("legacy", "new", true, true, 0, 2))
}

// ---------- buildProofFromPartial — multisig Cosmos side ----------

func TestBuildProofFromPartial_Multisig_Valid2of3(t *testing.T) {
	privs := []*secp256k1.PrivKey{
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
	}
	payload := []byte("lumera-evm-migration:lumera-test-1:76857769:claim:legacy:new")

	subPubKeys := make([]string, len(privs))
	for i, p := range privs {
		subPubKeys[i] = base64.StdEncoding.EncodeToString(p.PubKey().Bytes())
	}

	// Provide valid sigs at indices 0 and 2.
	sigs := []PartialSignature{
		{Index: 0, Signature: base64.StdEncoding.EncodeToString(cosmosSign(t, privs[0], payload))},
		{Index: 2, Signature: base64.StdEncoding.EncodeToString(cosmosSign(t, privs[2], payload))},
	}
	side := &SideSpec{Threshold: 2, SubPubKeys: subPubKeys, SigFormat: "SIG_FORMAT_CLI"}

	proof, err := buildProofFromPartial(side, sigs, payload, sigverify.SubKeyTypeCosmosSecp256k1, "legacy", io.Discard)
	require.NoError(t, err)
	mp := proof.GetMultisig()
	require.NotNil(t, mp)
	require.Equal(t, []uint32{0, 2}, mp.SignerIndices)
	require.Len(t, mp.SubSignatures, 2)
}

func TestBuildProofFromPartial_Multisig_DropsInvalidPartial_SelectsValidHigherIndex(t *testing.T) {
	privs := []*secp256k1.PrivKey{
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
	}
	payload := []byte("lumera-evm-migration:lumera-test-1:76857769:claim:legacy:new")
	wrongPayload := []byte("lumera-evm-migration:lumera-test-1:76857769:claim:legacy:WRONG")

	subPubKeys := make([]string, len(privs))
	for i, p := range privs {
		subPubKeys[i] = base64.StdEncoding.EncodeToString(p.PubKey().Bytes())
	}

	// Index 0: signed over the wrong payload — will fail verification.
	// Indices 1 and 2: valid.
	sigs := []PartialSignature{
		{Index: 0, Signature: base64.StdEncoding.EncodeToString(cosmosSign(t, privs[0], wrongPayload))},
		{Index: 1, Signature: base64.StdEncoding.EncodeToString(cosmosSign(t, privs[1], payload))},
		{Index: 2, Signature: base64.StdEncoding.EncodeToString(cosmosSign(t, privs[2], payload))},
	}
	side := &SideSpec{Threshold: 2, SubPubKeys: subPubKeys, SigFormat: "SIG_FORMAT_CLI"}

	proof, err := buildProofFromPartial(side, sigs, payload, sigverify.SubKeyTypeCosmosSecp256k1, "legacy", io.Discard)
	require.NoError(t, err)
	mp := proof.GetMultisig()
	require.NotNil(t, mp)
	// Index 0 must be dropped; result must use indices 1 and 2.
	require.Equal(t, []uint32{1, 2}, mp.SignerIndices)
	require.Len(t, mp.SubSignatures, 2)
}

func TestBuildProofFromPartial_Multisig_BelowThresholdAfterDrops(t *testing.T) {
	privs := []*secp256k1.PrivKey{
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
	}
	payload := []byte("lumera-evm-migration:lumera-test-1:76857769:claim:legacy:new")
	wrongPayload := []byte("wrong-payload")

	subPubKeys := make([]string, len(privs))
	for i, p := range privs {
		subPubKeys[i] = base64.StdEncoding.EncodeToString(p.PubKey().Bytes())
	}

	// Both sigs signed over wrong payload — both invalid.
	sigs := []PartialSignature{
		{Index: 0, Signature: base64.StdEncoding.EncodeToString(cosmosSign(t, privs[0], wrongPayload))},
		{Index: 1, Signature: base64.StdEncoding.EncodeToString(cosmosSign(t, privs[1], wrongPayload))},
	}
	side := &SideSpec{Threshold: 2, SubPubKeys: subPubKeys, SigFormat: "SIG_FORMAT_CLI"}

	_, err := buildProofFromPartial(side, sigs, payload, sigverify.SubKeyTypeCosmosSecp256k1, "legacy", io.Discard)
	require.ErrorContains(t, err, "need 2 valid partial signatures on legacy side, have 0")
}

func TestBuildProofFromPartial_Multisig_OutOfRangeIndex_Dropped(t *testing.T) {
	priv := secp256k1.GenPrivKey()
	payload := []byte("lumera-evm-migration:lumera-test-1:76857769:claim:legacy:new")

	subPubKeys := []string{
		base64.StdEncoding.EncodeToString(priv.PubKey().Bytes()),
	}
	// Index 5 is out of range for N=1; index 0 is valid.
	sigs := []PartialSignature{
		{Index: 0, Signature: base64.StdEncoding.EncodeToString(cosmosSign(t, priv, payload))},
		{Index: 5, Signature: base64.StdEncoding.EncodeToString(cosmosSign(t, priv, payload))},
	}
	side := &SideSpec{Threshold: 1, SubPubKeys: subPubKeys, SigFormat: "SIG_FORMAT_CLI"}

	proof, err := buildProofFromPartial(side, sigs, payload, sigverify.SubKeyTypeCosmosSecp256k1, "legacy", io.Discard)
	require.NoError(t, err)
	mp := proof.GetMultisig()
	require.NotNil(t, mp)
	// Only index 0 survives; index 5 is dropped.
	require.Equal(t, []uint32{0}, mp.SignerIndices)
}

// TestBuildProofFromPartial_Multisig_BadBase64Sig_Dropped exercises the drop-and-warn
// branch at tx_multisig.go:319-323 where a sig entry fails base64 decode. This is
// structurally distinct from out-of-range-index and bad-crypto-sig drops: it catches
// corruption at the decode step, before crypto verification is even attempted.
func TestBuildProofFromPartial_Multisig_BadBase64Sig_Dropped(t *testing.T) {
	privs := []*secp256k1.PrivKey{
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
	}
	payload := []byte("lumera-evm-migration:lumera-test-1:76857769:claim:legacy:new")

	subPubKeys := make([]string, len(privs))
	for i, p := range privs {
		subPubKeys[i] = base64.StdEncoding.EncodeToString(p.PubKey().Bytes())
	}

	// Indices 0 and 1: valid base64 + valid sigs. Index 2: malformed base64.
	sigs := []PartialSignature{
		{Index: 0, Signature: base64.StdEncoding.EncodeToString(cosmosSign(t, privs[0], payload))},
		{Index: 1, Signature: base64.StdEncoding.EncodeToString(cosmosSign(t, privs[1], payload))},
		{Index: 2, Signature: "!!!not-base64!!!"},
	}
	side := &SideSpec{Threshold: 2, SubPubKeys: subPubKeys, SigFormat: "SIG_FORMAT_CLI"}

	proof, err := buildProofFromPartial(side, sigs, payload, sigverify.SubKeyTypeCosmosSecp256k1, "legacy", io.Discard)
	require.NoError(t, err)
	mp := proof.GetMultisig()
	require.NotNil(t, mp)
	// Index 2 dropped at decode step; result uses only indices 0 and 1.
	require.Equal(t, []uint32{0, 1}, mp.SignerIndices)
	require.Len(t, mp.SubSignatures, 2)
}

// TestBuildProofFromPartial_Multisig_DedupeDuplicateIndex exercises the
// dedupe guard: two partial entries at the same Index should be collapsed
// to one in the resulting MultisigProof (keeping the first).
func TestBuildProofFromPartial_Multisig_DedupeDuplicateIndex(t *testing.T) {
	privs := []*secp256k1.PrivKey{
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
	}
	payload := []byte("lumera-evm-migration:lumera-test-1:76857769:claim:legacy:new")
	subPubKeys := make([]string, len(privs))
	for i, p := range privs {
		subPubKeys[i] = base64.StdEncoding.EncodeToString(p.PubKey().Bytes())
	}

	// Two valid sigs at Index=0 (same signer, same payload — deterministic),
	// plus one at Index=1. Below dedupe, all three would land in the proof.
	sig0a := cosmosSign(t, privs[0], payload)
	sig0b := cosmosSign(t, privs[0], payload) // second invocation of same signer
	sig1 := cosmosSign(t, privs[1], payload)
	sigs := []PartialSignature{
		{Index: 0, Signature: base64.StdEncoding.EncodeToString(sig0a)},
		{Index: 0, Signature: base64.StdEncoding.EncodeToString(sig0b)},
		{Index: 1, Signature: base64.StdEncoding.EncodeToString(sig1)},
	}
	side := &SideSpec{Threshold: 2, SubPubKeys: subPubKeys, SigFormat: "SIG_FORMAT_CLI"}

	proof, err := buildProofFromPartial(side, sigs, payload, sigverify.SubKeyTypeCosmosSecp256k1, "legacy", io.Discard)
	require.NoError(t, err)
	mp := proof.GetMultisig()
	require.NotNil(t, mp)
	require.Equal(t, []uint32{0, 1}, mp.SignerIndices)
	require.Len(t, mp.SubSignatures, 2)
}

func TestBuildProofFromPartial_Multisig_WrongPubKeyLength(t *testing.T) {
	// SubPubKeys entry decodes to only 4 bytes — must error.
	side := &SideSpec{
		Threshold:  1,
		SubPubKeys: []string{base64.StdEncoding.EncodeToString([]byte{0x01, 0x02, 0x03, 0x04})},
		SigFormat:  "SIG_FORMAT_CLI",
	}
	_, err := buildProofFromPartial(side, nil, []byte("payload"), sigverify.SubKeyTypeCosmosSecp256k1, "legacy", io.Discard)
	require.ErrorContains(t, err, "sub_pub_keys[0]")
	require.ErrorContains(t, err, "expected 33 bytes")
}

// ---------- buildProofFromPartial — single-key eth side ----------

func TestBuildProofFromPartial_SingleKey_Eth_Valid(t *testing.T) {
	priv, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	pk := priv.PubKey().(*evmcryptotypes.PubKey)
	payload := []byte("lumera-evm-migration:lumera-test-1:76857769:claim:legacy:new")
	sig := ethSign(t, priv, payload)
	require.Len(t, sig, 65)

	// Verify the eth sig verifies (sanity check before feeding buildProofFromPartial).
	require.NoError(t, sigverify.VerifyEthSecp256k1(pk, sdk.AccAddress(pk.Address()), payload, sig, types.SigFormat_SIG_FORMAT_CLI))

	side := &SideSpec{
		PubKey:    base64.StdEncoding.EncodeToString(pk.Key),
		SigFormat: "SIG_FORMAT_CLI",
	}
	sigs := []PartialSignature{{Index: 0, Signature: base64.StdEncoding.EncodeToString(sig)}}

	proof, err := buildProofFromPartial(side, sigs, payload, sigverify.SubKeyTypeEthSecp256k1, "new", io.Discard)
	require.NoError(t, err)
	sp := proof.GetSingle()
	require.NotNil(t, sp)
	require.Equal(t, pk.Key, sp.PubKey)
}

// ---------- AssertPartialProofsConsistent mismatch ----------

func TestCombineProof_MismatchedPayloads_Rejected(t *testing.T) {
	// Build two PartialProof files that differ in chain_id. Write each to disk,
	// then call AssertPartialProofsConsistent directly (no tx config needed).
	makeProof := func(chainID string) *PartialProof {
		payload := []byte(ComputePayload(chainID, 76857769, "claim", "lumera1legacy", "lumera1new"))
		return &PartialProof{
			Version:                 partialProofVersion,
			Kind:                    "claim",
			LegacyAddress:           "lumera1legacy",
			NewAddress:              "lumera1new",
			ChainID:                 chainID,
			EVMChainID:              76857769,
			PayloadHex:              hex.EncodeToString(payload),
			Legacy:                  &SideSpec{PubKey: "AAAA", SigFormat: "SIG_FORMAT_CLI"},
			New:                     &SideSpec{PubKey: "BBBB", SigFormat: "SIG_FORMAT_CLI"},
			PartialLegacySignatures: []PartialSignature{},
			PartialNewSignatures:    []PartialSignature{},
		}
	}

	a := makeProof("lumera-test-1")
	b := makeProof("lumera-test-2")

	err := AssertPartialProofsConsistent(a, b)
	require.Error(t, err)
	require.ErrorContains(t, err, "chain_id differs")
}

// ---------- existing tests below (unchanged) ----------

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

// TestBuildProofFromPartial_PrefersValidSubsetWhenExtrasPresent replaces the
// deleted TestAssembleMultisigProof_PrefersValidSubsetWhenExtrasPresent.
func TestBuildProofFromPartial_PrefersValidSubsetWhenExtrasPresent(t *testing.T) {
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
		sig := cosmosSign(t, priv, payload)
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
	proof, err := buildProofFromPartial(ss, partials, payload, sigverify.SubKeyTypeCosmosSecp256k1, "legacy", io.Discard)
	require.NoError(t, err)
	mp := proof.GetMultisig()
	require.NotNil(t, mp)
	require.Equal(t, []uint32{1, 2}, mp.SignerIndices)
	require.Len(t, mp.SubSignatures, 2)
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

// ---------- cmdSubmitProof regression-lock tests ----------

// TestCmdSubmitProof_HelpTextDoesNotMentionSigning locks down the post-Task-13
// contract that submit-proof does not sign anything. If a future refactor
// reintroduces signing language into the help text, this test fails, forcing
// the author to explicitly acknowledge the change. The actual runtime path is
// runMigrationTx -> BuildUnsignedTx -> BroadcastTx (no signing).
func TestCmdSubmitProof_HelpTextDoesNotMentionSigning(t *testing.T) {
	cmd := cmdSubmitProof()
	help := cmd.Short + "\n" + cmd.Long
	for _, banned := range []string{"sign", "Sign", "signature", "new_signature", "--from eth"} {
		require.NotContains(t, help, banned,
			"submit-proof help text must not mention %q; the command does not sign anything", banned)
	}
	// Sanity-check: make sure we actually read something (guards against a future
	// refactor that strips Short entirely and silently passes the banned-strings check).
	require.NotEmpty(t, strings.TrimSpace(cmd.Short), "Short description must be set")
}

// TestCmdSubmitProof_DoesNotExposeSigningFlags locks in the narrower flag
// surface: migration txs are unsigned at the Cosmos layer (GetSigners()
// returns empty, fees waived by the ante handler), so --from / --fees /
// --gas / --sign-mode / --fee-payer have no effect and should NOT be
// advertised on --help. A future change that reintroduces flags.AddTxFlagsToCmd
// here would re-add these and mislead operators into thinking they need to
// sign or pay.
func TestCmdSubmitProof_DoesNotExposeSigningFlags(t *testing.T) {
	cmd := cmdSubmitProof()
	for _, name := range []string{
		"from", "fees", "fee-payer", "fee-granter",
		"gas", "gas-adjustment", "gas-prices",
		"sign-mode", "offline", "generate-only",
	} {
		require.Nilf(t, cmd.Flags().Lookup(name),
			"--%s must NOT be advertised on submit-proof (command does not sign or pay)", name)
	}
	// Flags that DO affect submit-proof should still be present.
	for _, name := range []string{
		"node", "chain-id", "keyring-backend", "keyring-dir",
		"broadcast-mode", "yes", "tx-timeout",
	} {
		require.NotNilf(t, cmd.Flags().Lookup(name), "--%s must be advertised on submit-proof", name)
	}
}

// ---------- End-to-end multisig→multisig pipeline ----------

// TestCLI_MultisigToMultisig_EndToEnd exercises the complete four-step CLI
// flow IN PROCESS with real file IO between stages, but without a test network.
// It proves that the output of combine-proof is a valid MigrationProof for
// both sides that would pass server-side keeper.VerifyMigrationProof.
//
// Pipeline:
//  1. generate-proof-payload → unsigned PartialProof on disk
//  2. sign-proof (cosigner #1) → signed-1.json with legacy[0] + new[0]
//  3. sign-proof (cosigner #2) → signed-2.json with legacy[1] + new[1]
//  4. combine-proof → merge, buildProofFromPartial per side → MigrationProof
//
// Assertions:
//   - Each side emits MultisigProof{Threshold=2, SignerIndices=[0,1]}.
//   - keeper.VerifyMigrationProof passes for both sides with the right
//     keyType and boundAddr.
//   - MsgClaimLegacyAccount{LegacyProof, NewProof}.ValidateBasic passes.
//
// Deliberately skipped: network-backed tx submission (--from/keyring/grpc
// plumbing). That surface is covered by Tasks 19-21 which drive the msg
// server directly with real keeper state.
func TestCLI_MultisigToMultisig_EndToEnd(t *testing.T) {
	const (
		chainID    = "lumera-test-1"
		evmChainID = uint64(76857769)
		kind       = "claim"
	)

	// === SETUP: legacy side — 3 Cosmos secp256k1 sub-keys, 2-of-3 multisig ===
	legacyPrivs := []*secp256k1.PrivKey{
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
	}
	legacySubPubs := make([]cryptotypes.PubKey, len(legacyPrivs))
	legacySubB64 := make([]string, len(legacyPrivs))
	for i, p := range legacyPrivs {
		legacySubPubs[i] = p.PubKey()
		legacySubB64[i] = base64.StdEncoding.EncodeToString(p.PubKey().Bytes())
	}
	legacyMultiPK := kmultisig.NewLegacyAminoPubKey(2, legacySubPubs)
	legacyAddr := sdk.AccAddress(legacyMultiPK.Address())

	// === SETUP: new side — 3 eth_secp256k1 sub-keys, 2-of-3 multisig ===
	ethPrivs := make([]*ethsecp256k1.PrivKey, 3)
	newSubPubs := make([]cryptotypes.PubKey, 3)
	newSubB64 := make([]string, 3)
	for i := range ethPrivs {
		p, err := ethsecp256k1.GenerateKey()
		require.NoError(t, err)
		ethPrivs[i] = p
		pk := p.PubKey().(*evmcryptotypes.PubKey)
		newSubPubs[i] = pk
		newSubB64[i] = base64.StdEncoding.EncodeToString(pk.Key)
	}
	newMultiPK := kmultisig.NewLegacyAminoPubKey(2, newSubPubs)
	newAddr := sdk.AccAddress(newMultiPK.Address())

	// Canonical payload — same formula the keeper uses for verification.
	payloadStr := ComputePayload(chainID, evmChainID, kind, legacyAddr.String(), newAddr.String())
	payload := []byte(payloadStr)
	payloadHex := hex.EncodeToString(payload)

	tmpDir := t.TempDir()
	payloadPath := filepath.Join(tmpDir, "payload.json")
	signed1Path := filepath.Join(tmpDir, "signed-1.json")
	signed2Path := filepath.Join(tmpDir, "signed-2.json")

	// === STEP A: generate-proof-payload (in-memory equivalent) ===
	ppA := &PartialProof{
		Version:       partialProofVersion,
		Kind:          kind,
		LegacyAddress: legacyAddr.String(),
		NewAddress:    newAddr.String(),
		ChainID:       chainID,
		EVMChainID:    evmChainID,
		PayloadHex:    payloadHex,
		Legacy: &SideSpec{
			Threshold:  2,
			SubPubKeys: legacySubB64,
			SigFormat:  "SIG_FORMAT_CLI",
		},
		New: &SideSpec{
			Threshold:  2,
			SubPubKeys: newSubB64,
			SigFormat:  "SIG_FORMAT_CLI",
		},
		PartialLegacySignatures: []PartialSignature{},
		PartialNewSignatures:    []PartialSignature{},
	}
	require.NoError(t, SavePartialProof(payloadPath, ppA))
	// Round-trip through the loader to catch schema/validation regressions.
	loaded, err := LoadPartialProof(payloadPath)
	require.NoError(t, err)
	require.Equal(t, partialProofVersion, loaded.Version)
	require.Equal(t, kind, loaded.Kind)
	require.Equal(t, legacyAddr.String(), loaded.LegacyAddress)
	require.Equal(t, newAddr.String(), loaded.NewAddress)

	// === STEP B: sign-proof cosigner #1 (legacy-sub-0 + eth-sub-0) ===
	pp1, err := LoadPartialProof(payloadPath)
	require.NoError(t, err)
	pp1.PartialLegacySignatures = upsertSig(pp1.PartialLegacySignatures, PartialSignature{
		Index:     0,
		Signature: base64.StdEncoding.EncodeToString(cosmosSign(t, legacyPrivs[0], payload)),
	})
	pp1.PartialNewSignatures = upsertSig(pp1.PartialNewSignatures, PartialSignature{
		Index:     0,
		Signature: base64.StdEncoding.EncodeToString(ethSign(t, ethPrivs[0], payload)),
	})
	require.NoError(t, SavePartialProof(signed1Path, pp1))

	// === STEP C: sign-proof cosigner #2 (legacy-sub-1 + eth-sub-1) ===
	pp2, err := LoadPartialProof(payloadPath)
	require.NoError(t, err)
	pp2.PartialLegacySignatures = upsertSig(pp2.PartialLegacySignatures, PartialSignature{
		Index:     1,
		Signature: base64.StdEncoding.EncodeToString(cosmosSign(t, legacyPrivs[1], payload)),
	})
	pp2.PartialNewSignatures = upsertSig(pp2.PartialNewSignatures, PartialSignature{
		Index:     1,
		Signature: base64.StdEncoding.EncodeToString(ethSign(t, ethPrivs[1], payload)),
	})
	require.NoError(t, SavePartialProof(signed2Path, pp2))

	// === STEP D: combine-proof ===
	merged, err := LoadPartialProof(signed1Path)
	require.NoError(t, err)
	other, err := LoadPartialProof(signed2Path)
	require.NoError(t, err)
	require.NoError(t, AssertPartialProofsConsistent(merged, other))
	for _, p := range other.PartialLegacySignatures {
		merged.PartialLegacySignatures = upsertSig(merged.PartialLegacySignatures, p)
	}
	for _, p := range other.PartialNewSignatures {
		merged.PartialNewSignatures = upsertSig(merged.PartialNewSignatures, p)
	}
	require.Len(t, merged.PartialLegacySignatures, 2)
	require.Len(t, merged.PartialNewSignatures, 2)

	mergedPayload, err := hex.DecodeString(merged.PayloadHex)
	require.NoError(t, err)

	legacyProof, err := buildProofFromPartial(
		merged.Legacy, merged.PartialLegacySignatures, mergedPayload,
		sigverify.SubKeyTypeCosmosSecp256k1, "legacy", io.Discard,
	)
	require.NoError(t, err)
	newProof, err := buildProofFromPartial(
		merged.New, merged.PartialNewSignatures, mergedPayload,
		sigverify.SubKeyTypeEthSecp256k1, "new", io.Discard,
	)
	require.NoError(t, err)

	// === ASSERTIONS: shape of combine-proof output ===
	legacyMP := legacyProof.GetMultisig()
	require.NotNil(t, legacyMP, "legacy side must emit a MultisigProof")
	require.Equal(t, uint32(2), legacyMP.Threshold)
	require.Equal(t, []uint32{0, 1}, legacyMP.SignerIndices)
	require.Len(t, legacyMP.SubSignatures, 2)
	require.Equal(t, types.SigFormat_SIG_FORMAT_CLI, legacyMP.SigFormat)

	newMP := newProof.GetMultisig()
	require.NotNil(t, newMP, "new side must emit a MultisigProof")
	require.Equal(t, uint32(2), newMP.Threshold)
	require.Equal(t, []uint32{0, 1}, newMP.SignerIndices)
	require.Len(t, newMP.SubSignatures, 2)
	require.Equal(t, types.SigFormat_SIG_FORMAT_CLI, newMP.SigFormat)

	// === ASSERTIONS: server-side verifier accepts both sides ===
	require.NoError(t, keeper.VerifyMigrationProof(
		chainID, evmChainID, kind,
		legacyAddr, newAddr, legacyAddr,
		&legacyProof, sigverify.SubKeyTypeCosmosSecp256k1,
	), "keeper.VerifyMigrationProof must accept the legacy-side proof")

	require.NoError(t, keeper.VerifyMigrationProof(
		chainID, evmChainID, kind,
		legacyAddr, newAddr, newAddr,
		&newProof, sigverify.SubKeyTypeEthSecp256k1,
	), "keeper.VerifyMigrationProof must accept the new-side proof")

	// === ASSERTION: the assembled MsgClaimLegacyAccount passes ValidateBasic ===
	msg := &types.MsgClaimLegacyAccount{
		LegacyAddress: legacyAddr.String(),
		NewAddress:    newAddr.String(),
		LegacyProof:   legacyProof,
		NewProof:      newProof,
	}
	require.NoError(t, msg.ValidateBasic())
}

// ---------- buildMigrationProofs — cross-side intersection ----------

// multisigTestFixture creates fully-wired legacy (Cosmos secp256k1) and new
// (eth_secp256k1) 2-of-3 multisig sub-keys along with the canonical payload.
// Returns builders that can selectively produce signed partials at specific
// indices — used by the intersection tests below to fabricate asymmetric
// valid-index sets across the two sides.
func multisigTestFixture(t *testing.T) (
	legacySide, newSide *SideSpec,
	payload []byte,
	signLegacy func(idx uint32) PartialSignature,
	signNew func(idx uint32) PartialSignature,
) {
	t.Helper()
	const (
		chainID    = "lumera-test-1"
		evmChainID = uint64(76857769)
		kind       = "claim"
	)

	legacyPrivs := []*secp256k1.PrivKey{
		secp256k1.GenPrivKey(), secp256k1.GenPrivKey(), secp256k1.GenPrivKey(),
	}
	legacyPubs := make([]cryptotypes.PubKey, 3)
	legacyB64 := make([]string, 3)
	for i, p := range legacyPrivs {
		legacyPubs[i] = p.PubKey()
		legacyB64[i] = base64.StdEncoding.EncodeToString(p.PubKey().Bytes())
	}
	legacyAddr := sdk.AccAddress(kmultisig.NewLegacyAminoPubKey(2, legacyPubs).Address())

	ethPrivs := make([]*ethsecp256k1.PrivKey, 3)
	newPubs := make([]cryptotypes.PubKey, 3)
	newB64 := make([]string, 3)
	for i := range ethPrivs {
		p, err := ethsecp256k1.GenerateKey()
		require.NoError(t, err)
		ethPrivs[i] = p
		pk := p.PubKey().(*evmcryptotypes.PubKey)
		newPubs[i] = pk
		newB64[i] = base64.StdEncoding.EncodeToString(pk.Key)
	}
	newAddr := sdk.AccAddress(kmultisig.NewLegacyAminoPubKey(2, newPubs).Address())

	payloadStr := ComputePayload(chainID, evmChainID, kind, legacyAddr.String(), newAddr.String())
	payload = []byte(payloadStr)

	legacySide = &SideSpec{Threshold: 2, SubPubKeys: legacyB64, SigFormat: "SIG_FORMAT_CLI"}
	newSide = &SideSpec{Threshold: 2, SubPubKeys: newB64, SigFormat: "SIG_FORMAT_CLI"}

	signLegacy = func(idx uint32) PartialSignature {
		return PartialSignature{
			Index:     idx,
			Signature: base64.StdEncoding.EncodeToString(cosmosSign(t, legacyPrivs[idx], payload)),
		}
	}
	signNew = func(idx uint32) PartialSignature {
		return PartialSignature{
			Index:     idx,
			Signature: base64.StdEncoding.EncodeToString(ethSign(t, ethPrivs[idx], payload)),
		}
	}
	return
}

// TestBuildMigrationProofs_IntersectsIndicesAcrossSides proves that when
// co-signers contribute asymmetric valid signatures (legacy at [0,1,2], new
// at [1,2]), the assembled proofs share the intersected indices [1,2] on BOTH
// sides rather than each side selecting its own first K valid indices (which
// would give legacy=[0,1] vs new=[1,2] and fail the consensus mirror-source
// check). This is what lets operators produce a tx that passes ValidateBasic.
func TestBuildMigrationProofs_IntersectsIndicesAcrossSides(t *testing.T) {
	legacySide, newSide, payload, signLegacy, signNew := multisigTestFixture(t)
	pp := &PartialProof{
		Version:                 partialProofVersion,
		Kind:                    migrationProofKindClaim,
		Legacy:                  legacySide,
		New:                     newSide,
		PartialLegacySignatures: []PartialSignature{signLegacy(0), signLegacy(1), signLegacy(2)},
		PartialNewSignatures:    []PartialSignature{signNew(1), signNew(2)},
	}

	legacyProof, newProof, err := buildMigrationProofs(io.Discard, pp, payload)
	require.NoError(t, err)
	lm := legacyProof.GetMultisig()
	nm := newProof.GetMultisig()
	require.NotNil(t, lm)
	require.NotNil(t, nm)
	require.Equal(t, []uint32{1, 2}, lm.SignerIndices, "legacy side must use intersected indices")
	require.Equal(t, []uint32{1, 2}, nm.SignerIndices, "new side must use the SAME intersected indices")
	require.Len(t, lm.SubSignatures, 2)
	require.Len(t, nm.SubSignatures, 2)

	// Final sanity: the assembled pair satisfies the consensus mirror-source rule.
	require.NoError(t, types.ValidateProofPair(&legacyProof, &newProof))
}

// TestBuildMigrationProofs_IntersectionBelowThreshold proves that if the set
// of indices with valid signatures on BOTH sides is smaller than K, the
// dispatcher errors out — rather than silently using different K-subsets
// per side to produce an invalid tx.
func TestBuildMigrationProofs_IntersectionBelowThreshold(t *testing.T) {
	legacySide, newSide, payload, signLegacy, signNew := multisigTestFixture(t)
	// Legacy signed at [0,1]; new signed at [2]. Intersection is empty.
	pp := &PartialProof{
		Version:                 partialProofVersion,
		Kind:                    migrationProofKindClaim,
		Legacy:                  legacySide,
		New:                     newSide,
		PartialLegacySignatures: []PartialSignature{signLegacy(0), signLegacy(1)},
		PartialNewSignatures:    []PartialSignature{signNew(2)},
	}

	_, _, err := buildMigrationProofs(io.Discard, pp, payload)
	require.Error(t, err)
	require.Contains(t, err.Error(), "signed on BOTH sides at matching indices")
}

// TestBuildMigrationProofs_IntersectionHasOneButNeedsK pins the specific
// regression shape a reviewer flagged: legacy valid at {0,1}, new valid at
// {0,2}, K=2. Intersection is {0} — non-empty but below threshold. Catches
// an off-by-one where len(intersection) > 0 is mistakenly treated as
// "enough," producing a tx that would fail mirror-source at submit time.
func TestBuildMigrationProofs_IntersectionHasOneButNeedsK(t *testing.T) {
	legacySide, newSide, payload, signLegacy, signNew := multisigTestFixture(t)
	pp := &PartialProof{
		Version:                 partialProofVersion,
		Kind:                    migrationProofKindClaim,
		Legacy:                  legacySide,
		New:                     newSide,
		PartialLegacySignatures: []PartialSignature{signLegacy(0), signLegacy(1)},
		PartialNewSignatures:    []PartialSignature{signNew(0), signNew(2)},
	}

	_, _, err := buildMigrationProofs(io.Discard, pp, payload)
	require.Error(t, err)
	require.Contains(t, err.Error(), "signed on BOTH sides at matching indices")
	require.Contains(t, err.Error(), "have 1")
}

// TestValidateSideSpec_RejectsDuplicateSubKeys pins the CLI-layer duplicate
// check. A legacy multisig with duplicate on-chain sub-keys (SDK construction
// permits this) should be rejected by validateSideSpec before a proof.json is
// written or loaded — not just at ValidateBasic/submit time, since the
// documented raw-CLI flow doesn't require MigrationEstimate.
func TestValidateSideSpec_RejectsDuplicateSubKeys(t *testing.T) {
	// Positions 0 and 2 share the same base64-encoded sub-key.
	shared := base64.StdEncoding.EncodeToString(secp256k1.GenPrivKey().PubKey().Bytes())
	other := base64.StdEncoding.EncodeToString(secp256k1.GenPrivKey().PubKey().Bytes())
	side := &SideSpec{
		Threshold:  2,
		SubPubKeys: []string{shared, other, shared},
		SigFormat:  "SIG_FORMAT_CLI",
	}
	err := validateSideSpec("legacy", side)
	require.Error(t, err)
	require.Contains(t, err.Error(), "sub_pub_keys[2] duplicates sub_pub_keys[0]")
}

// TestBuildMigrationProofs_RejectsMixedShape covers the final dispatcher
// branch: a single-key legacy paired with a multisig new (or vice versa) is
// caught here before reaching ValidateBasic, so combine-proof never writes
// a tx.json that's guaranteed to fail mirror-source.
func TestBuildMigrationProofs_RejectsMixedShape(t *testing.T) {
	legacySide, newSide, payload, signLegacy, _ := multisigTestFixture(t)
	// Collapse the legacy side to single-key while keeping new as multisig.
	singleLegacy := &SideSpec{
		PubKey:    legacySide.SubPubKeys[0],
		SigFormat: "SIG_FORMAT_CLI",
	}
	pp := &PartialProof{
		Version:                 partialProofVersion,
		Kind:                    migrationProofKindClaim,
		Legacy:                  singleLegacy,
		New:                     newSide,
		PartialLegacySignatures: []PartialSignature{signLegacy(0)},
		PartialNewSignatures:    nil,
	}

	_, _, err := buildMigrationProofs(io.Discard, pp, payload)
	require.Error(t, err)
	require.Contains(t, err.Error(), "sides must match shape")
}

// TestCmdSignProof_RegistersNewKeyFlag is a regression test for a bug where
// cmdSignProof read --new-key from the flag set but never registered it. The
// command's body referenced flagNewKey at parse time and in two error
// messages, so the flag *looked* supported, but cobra would reject any
// invocation as "unknown flag: --new-key" before the body ever ran. The
// failure surfaced in devnet as:
//
//	sign-proof --from <legacy-sub> --new-key <new-sub>: unknown flag: --new-key
//
// Asserting that every flag the command reads is also registered keeps this
// class of bug from recurring silently.
func TestCmdSignProof_RegistersNewKeyFlag(t *testing.T) {
	cmd := cmdSignProof()
	for _, name := range []string{flagNewKey, flagOut} {
		require.NotNilf(t, cmd.Flags().Lookup(name),
			"flag --%s is read by cmdSignProof but never registered", name)
	}
}
