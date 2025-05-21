package testutils

import (
	"encoding/hex"
	"fmt"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"

	cryptoutils "github.com/LumeraProtocol/lumera/x/claim/keeper/crypto"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type TestData struct {
	OldAddress string
	PubKey     string
	NewAddress string
	Signature  string
}

type PastelAccount struct {
	Address string
	PubKey  string
	PrivKey *secp256k1.PrivKey
}

func GeneratePastelAddress() (PastelAccount, error) {
	// Generate a new key pair
	privKeyObj, pubKeyObj := cryptoutils.GenerateKeyPair()

	// Get hex encoded public key
	pubKey := hex.EncodeToString(pubKeyObj.Key)

	// Generate old address from public key
	oldAddr, err := cryptoutils.GetAddressFromPubKey(pubKey)
	if err != nil {
		return PastelAccount{}, fmt.Errorf("failed to generate pastel address: %w", err)
	}

	return PastelAccount{
		Address: oldAddr,
		PubKey:  pubKey,
		PrivKey: privKeyObj,
	}, nil
}

func GenerateClaimingTestData2(pastelAccount PastelAccount, lumeraAddr string) (TestData, error) {
	// Construct message for signature (without hashing)
	message := pastelAccount.Address + "." + pastelAccount.PubKey + "." + lumeraAddr

	// Sign the message directly without hashing
	signature, err := cryptoutils.SignMessage(pastelAccount.PrivKey, message)
	if err != nil {
		return TestData{}, fmt.Errorf("failed to sign message: %w", err)
	}

	// Verify the signature to ensure it's valid
	valid, err := cryptoutils.VerifySignature(pastelAccount.PubKey, message, signature)
	if err != nil {
		return TestData{}, fmt.Errorf("failed to verify generated signature: %w", err)
	}
	if !valid {
		return TestData{}, fmt.Errorf("generated signature verification failed")
	}

	return TestData{
		OldAddress: pastelAccount.Address,
		PubKey:     pastelAccount.PubKey,
		NewAddress: lumeraAddr,
		Signature:  signature,
	}, nil
}

func GenerateClaimingTestData() (TestData, error) {
	// Generate a new key pair
	privKeyObj, pubKeyObj := cryptoutils.GenerateKeyPair()

	// Get hex encoded public key
	pubKey := hex.EncodeToString(pubKeyObj.Key)

	// Generate old address from public key
	oldAddr, err := cryptoutils.GetAddressFromPubKey(pubKey)
	if err != nil {
		return TestData{}, fmt.Errorf("failed to generate old address: %w", err)
	}

	newAddr := sdk.AccAddress(privKeyObj.PubKey().Address()).String()

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
