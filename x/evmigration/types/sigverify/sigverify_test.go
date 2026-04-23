package sigverify_test

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ethsecp256k1 "github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
	"github.com/LumeraProtocol/lumera/x/evmigration/types/sigverify"
)

// --- Cosmos secp256k1 ---

func TestVerifyCosmosSecp256k1_CLI(t *testing.T) {
	priv := secp256k1.GenPrivKey()
	pk := priv.PubKey().(*secp256k1.PubKey)
	payload := []byte("payload-bytes")
	hash := sha256.Sum256(payload)
	sig, err := priv.Sign(hash[:])
	require.NoError(t, err)
	require.NoError(t, sigverify.VerifyCosmosSecp256k1(pk, sdk.AccAddress(pk.Address()), payload, sig, types.SigFormat_SIG_FORMAT_CLI))
}

func TestVerifyCosmosSecp256k1_ADR036(t *testing.T) {
	priv := secp256k1.GenPrivKey()
	pk := priv.PubKey().(*secp256k1.PubKey)
	payload := []byte("payload-bytes")
	signerAddr := sdk.AccAddress(pk.Address())
	doc := sigverify.ADR036SignDoc(signerAddr.String(), payload)
	sig, err := priv.Sign(doc)
	require.NoError(t, err)
	require.NoError(t, sigverify.VerifyCosmosSecp256k1(pk, signerAddr, payload, sig, types.SigFormat_SIG_FORMAT_ADR036))
}

func TestVerifyCosmosSecp256k1_EIP191_Rejected(t *testing.T) {
	priv := secp256k1.GenPrivKey()
	pk := priv.PubKey().(*secp256k1.PubKey)
	err := sigverify.VerifyCosmosSecp256k1(pk, sdk.AccAddress(pk.Address()), []byte("x"), []byte("y"), types.SigFormat_SIG_FORMAT_EIP191)
	require.Error(t, err)
	require.ErrorContains(t, err, "EIP191 is not valid for Cosmos secp256k1")
}

// --- Eth secp256k1 ---

// Cosmos EVM v0.6.0's ethsecp256k1.PrivKey.Sign returns a 65-byte recoverable
// signature (R||S||V). That is the ONLY accepted wire length on the new side —
// ValidateBasic rejects 64-byte input upfront, and sigverify.VerifyEthSecp256k1
// returns an error on anything but 65.

func TestVerifyEthSecp256k1_CLI_65byte(t *testing.T) {
	priv, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	pk := priv.PubKey().(*ethsecp256k1.PubKey)
	payload := []byte("payload-bytes")
	sig, err := priv.Sign(payload)
	require.NoError(t, err)
	require.Equal(t, 65, len(sig), "SDK eth sig contract is 65 bytes (R||S||V); got %d", len(sig))
	require.NoError(t, sigverify.VerifyEthSecp256k1(pk, sdk.AccAddress(pk.Address()), payload, sig, types.SigFormat_SIG_FORMAT_CLI))
}

func TestVerifyEthSecp256k1_EIP191_65byte(t *testing.T) {
	priv, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	pk := priv.PubKey().(*ethsecp256k1.PubKey)
	payload := []byte("payload-bytes")
	wrapped := sigverify.EIP191PersonalSignPayload(payload)
	sig, err := priv.Sign(wrapped)
	require.NoError(t, err)
	require.Equal(t, 65, len(sig))
	require.NoError(t, sigverify.VerifyEthSecp256k1(pk, sdk.AccAddress(pk.Address()), payload, sig, types.SigFormat_SIG_FORMAT_EIP191))
}

func TestVerifyEthSecp256k1_ADR036_65byte(t *testing.T) {
	priv, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	pk := priv.PubKey().(*ethsecp256k1.PubKey)
	payload := []byte("payload-bytes")
	signerAddr := sdk.AccAddress(pk.Address())
	doc := sigverify.ADR036SignDoc(signerAddr.String(), payload)
	sig, err := priv.Sign(doc)
	require.NoError(t, err)
	require.Equal(t, 65, len(sig))
	require.NoError(t, sigverify.VerifyEthSecp256k1(pk, signerAddr, payload, sig, types.SigFormat_SIG_FORMAT_ADR036))
}

// TestVerifyEthSecp256k1_VByteIgnoredByVerifier asserts that the V byte is
// recovery metadata ignored by the verifier. Clobbering V with a wrong value
// must NOT invalidate an otherwise-valid R||S signature. This locks in the
// design commitment that verify-under-pubkey uses R||S only; a future refactor
// to ecrecover-and-compare would break this test.
func TestVerifyEthSecp256k1_VByteIgnoredByVerifier(t *testing.T) {
	priv, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	pk := priv.PubKey().(*ethsecp256k1.PubKey)
	payload := []byte("payload-bytes")
	sig, err := priv.Sign(payload)
	require.NoError(t, err)
	tampered := bytes.Clone(sig)
	tampered[64] ^= 0xff // flip V
	require.NoError(t, sigverify.VerifyEthSecp256k1(pk, sdk.AccAddress(pk.Address()), payload, tampered, types.SigFormat_SIG_FORMAT_CLI))
}

func TestVerifyEthSecp256k1_Reject64Byte(t *testing.T) {
	priv, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	pk := priv.PubKey().(*ethsecp256k1.PubKey)
	payload := []byte("payload-bytes")
	sig, err := priv.Sign(payload)
	require.NoError(t, err)
	sig64 := bytes.Clone(sig[:64])
	err = sigverify.VerifyEthSecp256k1(pk, sdk.AccAddress(pk.Address()), payload, sig64, types.SigFormat_SIG_FORMAT_CLI)
	require.Error(t, err)
	require.ErrorContains(t, err, "65 bytes")
}

func TestVerifyEthSecp256k1_RejectOtherLengths(t *testing.T) {
	priv, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	pk := priv.PubKey().(*ethsecp256k1.PubKey)
	for _, badLen := range []int{0, 63, 66, 128} {
		err := sigverify.VerifyEthSecp256k1(pk, sdk.AccAddress(pk.Address()), []byte("x"), make([]byte, badLen), types.SigFormat_SIG_FORMAT_CLI)
		require.Error(t, err, "len=%d should be rejected", badLen)
		require.ErrorContains(t, err, "65 bytes")
	}
}

func TestVerifyEthSecp256k1_InvalidSigFormat(t *testing.T) {
	priv, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	pk := priv.PubKey().(*ethsecp256k1.PubKey)
	sig := make([]byte, 65)
	err = sigverify.VerifyEthSecp256k1(pk, sdk.AccAddress(pk.Address()), []byte("x"), sig, types.SigFormat_SIG_FORMAT_UNSPECIFIED)
	require.Error(t, err)
	require.ErrorContains(t, err, "sig_format unspecified")
}
