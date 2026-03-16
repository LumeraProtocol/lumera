package keeper

import (
	"crypto/sha256"
	"fmt"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	evmcryptotypes "github.com/cosmos/evm/crypto/ethsecp256k1"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

const (
	migrationPayloadKindClaim     = "claim"
	migrationPayloadKindValidator = "validator"
)

func migrationPayload(kind string, legacyAddr, newAddr sdk.AccAddress) []byte {
	return []byte(fmt.Sprintf("lumera-evm-migration:%s:%s:%s", kind, legacyAddr.String(), newAddr.String()))
}

// VerifyLegacySignature verifies the legacy-account proof embedded in a
// migration message. Legacy keys use Cosmos secp256k1 signing over SHA-256.
func VerifyLegacySignature(kind string, legacyAddr, newAddr sdk.AccAddress, legacyPubKeyBytes, legacySignature []byte) error {
	// Step 1: decode the compressed secp256k1 public key.
	if len(legacyPubKeyBytes) != secp256k1.PubKeySize {
		return types.ErrInvalidLegacyPubKey.Wrapf("expected %d bytes, got %d", secp256k1.PubKeySize, len(legacyPubKeyBytes))
	}
	pubKey := &secp256k1.PubKey{Key: legacyPubKeyBytes}

	// Step 2: derive address and verify it matches legacy_address.
	derivedAddr := sdk.AccAddress(pubKey.Address())
	if !derivedAddr.Equals(legacyAddr) {
		return types.ErrPubKeyAddressMismatch.Wrapf(
			"pubkey derives to %s, expected %s", derivedAddr, legacyAddr,
		)
	}

	// Step 3: construct canonical message hash.
	hash := sha256.Sum256(migrationPayload(kind, legacyAddr, newAddr))

	// Step 4: verify the legacy signature.
	if !pubKey.VerifySignature(hash[:], legacySignature) {
		return types.ErrInvalidLegacySignature
	}

	return nil
}

// VerifyNewSignature verifies the destination-account proof embedded in a
// migration message. New EVM addresses use eth_secp256k1 signing over the raw
// payload, which the eth key implementation internally hashes with Keccak-256.
func VerifyNewSignature(kind string, legacyAddr, newAddr sdk.AccAddress, newPubKeyBytes, newSignature []byte) error {
	if len(newPubKeyBytes) != evmcryptotypes.PubKeySize {
		return types.ErrInvalidNewPubKey.Wrapf("expected %d bytes, got %d", evmcryptotypes.PubKeySize, len(newPubKeyBytes))
	}
	pubKey := &evmcryptotypes.PubKey{Key: newPubKeyBytes}

	derivedAddr := sdk.AccAddress(pubKey.Address())
	if !derivedAddr.Equals(newAddr) {
		return types.ErrNewPubKeyAddressMismatch.Wrapf(
			"pubkey derives to %s, expected %s", derivedAddr, newAddr,
		)
	}

	if !pubKey.VerifySignature(migrationPayload(kind, legacyAddr, newAddr), newSignature) {
		return types.ErrInvalidNewSignature
	}

	return nil
}
