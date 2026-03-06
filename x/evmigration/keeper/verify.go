package keeper

import (
	"crypto/sha256"
	"fmt"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// VerifyLegacySignature verifies the inner legacy signature that proves
// the caller controls the legacy (secp256k1 / coin-type-118) private key.
//
// Steps:
//  1. Decode legacy_pub_key as secp256k1.PubKey
//  2. Derive address from pubkey, verify it matches legacy_address
//  3. Construct canonical message: SHA256("migrate:<legacy_address>:<new_address>")
//  4. Verify signature over the message hash
func VerifyLegacySignature(legacyAddr, newAddr sdk.AccAddress, legacyPubKeyBytes, legacySignature []byte) error {
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
	msg := fmt.Sprintf("lumera-evm-migration:%s:%s", legacyAddr.String(), newAddr.String())
	hash := sha256.Sum256([]byte(msg))

	// Step 4: verify the legacy signature.
	if !pubKey.VerifySignature(hash[:], legacySignature) {
		return types.ErrInvalidLegacySignature
	}

	return nil
}
