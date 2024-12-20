package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcutil/base58"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"golang.org/x/crypto/ripemd160"
)

// GetAddressFromPubKey generates a base58-encoded address from a hex-encoded public key.
// It follows these steps:
// 1. Decode the hex public key
// 2. Perform SHA256 hash on the public key
// 3. Perform RIPEMD160 hash on the result of SHA256
// 4. Prepend version bytes
// 5. Calculate checksum (double SHA256)
// 6. Append checksum to versioned payload
// 7. Encode the result in base58
func GetAddressFromPubKey(pubKey string) (string, error) {
	// Decode the hex-encoded public key
	publicKeyBytes, err := hex.DecodeString(pubKey)
	if err != nil {
		return "", fmt.Errorf("failed to decode public key hex: %w", err)
	}

	// Perform SHA256 hash on the public key
	sha256Hash := sha256.Sum256(publicKeyBytes)

	// Perform RIPEMD160 hash on the result of SHA256
	ripemd160Hasher := ripemd160.New()
	ripemd160Hasher.Write(sha256Hash[:])
	pubKeyHash := ripemd160Hasher.Sum(nil)

	// Prepend version bytes
	versionedPayload := []byte{0x0c, 0xe3}
	versionedPayload = append(versionedPayload, pubKeyHash...)

	// Calculate checksum (double SHA256)
	firstSHA := sha256.Sum256(versionedPayload)
	secondSHA := sha256.Sum256(firstSHA[:])
	checksum := secondSHA[:4]

	// Append checksum to versioned payload
	finalBytes := append(versionedPayload, checksum...)

	// Encode in base58
	address := base58.Encode(finalBytes)

	return address, nil
}

// VerifySignature verifies a signature against a message using a given public key.
// It takes hex-encoded strings for the public key and signature, and a plain string for the message.
// Returns true if the signature is valid, false otherwise.
func VerifySignature(compressedPubKeyHex string, message string, signatureHex string) (bool, error) {
	// Decode the hex-encoded public key
	compressedPubKey, err := hex.DecodeString(compressedPubKeyHex)
	if err != nil {
		return false, fmt.Errorf("failed to decode public key hex: %w", err)
	}

	// Verify the public key format
	if len(compressedPubKey) != 33 || (compressedPubKey[0] != 0x02 && compressedPubKey[0] != 0x03) {
		return false, fmt.Errorf("invalid compressed public key format")
	}

	// Create a secp256k1 public key object
	pubKey := &secp256k1.PubKey{
		Key: compressedPubKey,
	}

	// Decode the hex-encoded signature
	signature, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false, fmt.Errorf("failed to decode signature hex: %w", err)
	}

	// Verify signature length and extract the relevant bytes
	if len(signature) != 65 {
		return false, fmt.Errorf("invalid signature length: expected 65 bytes, got %d", len(signature))
	}

	// Remove the recovery ID from the signature
	signatureToVerify := signature[1:65]

	// Hash the message with SHA256 before verification
	messageHash := sha256.Sum256([]byte(message))

	// Verify the signature
	return pubKey.VerifySignature(messageHash[:], signatureToVerify), nil
}
