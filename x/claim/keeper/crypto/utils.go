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
// The function implements the following address generation algorithm:
// 1. Decode the hex-encoded public key string to bytes
// 2. Compute SHA256 hash of the public key bytes
// 3. Compute RIPEMD160 hash of the SHA256 result
// 4. Prepend version bytes (0x0c, 0xe3)
// 5. Compute checksum by double SHA256 of the versioned payload
// 6. Append 4-byte checksum to the versioned payload
// 7. Encode final bytes in base58
//
// Parameters:
//   - pubKey: Hex-encoded string of the public key
//
// Returns:
//   - string: Base58-encoded address
//   - error: Error if public key is empty, invalid hex, or processing fails
func GetAddressFromPubKey(pubKey string) (string, error) {
	// Check for empty public key
	if pubKey == "" {
		return "", fmt.Errorf("public key cannot be empty")
	}

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

// VerifySignature verifies a secp256k1 signature against a message using a given public key.
//
// Parameters:
//   - compressedPubKeyHex: Hex-encoded 33-byte compressed public key (starting with 0x02 or 0x03)
//   - message: Plain text message that was signed
//   - signatureHex: Hex-encoded 65-byte signature (1 byte recovery ID + 64 bytes signature)
//
// Returns:
//   - bool: True if signature is valid, false otherwise
//   - error: Error if inputs are invalid (wrong length, format) or processing fails
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

// GenerateKeyPair generates a new secp256k1 private/public key pair.
//
// Returns:
//   - *secp256k1.PrivKey: Generated private key
//   - *secp256k1.PubKey: Corresponding public key
func GenerateKeyPair() (*secp256k1.PrivKey, *secp256k1.PubKey) {
	privKey := secp256k1.GenPrivKey()
	pubKey := privKey.PubKey().(*secp256k1.PubKey)
	return privKey, pubKey
}

// SignMessage signs a message using the secp256k1 private key.
// The message is first hashed with SHA256, then signed.
// A recovery byte (27) is prepended to the signature for compatibility.
//
// Parameters:
//   - privKey: secp256k1 private key used for signing
//   - message: Plain text message to sign
//
// Returns:
//   - string: Hex-encoded 65-byte signature (1 byte recovery ID + 64 bytes signature)
//   - error: Error if private key is nil or signing fails
func SignMessage(privKey *secp256k1.PrivKey, message string) (string, error) {
	if privKey == nil {
		return "", fmt.Errorf("private key cannot be nil")
	}

	// Hash the message first as per the verification function
	messageHash := sha256.Sum256([]byte(message))

	// Sign the hash
	signature, err := privKey.Sign(messageHash[:])
	if err != nil {
		return "", fmt.Errorf("failed to sign message: %w", err)
	}

	// Convert to hex string
	// Add recovery byte (27 + recovery_id) at the beginning
	// Default to 27 (recovery_id = 0) for simplicity
	signatureWithRecovery := append([]byte{27}, signature...)
	return hex.EncodeToString(signatureWithRecovery), nil
}
