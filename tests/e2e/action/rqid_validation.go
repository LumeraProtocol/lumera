package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"lukechampine.com/blake3"
	mathrand "math/rand"
	"time"

	"github.com/LumeraProtocol/lumera/x/action/keeper"
	"github.com/cosmos/btcutil/base58"
)

// Supernode-style raw RQID file
type RawRQIDFile struct {
	SymbolIdentifiers []string `json:"symbol_identifiers"`
	Metadata          string   `json:"metadata"`
}

// --- Supernode logic ---
// Create Supernode-style signature string: base64(raw) + "." + signature
func buildSignatureString(raw RawRQIDFile, creatorSignature []byte) (string, error) {
	rawBytes, err := json.Marshal(raw)
	if err != nil {
		return "", fmt.Errorf("marshal raw file: %w", err)
	}
	rawB64 := base64.StdEncoding.EncodeToString(rawBytes)

	return fmt.Sprintf("%s.%s", rawB64, string(creatorSignature)), nil
}

// Generate RQID (ID) like Supernode: Base58(BLAKE3(zstd(signatureString.counter)))
func generateRQID(signatureString string, counter uint64) (string, error) {
	payload := fmt.Sprintf("%s.%d", signatureString, counter)

	compressed, err := keeper.HighCompress([]byte(payload))
	if err != nil {
		return "", fmt.Errorf("compress error: %w", err)
	}

	hash := blake3.Sum256(compressed)
	return base58.Encode(hash[:]), nil
}

// --- Main ---
func main() {
	// Step 1: Create mock raw data
	raw := RawRQIDFile{
		SymbolIdentifiers: []string{"sym-a", "sym-b", "sym-c"},
		Metadata:          "test-metadata",
	}
	creatorSig := []byte("mock_creator_signature") // same as Supernode would produce

	signatureStr, err := buildSignatureString(raw, creatorSig)
	if err != nil {
		panic(fmt.Errorf("failed to build signature string: %w", err))
	}

	// Step 2: Generate RQIDs using Supernode logic
	const count = 5
	mathrand.Seed(time.Now().UnixNano())
	startCounter := uint64(mathrand.Intn(1_000_000))

	var ids []string
	for i := uint64(0); i < count; i++ {
		id, err := generateRQID(signatureStr, startCounter+i)
		if err != nil {
			panic(fmt.Errorf("failed to generate RQID: %w", err))
		}
		fmt.Printf("Generated RQID [%d]: %s\n", startCounter+i, id)
		ids = append(ids, id)
	}

	// Step 3: Validate using Lumera logic
	err = keeper.VerifyKademliaIDs(ids, signatureStr, startCounter, count)
	if err != nil {
		panic(fmt.Errorf("❌ Validation failed: %w", err))
	}

	fmt.Println("✅ All RQIDs passed Lumera validation.")
}
