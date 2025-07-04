package testutils

import (
	"encoding/hex"
	"encoding/csv"
	"os"
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"

	cryptoutils "github.com/LumeraProtocol/lumera/x/claim/keeper/crypto"
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

type ClaimCSVRecord struct {
	OldAddress string `csv:"old_address"`
	Amount	 uint64 `csv:"amount"`
}

// GenerateClaimsCSVFile creates a temporary claims file using the provided test data.
// File is created in a temporary directory and has unique name to avoid conflicts.
// It returns the file path and an error if anything goes wrong.
func GenerateClaimsCSVFile(data []ClaimCSVRecord) (string, error) {
	// Create a uniquely named temporary file
	file, err := os.CreateTemp("", "claims-*.csv")
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Write CSV header and rows
	writer := csv.NewWriter(file)
	defer writer.Flush()

	for _, record := range data {
		if err := writer.Write([]string{record.OldAddress, strconv.FormatUint(record.Amount, 10)}); err != nil {
			return "", fmt.Errorf("failed to write record to CSV: %w", err)
		}
	}
	// set permissions to 0644
	if err := file.Chmod(0644); err != nil {
		return "", fmt.Errorf("failed to set file permissions: %w", err)
	}

	return file.Name(), nil
}

// CleanupClaimsCSVFile removes the specified claims CSV file.
// It returns an error if the file cannot be removed.
func CleanupClaimsCSVFile(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("file path is empty")
	}

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to remove claims CSV file: %w", err)
	}

	return nil
}