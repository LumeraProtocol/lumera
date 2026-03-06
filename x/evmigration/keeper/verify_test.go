package keeper_test

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/evmigration/keeper"
	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// signMigrationMessage creates a valid legacy signature over the canonical
// "lumera-evm-migration:<legacy>:<new>" message for testing.
func signMigrationMessage(t *testing.T, privKey *secp256k1.PrivKey, legacyAddr, newAddr sdk.AccAddress) []byte {
	t.Helper()
	msg := fmt.Sprintf("lumera-evm-migration:%s:%s", legacyAddr.String(), newAddr.String())
	hash := sha256.Sum256([]byte(msg))
	sig, err := privKey.Sign(hash[:])
	require.NoError(t, err)
	return sig
}

// TestVerifyLegacySignature_Valid verifies that a correctly signed migration
// message passes verification.
func TestVerifyLegacySignature_Valid(t *testing.T) {
	privKey := secp256k1.GenPrivKey()
	pubKey := privKey.PubKey().(*secp256k1.PubKey)
	legacyAddr := sdk.AccAddress(pubKey.Address())
	newAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())

	sig := signMigrationMessage(t, privKey, legacyAddr, newAddr)

	err := keeper.VerifyLegacySignature(legacyAddr, newAddr, pubKey.Key, sig)
	require.NoError(t, err)
}

// TestVerifyLegacySignature_InvalidPubKeySize rejects public keys that are
// not exactly 33 bytes (compressed secp256k1).
func TestVerifyLegacySignature_InvalidPubKeySize(t *testing.T) {
	legacyAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	newAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())

	// Too short.
	err := keeper.VerifyLegacySignature(legacyAddr, newAddr, []byte{0x01, 0x02}, nil)
	require.ErrorIs(t, err, types.ErrInvalidLegacyPubKey)

	// Too long.
	err = keeper.VerifyLegacySignature(legacyAddr, newAddr, make([]byte, 65), nil)
	require.ErrorIs(t, err, types.ErrInvalidLegacyPubKey)
}

// TestVerifyLegacySignature_PubKeyAddressMismatch rejects when the public key
// does not derive to the claimed legacy address.
func TestVerifyLegacySignature_PubKeyAddressMismatch(t *testing.T) {
	privKey := secp256k1.GenPrivKey()
	pubKey := privKey.PubKey().(*secp256k1.PubKey)

	// Use a different address as legacy (not derived from this pubkey).
	wrongLegacyAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	newAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())

	err := keeper.VerifyLegacySignature(wrongLegacyAddr, newAddr, pubKey.Key, nil)
	require.ErrorIs(t, err, types.ErrPubKeyAddressMismatch)
}

// TestVerifyLegacySignature_InvalidSignature rejects a signature produced by
// a different private key than the one matching the public key.
func TestVerifyLegacySignature_InvalidSignature(t *testing.T) {
	privKey := secp256k1.GenPrivKey()
	pubKey := privKey.PubKey().(*secp256k1.PubKey)
	legacyAddr := sdk.AccAddress(pubKey.Address())
	newAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())

	// Sign with a different key.
	otherPrivKey := secp256k1.GenPrivKey()
	badSig := signMigrationMessage(t, otherPrivKey, legacyAddr, newAddr)

	err := keeper.VerifyLegacySignature(legacyAddr, newAddr, pubKey.Key, badSig)
	require.ErrorIs(t, err, types.ErrInvalidLegacySignature)
}

// TestVerifyLegacySignature_WrongMessage rejects a valid signature that was
// produced over a different new address than the one being verified.
func TestVerifyLegacySignature_WrongMessage(t *testing.T) {
	privKey := secp256k1.GenPrivKey()
	pubKey := privKey.PubKey().(*secp256k1.PubKey)
	legacyAddr := sdk.AccAddress(pubKey.Address())
	newAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())

	// Sign over a different new address.
	otherNewAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	sig := signMigrationMessage(t, privKey, legacyAddr, otherNewAddr)

	err := keeper.VerifyLegacySignature(legacyAddr, newAddr, pubKey.Key, sig)
	require.ErrorIs(t, err, types.ErrInvalidLegacySignature)
}

// TestVerifyLegacySignature_EmptySignature rejects a nil/empty signature.
func TestVerifyLegacySignature_EmptySignature(t *testing.T) {
	privKey := secp256k1.GenPrivKey()
	pubKey := privKey.PubKey().(*secp256k1.PubKey)
	legacyAddr := sdk.AccAddress(pubKey.Address())
	newAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())

	err := keeper.VerifyLegacySignature(legacyAddr, newAddr, pubKey.Key, nil)
	require.ErrorIs(t, err, types.ErrInvalidLegacySignature)
}
