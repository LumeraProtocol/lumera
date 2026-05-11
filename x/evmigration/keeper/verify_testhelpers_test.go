package keeper_test

// verify_testhelpers_test.go contains shared test helpers used by both
// verify_test.go and the msg_server tests. These helpers were extracted from
// verify_test.go during the Task 8 cleanup to avoid duplication.

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	evmcryptotypes "github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/stretchr/testify/require"

	lcfg "github.com/LumeraProtocol/lumera/config"
)

const (
	keeperClaimKind     = "claim"
	keeperValidatorKind = "validator"
	testChainID         = "lumera-test-1"
)

// testNewMigrationAccount creates a fresh eth_secp256k1 private key and returns
// it along with the derived AccAddress.
func testNewMigrationAccount(t *testing.T) (*evmcryptotypes.PrivKey, sdk.AccAddress) {
	t.Helper()
	privKey, err := evmcryptotypes.GenerateKey()
	require.NoError(t, err)
	return privKey, sdk.AccAddress(privKey.PubKey().Address())
}

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

// signNewMigrationMessage produces a raw SIG_FORMAT_CLI-shaped new-side signature
// (eth keyring applies Keccak256 internally; caller passes raw payload). The
// returned signature is strictly 65 bytes (R||S||V) — matching the shape
// VerifyEthSecp256k1 expects under the strict schema.
func signNewMigrationMessage(t *testing.T, kind string, privKey *evmcryptotypes.PrivKey, legacyAddr, newAddr sdk.AccAddress) []byte {
	t.Helper()
	msg := fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s", testChainID, lcfg.EVMChainID, kind, legacyAddr.String(), newAddr.String())
	sig, err := privKey.Sign([]byte(msg))
	require.NoError(t, err)
	return sig
}

// signNewMigrationEIP191 simulates what a wallet's personal_sign does:
// sign(Keccak256("\x19Ethereum Signed Message:\n" + len(payload) + payload))
// eth_secp256k1.Sign(msg) internally does Keccak256(msg) when len(msg) != 32,
// so passing the EIP-191-prefixed payload produces the correct digest.
func signNewMigrationEIP191(t *testing.T, kind string, privKey *evmcryptotypes.PrivKey, legacyAddr, newAddr sdk.AccAddress) []byte {
	t.Helper()
	payload := []byte(fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s", testChainID, lcfg.EVMChainID, kind, legacyAddr.String(), newAddr.String()))
	prefix := fmt.Appendf(nil, "\x19Ethereum Signed Message:\n%d", len(payload))
	eip191Msg := append(prefix, payload...)
	sig, err := privKey.Sign(eip191Msg)
	require.NoError(t, err)
	return sig
}
