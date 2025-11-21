package keeper

import (
	"context"
	"crypto/rand"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"time"

	"golang.org/x/sync/semaphore"

	errorsmod "cosmossdk.io/errors"
	"github.com/DataDog/zstd"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	"github.com/cosmos/btcutil/base58"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"lukechampine.com/blake3"
)

const (
	semaphoreWeight              = 1
	maxParallelHighCompressCalls = 5
	highCompressionLevel         = 4
	highCompressTimeout          = 30 * time.Minute
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
func (k *Keeper) VerifySignature(ctx sdk.Context, dataB64 string, signature string, signerAddress string) error {
	// 1. Get account PubKey
	accAddr, err := k.addressCodec.StringToBytes(signerAddress)
	if err != nil {
		return errorsmod.Wrapf(actiontypes.ErrInvalidSignature,
			"invalid account address: %s", err)
	}
	account := k.authKeeper.GetAccount(ctx, accAddr)
	if account == nil {
		return errorsmod.Wrapf(actiontypes.ErrInvalidSignature,
			"account not found for address: %s", signerAddress)
	}
	pubKey := account.GetPubKey()
	if pubKey == nil {
		return errorsmod.Wrapf(actiontypes.ErrInvalidSignature,
			"account has no public key: %s", signerAddress)
	}

	// 2. Decode the base64 signature
	sigRaw, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return errorsmod.Wrapf(actiontypes.ErrInvalidSignature,
			"failed to decode signature: %s", err)
	}
	sigRS, err := CoerceToRS64(sigRaw)
	if err != nil {
		return errorsmod.Wrapf(actiontypes.ErrInvalidSignature, "sig format: %s", err)
	}

	// 3. Verify the signature
	if pubKey.VerifySignature([]byte(dataB64), sigRS) {
		return nil
	}

	// 4) ADR-36 (Keplr/browser)
	signBytes, err := MakeADR36AminoSignBytes(signerAddress, dataB64)
	if err == nil && pubKey.VerifySignature(signBytes, sigRS) {
		return nil
	}

	return errorsmod.Wrap(actiontypes.ErrInvalidSignature, "signature verification failed")
}

// VerifyKademliaIDs verifies that a Kademlia ID matches the expected format and content.
//
// Cascade ID Format is `Base58(BLAKE3(zstd_compressed(Base64(rq_ids).creators_signature.counter)))`
// Sense Format is `Base58(BLAKE3(zstd_compressed(Base64(rq_ids).sn1_signature.sn2_signature.sn3_signature.counter)))`
//
// ID Format is `Base58(BLAKE3(zstd_compressed(<Metadata.Signatures>.counter)))`
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
	if len(ids) == 0 {
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
	if idIndex >= counterMax {
		return fmt.Errorf("invalid random index: %d", idIndex)
	}

	// Use the random index to get a random ID
	randomID := ids[idIndex]
	counter := counterIc + idIndex

	// Create the expected format: Base58(BLAKE3(zstd_compressed(signatures.counter)))
	expectedID, err := CreateKademliaID(signatures, counter)
	if err != nil {
		return fmt.Errorf("failed to create expected ID: %v", err)
	}

	// Compare with the provided ID
	if randomID != expectedID {
		return errorsmod.Wrap(actiontypes.ErrInvalidID, "Kademlia ID doesn't match expected format")
	}

	return nil
}

// CreateKademliaID - Create the expected format: Base58(BLAKE3(zstd_compressed(signatures.counter)))
func CreateKademliaID(signatures string, counter uint64) (string, error) {
	// Concatenate signatures and counter
	dataToCompress := fmt.Sprintf("%s.%d", signatures, counter)

	// Compress the data using zstd
	compressedData, err := ZstdCompress([]byte(dataToCompress))
	if err != nil {
		return "", fmt.Errorf("failed to zstd compress data: %v", err)
	}

	// Compute BLAKE3 hash of the compressed data
	hashedData := blake3.Sum256(compressedData)

	// Encode the hashed data using Base58
	return base58.Encode(hashedData[:]), nil
}

// ZstdCompress Helper function for zstd compression using official C library at level 3
func ZstdCompress(data []byte) ([]byte, error) {
	compressed, err := zstd.CompressLevel(nil, data, 3)
	if err != nil {
		return nil, fmt.Errorf("failed to compress with zstd: %v", err)
	}
	return compressed, nil
}

func HighCompress(data []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), highCompressTimeout)
	defer cancel()

	sem := semaphore.NewWeighted(maxParallelHighCompressCalls)

	// Acquire the semaphore. This will block if 5 other goroutines are already inside this function.
	if err := sem.Acquire(ctx, semaphoreWeight); err != nil {
		return nil, fmt.Errorf("failed to acquire semaphore: %v", err)
	}
	defer sem.Release(semaphoreWeight) // Ensure that the semaphore is always released

	compressed, err := zstd.CompressLevel(nil, data, highCompressionLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to compress data: %v", err)
	}

	return compressed, nil
}

// --- DER → 64-byte r||s ---
type ecdsaSig struct{ R, S *big.Int }

// CoerceToRS64 returns r||s (64 bytes) from either 64-byte input or DER.
func CoerceToRS64(sig []byte) ([]byte, error) {
	if len(sig) == 64 {
		return sig, nil
	}
	var es ecdsaSig
	if _, err := asn1.Unmarshal(sig, &es); err != nil {
		return nil, fmt.Errorf("parse DER: %w", err)
	}
	r := es.R.Bytes()
	s := es.S.Bytes()
	if len(r) > 32 || len(s) > 32 {
		return nil, fmt.Errorf("r/s too large")
	}
	out := make([]byte, 64)
	copy(out[32-len(r):32], r)
	copy(out[64-len(s):], s)
	return out, nil
}

// --- ADR-36 sign-bytes (Amino JSON) ---

// MakeADR36AminoSignBytes returns the exact JSON bytes Keplr signs.
// signerBech32: bech32 address; dataB64: base64 STRING that was given to Keplr signArbitrary().
func MakeADR36AminoSignBytes(signerBech32, dataB64 string) ([]byte, error) {
	doc := map[string]any{
		"account_number": "0",
		"chain_id":       "",
		"fee": map[string]any{
			"amount": []any{},
			"gas":    "0",
		},
		"memo": "",
		"msgs": []any{
			map[string]any{
				"type": "sign/MsgSignData",
				"value": map[string]any{
					"data":   dataB64,      // IMPORTANT: base64 STRING (do not decode)
					"signer": signerBech32, // bech32 account address
				},
			},
		},
		"sequence": "0",
	}

	canon := sortObjectByKey(doc)
	bz, err := json.Marshal(canon)
	if err != nil {
		return nil, fmt.Errorf("marshal adr36 doc: %w", err)
	}
	return bz, nil
}

func sortObjectByKey(v any) any {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(x))
		for _, k := range keys {
			out[k] = sortObjectByKey(x[k])
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = sortObjectByKey(x[i])
		}
		return out
	default:
		return v
	}
}
