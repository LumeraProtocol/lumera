package keeper_test

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
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

	err := keeper.VerifyLegacySignature(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, pubKey.Key, sig)
	require.NoError(t, err)
}

// TestVerifyLegacySignature_InvalidPubKeySize rejects public keys that are
// not exactly 33 bytes (compressed secp256k1).
func TestVerifyLegacySignature_InvalidPubKeySize(t *testing.T) {
	legacyAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	_, newAddr := testNewMigrationAccount(t)

	// Too short.
	err := keeper.VerifyLegacySignature(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, []byte{0x01, 0x02}, nil)
	require.ErrorIs(t, err, types.ErrInvalidLegacyPubKey)

	// Too long.
	err = keeper.VerifyLegacySignature(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, make([]byte, 65), nil)
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

	err := keeper.VerifyLegacySignature(testChainID, lcfg.EVMChainID, keeperClaimKind, wrongLegacyAddr, newAddr, pubKey.Key, nil)
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

	err := keeper.VerifyLegacySignature(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, pubKey.Key, badSig)
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

	err := keeper.VerifyLegacySignature(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, pubKey.Key, sig)
	require.ErrorIs(t, err, types.ErrInvalidLegacySignature)
}

// TestVerifyLegacySignature_EmptySignature rejects a nil/empty signature.
func TestVerifyLegacySignature_EmptySignature(t *testing.T) {
	privKey := secp256k1.GenPrivKey()
	pubKey := privKey.PubKey().(*secp256k1.PubKey)
	legacyAddr := sdk.AccAddress(pubKey.Address())
	_, newAddr := testNewMigrationAccount(t)

	err := keeper.VerifyLegacySignature(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, pubKey.Key, nil)
	require.ErrorIs(t, err, types.ErrInvalidLegacySignature)
}

// TestVerifyNewSignature_Valid verifies that a correctly signed destination
// proof passes verification.
func TestVerifyNewSignature_Valid(t *testing.T) {
	legacyAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	privKey, newAddr := testNewMigrationAccount(t)
	pubKey := privKey.PubKey().(*evmcryptotypes.PubKey)
	sig := signNewMigrationMessage(t, keeperClaimKind, privKey, legacyAddr, newAddr)

	err := keeper.VerifyNewSignature(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, pubKey.Key, sig)
	require.NoError(t, err)
}

// TestVerifyNewSignature_AddressMismatch rejects when the new pubkey does not
// derive to the claimed destination address.
func TestVerifyNewSignature_AddressMismatch(t *testing.T) {
	legacyAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	privKey, _ := testNewMigrationAccount(t)
	_, wrongNewAddr := testNewMigrationAccount(t)
	pubKey := privKey.PubKey().(*evmcryptotypes.PubKey)

	err := keeper.VerifyNewSignature(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, wrongNewAddr, pubKey.Key, nil)
	require.ErrorIs(t, err, types.ErrNewPubKeyAddressMismatch)
}

// TestVerifyNewSignature_InvalidSignature rejects signatures produced by a
// different destination private key.
func TestVerifyNewSignature_InvalidSignature(t *testing.T) {
	legacyAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	privKey, newAddr := testNewMigrationAccount(t)
	pubKey := privKey.PubKey().(*evmcryptotypes.PubKey)
	otherPrivKey, _ := testNewMigrationAccount(t)
	badSig := signNewMigrationMessage(t, keeperClaimKind, otherPrivKey, legacyAddr, newAddr)

	err := keeper.VerifyNewSignature(testChainID, lcfg.EVMChainID, keeperClaimKind, legacyAddr, newAddr, pubKey.Key, badSig)
	require.ErrorIs(t, err, types.ErrInvalidNewSignature)
}
