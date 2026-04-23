package cli

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ethsecp256k1 "github.com/cosmos/evm/crypto/ethsecp256k1"
	evmcryptotypes "github.com/cosmos/evm/crypto/ethsecp256k1"

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
