package keeper

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	"github.com/LumeraProtocol/lumera/x/action/types"
	"github.com/cosmos/btcutil/base58"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/klauspost/compress/zstd"
	"golang.org/x/crypto/sha3"
	"math/big"
)

// VerifySignature verifies that a signature is valid for given data and signer.
//
// The function performs these validation steps:
// 1. Validates that the signer address is valid
// 2. Decodes the base64-encoded signature
// 3. Verifies the signature against the provided data
//
// Parameters:
// - data: The original data that was signed (string format)
// - signature: Base64-encoded signature
// - signerAddress: Bech32 address of the signer
//
// Returns an error if:
// - The address format is invalid
// - The signature cannot be decoded
// - The signature verification fails
// - Any other validation error occurs
func (k *Keeper) VerifySignature(ctx sdk.Context, data string, signature string, signerAddress string) error {
	// 1. Get account PubKey
	accAddr, err := sdk.AccAddressFromBech32(signerAddress)
	if err != nil {
		return errorsmod.Wrapf(types.ErrInvalidSignature,
			"invalid account address: %s", err)
	}
	account := k.accountKeeper.GetAccount(ctx, accAddr)
	if account == nil {
		return errorsmod.Wrapf(types.ErrInvalidSignature,
			"account not found for address: %s", signerAddress)
	}
	pubKey := account.GetPubKey()
	if pubKey == nil {
		return errorsmod.Wrapf(types.ErrInvalidSignature,
			"account has no public key: %s", signerAddress)
	}

	// 2. Decode the base64 signature
	sigBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return errorsmod.Wrapf(types.ErrInvalidSignature,
			"failed to decode signature: %s", err)
	}

	// 3. Verify the signature
	// PubKey.VerifySignature uses `ed25519consensus.Verify` from `https://github.com/hdevalence/ed25519consensus`
	// it uses sha512 internally
	isValid := pubKey.VerifySignature([]byte(data), sigBytes)
	if !isValid {
		return errorsmod.Wrap(types.ErrInvalidSignature, "signature verification failed")
	}

	return nil
}

// VerifyKademliaIDs verifies that a Kademlia ID matches the expected format and content.
//
// Cascade ID Format is `Base58(SHA3_256(zstd_compressed(Base64(rq_ids).creators_signature.counter)))`
// Sense Format is `Base58(SHA3_256(zstd_compressed(Base64(rq_ids).sn1_signature.sn2_signature.sn3_signature.counter)))`
//
// ID Format is `Base58(SHA3_256(zstd_compressed(<Metadata.Signatures>.counter)))`
//
// Parameters:
// - id: The Kademlia ID to verify
// - signatures: Metadata.Signatures
// - counter: The counter of the identifier
//
// Returns an error if:
// - Any input parameter is empty or invalid
// - The ID doesn't match the expected format or value
// - Any step in the verification process fails
func VerifyKademliaIDs(ids []string, signatures string, counterIc uint64, counterMax uint64) error {
	// Validate input parameters
	if ids == nil || len(ids) == 0 {
		return fmt.Errorf("empty ID")
	}

	if signatures == "" {
		return fmt.Errorf("empty signatures")
	}

	if counterMax <= 0 {
		return fmt.Errorf("invalid counter: %d", counterMax)
	}

	// 1. Verify RqIdsIds size
	if uint64(len(ids)) != counterMax {
		return fmt.Errorf("number of ids (%d) doesn't match ids_max (%d)", len(ids), counterMax)
	}

	// 2. Verify IDs are not empty
	for i, id := range ids {
		if len(id) == 0 {
			return fmt.Errorf("rq_id at position %d is empty", i)
		}
	}

	// 3. Verify IDs match the expected format

	// Generate a random index between 0 and RqIdsMax-1
	randomIndex, err := rand.Int(rand.Reader, big.NewInt(int64(counterMax)))
	if err != nil {
		return fmt.Errorf("failed to generate random index: %v", err)
	}

	idIndex := randomIndex.Uint64()
	if idIndex < 0 || idIndex >= counterMax {
		return fmt.Errorf("invalid random index: %d", idIndex)
	}

	// Use the random index to get a random ID
	randomID := ids[idIndex]
	counter := counterIc + idIndex

	// Create the expected format: Base58(SHA3_256(zstd_compressed(signatures.counter)))
	expectedID, err := CreateKademliaID(signatures, counter)
	if err != nil {
		return fmt.Errorf("failed to create expected ID: %v", err)
	}

	// Compare with the provided ID
	if randomID != expectedID {
		return errorsmod.Wrap(types.ErrInvalidID, "Kademlia ID doesn't match expected format")
	}

	return nil
}

// CreateKademliaID - Create the expected format: Base58(SHA3_256(zstd_compressed(signatures.counter)))
func CreateKademliaID(signatures string, counter uint64) (string, error) {
	// Concatenate signatures and counter
	dataToCompress := fmt.Sprintf("%s.%d", signatures, counter)

	// Compress the data using zstd
	compressedData, err := zstdCompress([]byte(dataToCompress))
	if err != nil {
		return "", fmt.Errorf("failed to zstd compress data: %v", err)
	}

	// Compute SHA3-256 hash of the compressed data
	hasher := sha3.New256()
	_, err = hasher.Write(compressedData)
	if err != nil {
		return "", fmt.Errorf("failed to compute SHA3-256 hash: %v", err)
	}
	hashedData := hasher.Sum(nil)

	// Encode the hashed data using Base58
	return base58.Encode(hashedData), nil
}

// Helper function for zstd compression
func zstdCompress(data []byte) ([]byte, error) {
	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd encoder: %v", err)
	}
	defer encoder.Close()

	return encoder.EncodeAll(data, nil), nil
}
