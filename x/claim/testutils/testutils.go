package testutils

import (
	"encoding/hex"
	"fmt"

	cryptoutils "github.com/LumeraProtocol/lumera/x/claim/keeper/crypto"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type TestData struct {
	OldAddress string
	PubKey     string
	NewAddress string
	Signature  string
}

func GenerateClaimingTestData() (TestData, error) {
	return GenerateClaimingTestData2("")
}

func GenerateClaimingTestData2(existingNewAddr string) (TestData, error) {
	// Generate a new key pair
	privKeyObj, pubKeyObj := cryptoutils.GenerateKeyPair()

	// Get hex encoded public key
	pubKey := hex.EncodeToString(pubKeyObj.Key)

	// Generate old address from public key
	oldAddr, err := cryptoutils.GetAddressFromPubKey(pubKey)
	if err != nil {
		return TestData{}, fmt.Errorf("failed to generate old address: %w", err)
	}

	// If an existing new address is provided, use it; otherwise, generate a new one
	newAddr := existingNewAddr
	if newAddr == "" {
		newAddr = sdk.AccAddress(privKeyObj.PubKey().Address()).String()
	}

	// Construct message for signature (without hashing)
	message := oldAddr + "." + pubKey + "." + newAddr

	// Sign the message directly without hashing
	signature, err := cryptoutils.SignMessage(privKeyObj, message)
	if err != nil {
		return TestData{}, fmt.Errorf("failed to sign message: %w", err)
	}

	// Verify the signature to ensure it's valid
	valid, err := cryptoutils.VerifySignature(pubKey, message, signature)
	if err != nil {
		return TestData{}, fmt.Errorf("failed to verify generated signature: %w", err)
	}
	if !valid {
		return TestData{}, fmt.Errorf("generated signature verification failed")
	}

	return TestData{
		OldAddress: oldAddr,
		PubKey:     pubKey,
		NewAddress: newAddr,
		Signature:  signature,
	}, nil
}
