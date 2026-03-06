package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/btcsuite/btcutil/base58"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"golang.org/x/crypto/ripemd160"
)

// claimKeyEntry holds a pre-seeded Pastel keypair for claim testing.
type claimKeyEntry struct {
	PrivKeyHex string // 32-byte secp256k1 private key (hex)
	PubKeyHex  string // 33-byte compressed public key (hex)
	OldAddress string // Pastel base58 address
	Amount     int64  // claim amount in ulume
}

const numClaimKeys = 100

// claimAmountPattern is the repeating 20-element cycle used for claim amounts.
// Each full cycle sums to 20,450,000 ulume; 5 cycles = 102,250,000 total.
var claimAmountPattern = [20]int64{
	500000, 750000, 1000000, 1250000, 1500000,
	600000, 800000, 1100000, 1300000, 1600000,
	550000, 700000, 950000, 1150000, 1400000,
	650000, 850000, 1050000, 1200000, 1550000,
}

// preseededClaimKeys maps Pastel base58 address → claimKeyEntry.
// Generated deterministically from SHA256("lumera-devnet-claim-test-{i}").
var preseededClaimKeys map[string]claimKeyEntry

// preseededClaimKeysByIndex preserves insertion order for iteration.
var preseededClaimKeysByIndex []claimKeyEntry

func init() {
	preseededClaimKeys = make(map[string]claimKeyEntry, numClaimKeys)
	preseededClaimKeysByIndex = make([]claimKeyEntry, 0, numClaimKeys)

	for i := 0; i < numClaimKeys; i++ {
		seed := sha256.Sum256([]byte(fmt.Sprintf("lumera-devnet-claim-test-%d", i)))
		privKey := &secp256k1.PrivKey{Key: seed[:]}
		pubKey := privKey.PubKey().(*secp256k1.PubKey)

		entry := claimKeyEntry{
			PrivKeyHex: hex.EncodeToString(seed[:]),
			PubKeyHex:  hex.EncodeToString(pubKey.Key),
			OldAddress: pastelAddressFromPubKey(pubKey.Key),
			Amount:     claimAmountPattern[i%len(claimAmountPattern)],
		}
		preseededClaimKeys[entry.OldAddress] = entry
		preseededClaimKeysByIndex = append(preseededClaimKeysByIndex, entry)
	}
}

// signClaimMessage signs the claim verification message using a pre-seeded Pastel private key.
// Message format: "old_address.pubkey_hex.new_address"
// Returns hex-encoded 65-byte signature (recovery byte + 64 bytes).
func signClaimMessage(entry claimKeyEntry, newAddress string) (string, error) {
	privBytes, err := hex.DecodeString(entry.PrivKeyHex)
	if err != nil {
		return "", fmt.Errorf("decode private key: %w", err)
	}
	privKey := &secp256k1.PrivKey{Key: privBytes}

	msg := entry.OldAddress + "." + entry.PubKeyHex + "." + newAddress
	hash := sha256.Sum256([]byte(msg))
	sig, err := privKey.Sign(hash[:])
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}
	// Prepend recovery byte (27) for Pastel-compatible format.
	return hex.EncodeToString(append([]byte{27}, sig...)), nil
}

// verifyClaimKeyIntegrity checks that all pre-seeded keys produce the expected Pastel addresses.
// Call once at startup to catch data corruption.
func verifyClaimKeyIntegrity() error {
	if len(preseededClaimKeys) != numClaimKeys {
		return fmt.Errorf("expected %d claim keys, got %d", numClaimKeys, len(preseededClaimKeys))
	}
	for i, entry := range preseededClaimKeysByIndex {
		pubBytes, err := hex.DecodeString(entry.PubKeyHex)
		if err != nil {
			return fmt.Errorf("key %d: decode pubkey: %w", i, err)
		}
		addr := pastelAddressFromPubKey(pubBytes)
		if addr != entry.OldAddress {
			return fmt.Errorf("key %d: expected address %s, got %s", i, entry.OldAddress, addr)
		}
	}
	log.Printf("claim key integrity check passed: %d keys verified", numClaimKeys)
	return nil
}

// pastelAddressFromPubKey derives a Pastel base58 address from a compressed secp256k1 public key.
func pastelAddressFromPubKey(pubKeyBytes []byte) string {
	sha := sha256.Sum256(pubKeyBytes)
	rip := ripemd160.New()
	rip.Write(sha[:])
	pubKeyHash := rip.Sum(nil)
	versioned := append([]byte{0x0c, 0xe3}, pubKeyHash...)
	first := sha256.Sum256(versioned)
	second := sha256.Sum256(first[:])
	return base58.Encode(append(versioned, second[:4]...))
}
