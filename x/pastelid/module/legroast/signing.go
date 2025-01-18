package legroast

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/argon2"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
)

const (
	LegRoastSeedPassphrase = "Pastel_LegRoast_Seed_Generator"
)

// GenerateLegRoastKeySeed generates a seed for the LegRoast key generation from address.
var GenerateLegRoastKeySeed = func (address string, kr keyring.Keyring) ([]byte, error) {
	// Convert address string to Cosmos SDK address
	addr, err := types.AccAddressFromBech32(address)
	if err != nil {
		return nil, fmt.Errorf("invalid Cosmos address: %w", err)
	}

	if kr == nil {
		return nil, errors.New("keyring is not set")
	}

	// Retrieve the public key
	keyInfo, err := kr.KeyByAddress(addr)
	if err != nil {
		return nil, fmt.Errorf("address not found in keyring: %w", err)
	}
	pubKey, err := keyInfo.GetPubKey()
	if err != nil {
		return nil, errors.New("failed to get public key")
	}

	// Concatenate the public key with the predefined phrase
	dataToSign := append(pubKey.Bytes(), []byte(LegRoastSeedPassphrase)...)

	// Sign the concatenated data
	sig, _, err := kr.SignByAddress(addr, dataToSign, signingtypes.SignMode_SIGN_MODE_DIRECT)
	if err != nil {
		return nil, fmt.Errorf("failed to sign data: %w", err)
	}

	// Hash the signature using Argon2
	hashed := argon2.IDKey(sig, []byte(LegRoastSeedPassphrase), 1, 64*1024, 4, 32)

	// Extract the first 16 bytes as the seed
	return hashed[:16], nil
}

// Sign generates a LegRoast signature for the provided message.
var Sign = func (address string, kr keyring.Keyring, message []byte, algorithm LegRoastAlgorithm) ([]byte, []byte, error) {
	// Generate the seed
	seed, err := GenerateLegRoastKeySeed(address, kr)
	if err != nil {
		return nil, nil, err
	}

	// Initialize LegRoast with the specified algorithm
	lr := NewLegRoast(algorithm)

	// Generate keys
	if err := lr.Keygen(seed); err != nil {
		return nil, nil, fmt.Errorf("failed to generate LegRoast keys: %w", err) 
	}

	// Sign the message
	signature, err := lr.Sign(message)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to sign message with LegRoast: %w", err)
	}

	return signature, lr.PublicKey(), nil
}

// Verify verifies the LegRoast signature for the provided message.
var Verify = func (message, pubKey, signature []byte) error {
	// Get the algorithm based on the signature size
	algorithm, err := GetAlgorithmBySigSize(len(signature))
	if err != nil {
		return err
	}

	// Initialize LegRoast with the specified algorithm
	lr := NewLegRoast(algorithm)

	// Set the public key
	if err := lr.SetPublicKey(pubKey); err != nil {
		return fmt.Errorf("failed to set public key for LegRoast: %w", err)
	}

	// Verify the signature
	if err := lr.Verify(message, signature); err != nil {
		return fmt.Errorf("failed to verify LegRoast signature: %w", err)
	}

	return nil
}