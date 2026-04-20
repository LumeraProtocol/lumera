package keeper_test

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sort"
	"testing"

	kmultisig "github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	evmcryptotypes "github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/stretchr/testify/require"

	lcfg "github.com/LumeraProtocol/lumera/config"
	"github.com/LumeraProtocol/lumera/x/evmigration/keeper"
	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// signMigrationMessage creates a valid legacy signature over the canonical
// migration payload for account-claim messages.
func signMigrationMessage(t *testing.T, privKey *secp256k1.PrivKey, legacyAddr, newAddr sdk.AccAddress) []byte {
	t.Helper()
	return signLegacyMigrationMessage(t, keeperClaimKind, privKey, legacyAddr, newAddr)
}

func signLegacyMigrationMessage(t *testing.T, kind string, privKey *secp256k1.PrivKey, legacyAddr, newAddr sdk.AccAddress) []byte {
	t.Helper()
	msg := fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s", testChainID, lcfg.EVMChainID, kind, legacyAddr.String(), newAddr.String())
	hash := sha256.Sum256([]byte(msg))
	sig, err := privKey.Sign(hash[:])
	require.NoError(t, err)
	return sig
}

func signNewMigrationMessage(t *testing.T, kind string, privKey *evmcryptotypes.PrivKey, legacyAddr, newAddr sdk.AccAddress) []byte {
	t.Helper()
	msg := fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s", testChainID, lcfg.EVMChainID, kind, legacyAddr.String(), newAddr.String())
	sig, err := privKey.Sign([]byte(msg))
	require.NoError(t, err)
	if len(sig) == 65 {
		return sig[:64]
	}
	return sig
}

func testNewMigrationAccount(t *testing.T) (*evmcryptotypes.PrivKey, sdk.AccAddress) {
	t.Helper()
	privKey, err := evmcryptotypes.GenerateKey()
	require.NoError(t, err)
	return privKey, sdk.AccAddress(privKey.PubKey().Address())
}

const (
	keeperClaimKind     = "claim"
	keeperValidatorKind = "validator"
	testChainID         = "lumera-test-1"
)

// TestVerifyLegacySignature_Valid verifies that a correctly signed migration
// message passes verification.
func TestVerifyLegacySignature_Valid(t *testing.T) {
	privKey := secp256k1.GenPrivKey()
	pubKey := privKey.PubKey().(*secp256k1.PubKey)
	legacyAddr := sdk.AccAddress(pubKey.Address())
	_, newAddr := testNewMigrationAccount(t)

	sig := signMigrationMessage(t, privKey, legacyAddr, newAddr)

	proof := &types.LegacyProof{Proof: &types.LegacyProof_Single{Single: &types.SingleKeyProof{
		PubKey: pubKey.Key, Signature: sig, SigFormat: types.SigFormat_SIG_FORMAT_CLI,
	}}}
	err := keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, proof)
	require.NoError(t, err)
}

// TestVerifyLegacySignature_InvalidPubKeySize rejects public keys that are
// not exactly 33 bytes (compressed secp256k1).
func TestVerifyLegacySignature_InvalidPubKeySize(t *testing.T) {
	legacyAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	_, newAddr := testNewMigrationAccount(t)

	// Too short.
	proof := &types.LegacyProof{Proof: &types.LegacyProof_Single{Single: &types.SingleKeyProof{
		PubKey: []byte{0x01, 0x02}, Signature: []byte{0x00}, SigFormat: types.SigFormat_SIG_FORMAT_CLI,
	}}}
	err := keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, proof)
	require.ErrorIs(t, err, types.ErrInvalidLegacyPubKey)

	// Too long.
	proof = &types.LegacyProof{Proof: &types.LegacyProof_Single{Single: &types.SingleKeyProof{
		PubKey: make([]byte, 65), Signature: []byte{0x00}, SigFormat: types.SigFormat_SIG_FORMAT_CLI,
	}}}
	err = keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, proof)
	require.ErrorIs(t, err, types.ErrInvalidLegacyPubKey)
}

// TestVerifyLegacySignature_PubKeyAddressMismatch rejects when the public key
// does not derive to the claimed legacy address.
func TestVerifyLegacySignature_PubKeyAddressMismatch(t *testing.T) {
	privKey := secp256k1.GenPrivKey()
	pubKey := privKey.PubKey().(*secp256k1.PubKey)

	// Use a different address as legacy (not derived from this pubkey).
	wrongLegacyAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	_, newAddr := testNewMigrationAccount(t)

	proof := &types.LegacyProof{Proof: &types.LegacyProof_Single{Single: &types.SingleKeyProof{
		PubKey: pubKey.Key, Signature: []byte{0x00}, SigFormat: types.SigFormat_SIG_FORMAT_CLI,
	}}}
	err := keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, keeperClaimKind, wrongLegacyAddr, newAddr, proof)
	require.ErrorIs(t, err, types.ErrPubKeyAddressMismatch)
}

// TestVerifyLegacySignature_InvalidSignature rejects a signature produced by
// a different private key than the one matching the public key.
func TestVerifyLegacySignature_InvalidSignature(t *testing.T) {
	privKey := secp256k1.GenPrivKey()
	pubKey := privKey.PubKey().(*secp256k1.PubKey)
	legacyAddr := sdk.AccAddress(pubKey.Address())
	_, newAddr := testNewMigrationAccount(t)

	// Sign with a different key.
	otherPrivKey := secp256k1.GenPrivKey()
	badSig := signMigrationMessage(t, otherPrivKey, legacyAddr, newAddr)

	proof := &types.LegacyProof{Proof: &types.LegacyProof_Single{Single: &types.SingleKeyProof{
		PubKey: pubKey.Key, Signature: badSig, SigFormat: types.SigFormat_SIG_FORMAT_CLI,
	}}}
	err := keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, proof)
	require.ErrorIs(t, err, types.ErrInvalidLegacySignature)
}

// TestVerifyLegacySignature_WrongMessage rejects a valid signature that was
// produced over a different new address than the one being verified.
func TestVerifyLegacySignature_WrongMessage(t *testing.T) {
	privKey := secp256k1.GenPrivKey()
	pubKey := privKey.PubKey().(*secp256k1.PubKey)
	legacyAddr := sdk.AccAddress(pubKey.Address())
	_, newAddr := testNewMigrationAccount(t)

	// Sign over a different new address.
	_, otherNewAddr := testNewMigrationAccount(t)
	sig := signMigrationMessage(t, privKey, legacyAddr, otherNewAddr)

	proof := &types.LegacyProof{Proof: &types.LegacyProof_Single{Single: &types.SingleKeyProof{
		PubKey: pubKey.Key, Signature: sig, SigFormat: types.SigFormat_SIG_FORMAT_CLI,
	}}}
	err := keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, proof)
	require.ErrorIs(t, err, types.ErrInvalidLegacySignature)
}

// TestVerifyLegacySignature_EmptySignature rejects a nil/empty signature.
func TestVerifyLegacySignature_EmptySignature(t *testing.T) {
	privKey := secp256k1.GenPrivKey()
	pubKey := privKey.PubKey().(*secp256k1.PubKey)
	legacyAddr := sdk.AccAddress(pubKey.Address())
	_, newAddr := testNewMigrationAccount(t)

	proof := &types.LegacyProof{Proof: &types.LegacyProof_Single{Single: &types.SingleKeyProof{
		PubKey: pubKey.Key, Signature: nil, SigFormat: types.SigFormat_SIG_FORMAT_CLI,
	}}}
	err := keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, proof)
	require.ErrorIs(t, err, types.ErrInvalidLegacySignature)
}

// TestVerifyNewSignature_Valid verifies that a correctly signed destination
// proof passes verification.
func TestVerifyNewSignature_Valid(t *testing.T) {
	legacyAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	privKey, newAddr := testNewMigrationAccount(t)
	sig := signNewMigrationMessage(t, keeperClaimKind, privKey, legacyAddr, newAddr)

	err := keeper.VerifyNewSignature(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, sig)
	require.NoError(t, err)
}

// TestVerifyNewSignature_AddressMismatch rejects when the recovered signer does
// not derive to the claimed destination address.
func TestVerifyNewSignature_AddressMismatch(t *testing.T) {
	legacyAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	wrongPrivKey, _ := testNewMigrationAccount(t)
	_, newAddr := testNewMigrationAccount(t)
	sig := signNewMigrationMessage(t, keeperClaimKind, wrongPrivKey, legacyAddr, newAddr)

	err := keeper.VerifyNewSignature(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, sig)
	require.ErrorIs(t, err, types.ErrNewPubKeyAddressMismatch)
}

// TestVerifyNewSignature_InvalidSignature rejects malformed signatures that
// cannot recover any signer.
func TestVerifyNewSignature_InvalidSignature(t *testing.T) {
	legacyAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	_, newAddr := testNewMigrationAccount(t)
	badSig := []byte{0x01}

	err := keeper.VerifyNewSignature(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, badSig)
	require.ErrorIs(t, err, types.ErrInvalidNewSignature)
}

// --- EIP-191 personal_sign tests (new key, wallet path) ---

// testMigrationPayload reconstructs the canonical payload for test signing.
func testMigrationPayload(kind string, legacyAddr, newAddr sdk.AccAddress) []byte {
	return []byte(fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s", testChainID, lcfg.EVMChainID, kind, legacyAddr.String(), newAddr.String()))
}

// signNewMigrationEIP191 simulates what a wallet's personal_sign does:
// sign(Keccak256("\x19Ethereum Signed Message:\n" + len(payload) + payload))
// eth_secp256k1.Sign(msg) internally does Keccak256(msg) when len(msg) != 32,
// so passing the EIP-191-prefixed payload produces the correct digest.
func signNewMigrationEIP191(t *testing.T, kind string, privKey *evmcryptotypes.PrivKey, legacyAddr, newAddr sdk.AccAddress) []byte {
	t.Helper()
	payload := testMigrationPayload(kind, legacyAddr, newAddr)
	prefix := fmt.Appendf(nil, "\x19Ethereum Signed Message:\n%d", len(payload))
	eip191Msg := append(prefix, payload...)
	sig, err := privKey.Sign(eip191Msg)
	require.NoError(t, err)
	return sig
}

// TestVerifyNewSignature_EIP191 verifies that an EIP-191 personal_sign
// signature (as produced by Keplr/Leap Ethereum provider) passes verification.
func TestVerifyNewSignature_EIP191(t *testing.T) {
	legacyAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	privKey, newAddr := testNewMigrationAccount(t)
	sig := signNewMigrationEIP191(t, keeperClaimKind, privKey, legacyAddr, newAddr)

	err := keeper.VerifyNewSignature(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, sig)
	require.NoError(t, err)
}

// TestVerifyNewSignature_EIP191_Validator verifies the EIP-191 path works
// for the "validator" kind as well.
func TestVerifyNewSignature_EIP191_Validator(t *testing.T) {
	legacyAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	privKey, newAddr := testNewMigrationAccount(t)
	sig := signNewMigrationEIP191(t, keeperValidatorKind, privKey, legacyAddr, newAddr)

	err := keeper.VerifyNewSignature(testChainID, lcfg.EVMChainID, keeperValidatorKind, legacyAddr, newAddr, sig)
	require.NoError(t, err)
}

// TestVerifyNewSignature_EIP191_WrongKey rejects an EIP-191 signature from the
// wrong destination private key.
func TestVerifyNewSignature_EIP191_WrongKey(t *testing.T) {
	legacyAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	otherPrivKey, _ := testNewMigrationAccount(t)
	_, newAddr := testNewMigrationAccount(t)
	badSig := signNewMigrationEIP191(t, keeperClaimKind, otherPrivKey, legacyAddr, newAddr)

	err := keeper.VerifyNewSignature(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, badSig)
	require.ErrorIs(t, err, types.ErrNewPubKeyAddressMismatch)
}

// --- ADR-036 signArbitrary tests (legacy key, wallet path) ---

// testADR036SignDoc builds the canonical ADR-036 sign doc (same logic as
// keeper.adr036SignDoc, independently implemented for test verification).
func testADR036SignDoc(signer string, data []byte) []byte {
	return []byte(fmt.Sprintf(
		`{"account_number":"0","chain_id":"","fee":{"amount":[],"gas":"0"},`+
			`"memo":"","msgs":[{"type":"sign/MsgSignData","value":`+
			`{"data":"%s","signer":"%s"}}],"sequence":"0"}`,
		base64.StdEncoding.EncodeToString(data), signer,
	))
}

// signLegacyMigrationADR036 simulates what Keplr's signArbitrary does:
// Sign(adr036_doc) — the SDK's secp256k1.Sign internally does SHA256(adr036_doc).
func signLegacyMigrationADR036(t *testing.T, kind string, privKey *secp256k1.PrivKey, legacyAddr, newAddr sdk.AccAddress) []byte {
	t.Helper()
	payload := testMigrationPayload(kind, legacyAddr, newAddr)
	adr036Doc := testADR036SignDoc(legacyAddr.String(), payload)
	// secp256k1.Sign(msg) internally does SHA256(msg) then ECDSA signs.
	sig, err := privKey.Sign(adr036Doc)
	require.NoError(t, err)
	return sig
}

// TestVerifyLegacySignature_ADR036 verifies that an ADR-036 signArbitrary
// signature (as produced by Keplr/Leap) passes verification.
func TestVerifyLegacySignature_ADR036(t *testing.T) {
	privKey := secp256k1.GenPrivKey()
	pubKey := privKey.PubKey().(*secp256k1.PubKey)
	legacyAddr := sdk.AccAddress(pubKey.Address())
	_, newAddr := testNewMigrationAccount(t)

	sig := signLegacyMigrationADR036(t, keeperClaimKind, privKey, legacyAddr, newAddr)

	proof := &types.LegacyProof{Proof: &types.LegacyProof_Single{Single: &types.SingleKeyProof{
		PubKey: pubKey.Key, Signature: sig, SigFormat: types.SigFormat_SIG_FORMAT_ADR036,
	}}}
	err := keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, proof)
	require.NoError(t, err)
}

// TestVerifyLegacySignature_ADR036_Validator verifies the ADR-036 path works
// for the "validator" kind.
func TestVerifyLegacySignature_ADR036_Validator(t *testing.T) {
	privKey := secp256k1.GenPrivKey()
	pubKey := privKey.PubKey().(*secp256k1.PubKey)
	legacyAddr := sdk.AccAddress(pubKey.Address())
	_, newAddr := testNewMigrationAccount(t)

	sig := signLegacyMigrationADR036(t, keeperValidatorKind, privKey, legacyAddr, newAddr)

	proof := &types.LegacyProof{Proof: &types.LegacyProof_Single{Single: &types.SingleKeyProof{
		PubKey: pubKey.Key, Signature: sig, SigFormat: types.SigFormat_SIG_FORMAT_ADR036,
	}}}
	err := keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, keeperValidatorKind, legacyAddr, newAddr, proof)
	require.NoError(t, err)
}

// TestVerifyLegacySignature_ADR036_WrongKey rejects an ADR-036 signature from
// the wrong private key.
func TestVerifyLegacySignature_ADR036_WrongKey(t *testing.T) {
	privKey := secp256k1.GenPrivKey()
	pubKey := privKey.PubKey().(*secp256k1.PubKey)
	legacyAddr := sdk.AccAddress(pubKey.Address())
	_, newAddr := testNewMigrationAccount(t)

	otherPrivKey := secp256k1.GenPrivKey()
	badSig := signLegacyMigrationADR036(t, keeperClaimKind, otherPrivKey, legacyAddr, newAddr)

	proof := &types.LegacyProof{Proof: &types.LegacyProof_Single{Single: &types.SingleKeyProof{
		PubKey: pubKey.Key, Signature: badSig, SigFormat: types.SigFormat_SIG_FORMAT_ADR036,
	}}}
	err := keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, proof)
	require.ErrorIs(t, err, types.ErrInvalidLegacySignature)
}

// TestVerifyLegacySignature_ADR036_WrongSigner rejects an ADR-036 signature
// where the signer field doesn't match (different address in the sign doc).
func TestVerifyLegacySignature_ADR036_WrongSigner(t *testing.T) {
	privKey := secp256k1.GenPrivKey()
	pubKey := privKey.PubKey().(*secp256k1.PubKey)
	legacyAddr := sdk.AccAddress(pubKey.Address())
	_, newAddr := testNewMigrationAccount(t)

	// Build ADR-036 doc with wrong signer address.
	payload := testMigrationPayload(keeperClaimKind, legacyAddr, newAddr)
	wrongSignerDoc := testADR036SignDoc("lumera1wrongsigneraddress", payload)
	sig, err := privKey.Sign(wrongSignerDoc)
	require.NoError(t, err)

	// The verifier builds the ADR-036 doc using legacyAddr, so a doc signed
	// with a different signer produces a different digest → verification fails.
	proof := &types.LegacyProof{Proof: &types.LegacyProof_Single{Single: &types.SingleKeyProof{
		PubKey: pubKey.Key, Signature: sig, SigFormat: types.SigFormat_SIG_FORMAT_ADR036,
	}}}
	err = keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, proof)
	require.ErrorIs(t, err, types.ErrInvalidLegacySignature)
}

// TestVerifyLegacySignature_ADR036_DocFormat verifies that the test's ADR-036
// doc matches the expected canonical form byte-for-byte.
func TestVerifyLegacySignature_ADR036_DocFormat(t *testing.T) {
	data := []byte("test-payload")
	signer := "lumera1abc123"
	doc := testADR036SignDoc(signer, data)

	// Verify JSON structure: fields alphabetically sorted, no whitespace.
	expected := fmt.Sprintf(
		`{"account_number":"0","chain_id":"","fee":{"amount":[],"gas":"0"},`+
			`"memo":"","msgs":[{"type":"sign/MsgSignData","value":`+
			`{"data":"%s","signer":"%s"}}],"sequence":"0"}`,
		base64.StdEncoding.EncodeToString(data), signer,
	)
	require.Equal(t, expected, string(doc))
}

// TestVerifyNewSignature_EIP191_PayloadFormat verifies that the EIP-191 prefix
// is constructed correctly for a known payload.
func TestVerifyNewSignature_EIP191_PayloadFormat(t *testing.T) {
	msg := []byte("hello")
	prefix := fmt.Appendf(nil, "\x19Ethereum Signed Message:\n%d", len(msg))
	eip191 := append(prefix, msg...)
	require.Equal(t, "\x19Ethereum Signed Message:\n5hello", string(eip191))
}

// TestVerifyLegacySignature_BothPathsRejectGarbage verifies that neither the
// raw nor ADR-036 path accepts a completely garbage signature.
func TestVerifyLegacySignature_BothPathsRejectGarbage(t *testing.T) {
	privKey := secp256k1.GenPrivKey()
	pubKey := privKey.PubKey().(*secp256k1.PubKey)
	legacyAddr := sdk.AccAddress(pubKey.Address())
	_, newAddr := testNewMigrationAccount(t)

	// A valid-length but wrong signature (64 bytes of zeros).
	garbageSig := make([]byte, 64)

	proof := &types.LegacyProof{Proof: &types.LegacyProof_Single{Single: &types.SingleKeyProof{
		PubKey: pubKey.Key, Signature: garbageSig, SigFormat: types.SigFormat_SIG_FORMAT_CLI,
	}}}
	err := keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, proof)
	require.ErrorIs(t, err, types.ErrInvalidLegacySignature)
}

// TestVerifyNewSignature_BothPathsRejectGarbage verifies that neither the
// raw nor EIP-191 path accepts a completely garbage signature.
func TestVerifyNewSignature_BothPathsRejectGarbage(t *testing.T) {
	legacyAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	_, newAddr := testNewMigrationAccount(t)

	garbageSig := []byte{0x01, 0x02, 0x03}

	err := keeper.VerifyNewSignature(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, garbageSig)
	require.ErrorIs(t, err, types.ErrInvalidNewSignature)
}

// TestVerifyLegacySignature_ChainIDMismatch verifies that a valid signature
// signed with a different chain ID is rejected.
func TestVerifyLegacySignature_ChainIDMismatch(t *testing.T) {
	privKey := secp256k1.GenPrivKey()
	pubKey := privKey.PubKey().(*secp256k1.PubKey)
	legacyAddr := sdk.AccAddress(pubKey.Address())
	_, newAddr := testNewMigrationAccount(t)

	// Sign with a different chain ID.
	wrongChainID := "lumera-wrong-99"
	msg := fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s", wrongChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr.String(), newAddr.String())
	hash := sha256.Sum256([]byte(msg))
	sig, err := privKey.Sign(hash[:])
	require.NoError(t, err)

	// Verify against the correct chain ID — should fail.
	proof := &types.LegacyProof{Proof: &types.LegacyProof_Single{Single: &types.SingleKeyProof{
		PubKey: pubKey.Key, Signature: sig, SigFormat: types.SigFormat_SIG_FORMAT_CLI,
	}}}
	err = keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, proof)
	require.ErrorIs(t, err, types.ErrInvalidLegacySignature)
}

// TestVerifyNewSignature_ChainIDMismatch verifies that a valid new-key
// signature signed with a different chain ID is rejected and the error
// hints at the chain ID.
func TestVerifyNewSignature_ChainIDMismatch(t *testing.T) {
	legacyAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	newPrivKey, newAddr := testNewMigrationAccount(t)

	// Sign with a different chain ID.
	wrongChainID := "lumera-wrong-99"
	payload := []byte(fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s", wrongChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr.String(), newAddr.String()))
	sig, err := newPrivKey.Sign(payload)
	require.NoError(t, err)

	// Verify against the correct chain ID — should fail.
	err = keeper.VerifyNewSignature(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, sig)
	require.Error(t, err)
	require.ErrorContains(t, err, testChainID, "error must include the expected chain ID to help diagnose mismatches")
}

// makeMultisigAccount creates N secp256k1 sub-keys and the resulting
// LegacyAminoPubKey for a K-of-N multisig.
func makeMultisigAccount(t *testing.T, threshold, n int) (*kmultisig.LegacyAminoPubKey, []*secp256k1.PrivKey, sdk.AccAddress) {
	t.Helper()
	privKeys := make([]*secp256k1.PrivKey, n)
	pubKeys := make([]cryptotypes.PubKey, n)
	for i := 0; i < n; i++ {
		privKeys[i] = secp256k1.GenPrivKey()
		pubKeys[i] = privKeys[i].PubKey()
	}
	multiPK := kmultisig.NewLegacyAminoPubKey(threshold, pubKeys)
	addr := sdk.AccAddress(multiPK.Address())
	return multiPK, privKeys, addr
}

// buildMultisigProof builds a valid MultisigProof signed by the K sub-keys at
// signerIdxs. format selects CLI (SHA256) or ADR-036 envelope.
func buildMultisigProof(t *testing.T, kind string, multiPK *kmultisig.LegacyAminoPubKey, privKeys []*secp256k1.PrivKey, signerIdxs []int, legacyAddr, newAddr sdk.AccAddress, format types.SigFormat) *types.LegacyProof {
	t.Helper()
	payload := fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s",
		testChainID, lcfg.EVMChainID, kind, legacyAddr.String(), newAddr.String())

	sort.Ints(signerIdxs)
	subPubKeys := make([][]byte, len(multiPK.GetPubKeys()))
	for i, pk := range multiPK.GetPubKeys() {
		subPubKeys[i] = pk.Bytes()
	}

	indices := make([]uint32, len(signerIdxs))
	sigs := make([][]byte, len(signerIdxs))
	for i, idx := range signerIdxs {
		indices[i] = uint32(idx)
		if format == types.SigFormat_SIG_FORMAT_ADR036 {
			signerAddr := sdk.AccAddress(privKeys[idx].PubKey().Address()).String()
			doc := []byte(fmt.Sprintf(`{"account_number":"0","chain_id":"","fee":{"amount":[],"gas":"0"},"memo":"","msgs":[{"type":"sign/MsgSignData","value":{"data":"%s","signer":"%s"}}],"sequence":"0"}`,
				base64.StdEncoding.EncodeToString([]byte(payload)), signerAddr))
			sig, err := privKeys[idx].Sign(doc)
			require.NoError(t, err)
			sigs[i] = sig
			continue
		}
		hash := sha256.Sum256([]byte(payload))
		sig, err := privKeys[idx].Sign(hash[:])
		require.NoError(t, err)
		sigs[i] = sig
	}
	return &types.LegacyProof{Proof: &types.LegacyProof_Multisig{Multisig: &types.MultisigProof{
		Threshold:     uint32(multiPK.Threshold),
		SubPubKeys:    subPubKeys,
		SignerIndices: indices,
		SubSignatures: sigs,
		SigFormat:     format,
	}}}
}

func TestVerifyLegacyProof_Multisig_Valid_CLI(t *testing.T) {
	multiPK, privs, legacyAddr := makeMultisigAccount(t, 2, 3)
	_, newAddr := testNewMigrationAccount(t)
	proof := buildMultisigProof(t, keeperClaimKind, multiPK, privs, []int{0, 2}, legacyAddr, newAddr, types.SigFormat_SIG_FORMAT_CLI)
	require.NoError(t, proof.ValidateBasic())
	require.NoError(t, keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, proof))
}

func TestVerifyLegacyProof_Multisig_Valid_ADR036(t *testing.T) {
	multiPK, privs, legacyAddr := makeMultisigAccount(t, 2, 3)
	_, newAddr := testNewMigrationAccount(t)
	proof := buildMultisigProof(t, keeperClaimKind, multiPK, privs, []int{1, 2}, legacyAddr, newAddr, types.SigFormat_SIG_FORMAT_ADR036)
	require.NoError(t, proof.ValidateBasic())
	require.NoError(t, keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, proof))
}

func TestVerifyLegacyProof_Multisig_1of1(t *testing.T) {
	multiPK, privs, legacyAddr := makeMultisigAccount(t, 1, 1)
	_, newAddr := testNewMigrationAccount(t)
	proof := buildMultisigProof(t, keeperClaimKind, multiPK, privs, []int{0}, legacyAddr, newAddr, types.SigFormat_SIG_FORMAT_CLI)
	require.NoError(t, keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, proof))
}

func TestVerifyLegacyProof_Multisig_WrongAddress(t *testing.T) {
	multiPK, privs, legacyAddr := makeMultisigAccount(t, 2, 3)
	_, newAddr := testNewMigrationAccount(t)
	proof := buildMultisigProof(t, keeperClaimKind, multiPK, privs, []int{0, 1}, legacyAddr, newAddr, types.SigFormat_SIG_FORMAT_CLI)

	bogusAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	err := keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, keeperClaimKind, bogusAddr, newAddr, proof)
	require.ErrorContains(t, err, "multisig pubkey derives to")
}

func TestVerifyLegacyProof_Multisig_InvalidSubSig(t *testing.T) {
	multiPK, privs, legacyAddr := makeMultisigAccount(t, 2, 3)
	_, newAddr := testNewMigrationAccount(t)
	proof := buildMultisigProof(t, keeperClaimKind, multiPK, privs, []int{0, 1}, legacyAddr, newAddr, types.SigFormat_SIG_FORMAT_CLI)
	// Corrupt the second sub-signature.
	proof.GetMultisig().SubSignatures[1][0] ^= 0xFF
	err := keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, proof)
	require.ErrorContains(t, err, "sub-sig 1")
}

func TestVerifyLegacyProof_Multisig_LengthMismatchRejectedBeforeVerification(t *testing.T) {
	multiPK, _, legacyAddr := makeMultisigAccount(t, 2, 3)
	_, newAddr := testNewMigrationAccount(t)

	subPubKeys := make([][]byte, len(multiPK.GetPubKeys()))
	for i, pk := range multiPK.GetPubKeys() {
		subPubKeys[i] = pk.Bytes()
	}
	proof := &types.LegacyProof{Proof: &types.LegacyProof_Multisig{Multisig: &types.MultisigProof{
		Threshold:     2,
		SubPubKeys:    subPubKeys,
		SignerIndices: []uint32{0, 1},
		SubSignatures: [][]byte{make([]byte, 64)},
		SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
	}}}

	require.NotPanics(t, func() {
		err := keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, proof)
		require.ErrorContains(t, err, "sub_signatures length mismatch")
	})
}

func TestVerifyLegacyProof_Multisig_MaxBoundary(t *testing.T) {
	multiPK, privs, legacyAddr := makeMultisigAccount(t, 20, 20)
	_, newAddr := testNewMigrationAccount(t)
	signerIdxs := make([]int, 20)
	for i := range signerIdxs {
		signerIdxs[i] = i
	}
	proof := buildMultisigProof(t, keeperClaimKind, multiPK, privs, signerIdxs, legacyAddr, newAddr, types.SigFormat_SIG_FORMAT_CLI)
	require.NoError(t, proof.ValidateBasic())
	require.NoError(t, proof.ValidateParams(20))
	require.NoError(t, keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, proof))

	// Same proof should fail the param cap when MaxMultisigSubKeys=19.
	require.ErrorContains(t, proof.ValidateParams(19), "exceeds max 19")
}
