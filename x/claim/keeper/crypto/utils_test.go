package crypto

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
)

func TestGetAddressFromPubKey(t *testing.T) {
	tests := []struct {
		name            string
		pubKey          string
		expectedAddress string
		expectError     bool
	}{
		{
			name:            "Valid public key",
			pubKey:          "0309331fc3d23ca17d91eec40ee7711efcd56facf949d46cbfa6393d43f2747e90",
			expectedAddress: "PtqHAEacynVd3V821NPhgxu9K4Ab6kAguHi",
			expectError:     false,
		},
		{
			name:            "Invalid hex string",
			pubKey:          "invalid",
			expectedAddress: "",
			expectError:     true,
		},
		{
			name:            "Empty public key",
			pubKey:          "",
			expectedAddress: "",
			expectError:     true, // hex.DecodeString will fail for empty string
		},
		{
			name:            "Odd length hex string",
			pubKey:          "123",
			expectedAddress: "",
			expectError:     true,
		},
		{
			name:            "Wrong length public key (32 bytes)",
			pubKey:          strings.Repeat("00", 32),
			expectedAddress: "",
			expectError:     false,
		},
		{
			name:            "Public key with invalid prefix (not 0x02 or 0x03)",
			pubKey:          "0409331fc3d23ca17d91eec40ee7711efcd56facf949d46cbfa6393d43f2747e90",
			expectedAddress: "",
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			address, err := GetAddressFromPubKey(tt.pubKey)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected an error for input '%s', but got none", tt.pubKey)
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if tt.expectedAddress != "" && address != tt.expectedAddress {
					t.Errorf("Expected address %s, but got %s", tt.expectedAddress, address)
				}
			}
		})
	}
}

func TestVerifySignature(t *testing.T) {
	tests := []struct {
		name        string
		pubKey      string
		message     string
		signature   string
		expectValid bool
		expectError bool
	}{
		{
			name:        "Valid signature",
			pubKey:      "0309331fc3d23ca17d91eec40ee7711efcd56facf949d46cbfa6393d43f2747e90",
			message:     "PtqHAEacynVd3V821NPhgxu9K4Ab6kAguHi.0309331fc3d23ca17d91eec40ee7711efcd56facf949d46cbfa6393d43f2747e90.pastel139k6camfq63u9gtc4pq8yjw4j7tmwmqeggr4p0",
			signature:   "1f46b3a2129047a0d7a6bf91e2879e940ed3db06a2cafaaaabacc337141146f43e4932d357b435bbf2c48227f5c2f738df23a2ebc221dd11cb14ed4b83bd2a95c7",
			expectValid: true,
			expectError: false,
		},
		{
			name:        "Modified signature",
			pubKey:      "0309331fc3d23ca17d91eec40ee7711efcd56facf949d46cbfa6393d43f2747e90",
			message:     "PtqHAEacynVd3V821NPhgxu9K4Ab6kAguHi.0309331fc3d23ca17d91eec40ee7711efcd56facf949d46cbfa6393d43f2747e90.pastel139k6camfq63u9gtc4pq8yjw4j7tmwmqeggr4p0",
			signature:   "1f46b3a2129047a0d7a6bf91e2879e940ed3db06a2cafaaaabacc337141146f43e4932d357b435bbf2c48227f5c2f738df23a2ebc221dd11cb14ed4b83bd2a95c8",
			expectValid: false,
			expectError: false,
		},
		{
			name:        "Invalid public key hex",
			pubKey:      "invalid",
			message:     "test",
			signature:   "1f46b3a2129047a0d7a6bf91e2879e940ed3db06a2cafaaaabacc337141146f43e4932d357b435bbf2c48227f5c2f738df23a2ebc221dd11cb14ed4b83bd2a95c7",
			expectValid: false,
			expectError: true,
		},
		{
			name:        "Empty public key",
			pubKey:      "",
			message:     "test",
			signature:   "1f46b3a2129047a0d7a6bf91e2879e940ed3db06a2cafaaaabacc337141146f43e4932d357b435bbf2c48227f5c2f738df23a2ebc221dd11cb14ed4b83bd2a95c7",
			expectValid: false,
			expectError: true,
		},
		{
			name:        "Invalid signature hex",
			pubKey:      "0309331fc3d23ca17d91eec40ee7711efcd56facf949d46cbfa6393d43f2747e90",
			message:     "test",
			signature:   "invalid",
			expectValid: false,
			expectError: true,
		},
		{
			name:        "Empty signature",
			pubKey:      "0309331fc3d23ca17d91eec40ee7711efcd56facf949d46cbfa6393d43f2747e90",
			message:     "test",
			signature:   "",
			expectValid: false,
			expectError: true,
		},
		{
			name:        "Empty message",
			pubKey:      "0309331fc3d23ca17d91eec40ee7711efcd56facf949d46cbfa6393d43f2747e90",
			message:     "",
			signature:   "1f46b3a2129047a0d7a6bf91e2879e940ed3db06a2cafaaaabacc337141146f43e4932d357b435bbf2c48227f5c2f738df23a2ebc221dd11cb14ed4b83bd2a95c7",
			expectValid: false,
			expectError: false,
		},
		{
			name:        "Wrong length signature (64 bytes)",
			pubKey:      "0309331fc3d23ca17d91eec40ee7711efcd56facf949d46cbfa6393d43f2747e90",
			message:     "test",
			signature:   strings.Repeat("00", 64),
			expectValid: false,
			expectError: true,
		},
		{
			name:        "Invalid public key format (wrong length)",
			pubKey:      strings.Repeat("00", 32), // 64 chars = 32 bytes instead of 33
			message:     "test",
			signature:   "1f46b3a2129047a0d7a6bf91e2879e940ed3db06a2cafaaaabacc337141146f43e4932d357b435bbf2c48227f5c2f738df23a2ebc221dd11cb14ed4b83bd2a95c7",
			expectValid: false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid, err := VerifySignature(tt.pubKey, tt.message, tt.signature)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected an error, but got none")
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if isValid != tt.expectValid {
					t.Errorf("Expected validity %v, but got %v", tt.expectValid, isValid)
				}
			}
		})
	}
}

func TestGenerateKeyPair(t *testing.T) {
	// Test multiple key pairs to ensure consistency
	for i := 0; i < 10; i++ {
		privKey, pubKey := GenerateKeyPair()

		// Test private key
		if privKey == nil {
			t.Fatal("Expected private key, got nil")
		}
		if len(privKey.Bytes()) != 32 {
			t.Errorf("Expected private key length 32, got %d", len(privKey.Bytes()))
		}

		// Test public key
		if pubKey == nil {
			t.Fatal("Expected public key, got nil")
		}
		if len(pubKey.Bytes()) != 33 {
			t.Errorf("Expected compressed public key length 33, got %d", len(pubKey.Bytes()))
		}

		// Verify public key format (should start with 0x02 or 0x03)
		firstByte := pubKey.Bytes()[0]
		if firstByte != 0x02 && firstByte != 0x03 {
			t.Errorf("Invalid public key format: first byte should be 0x02 or 0x03, got 0x%x", firstByte)
		}

		// Verify key pair relationship
		derivedPub := privKey.PubKey()
		if !derivedPub.Equals(pubKey) {
			t.Error("Derived public key doesn't match generated public key")
		}
	}
}

func TestSignMessage(t *testing.T) {
	tests := []struct {
		name        string
		message     string
		useNilKey   bool
		expectError bool
	}{
		{
			name:        "Valid message",
			message:     "test message",
			useNilKey:   false,
			expectError: false,
		},
		{
			name:        "Empty message",
			message:     "",
			useNilKey:   false,
			expectError: false,
		},
		{
			name:        "Long message",
			message:     strings.Repeat("test", 1000),
			useNilKey:   false,
			expectError: false,
		},
		{
			name:        "Message with special characters",
			message:     "!@#$%^&*()_+-=[]{}|;:,.<>?",
			useNilKey:   false,
			expectError: false,
		},
		{
			name:        "Unicode message",
			message:     "Hello, 世界",
			useNilKey:   false,
			expectError: false,
		},
		{
			name:        "Nil private key",
			message:     "test message",
			useNilKey:   true,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var privKey *secp256k1.PrivKey
			var pubKey *secp256k1.PubKey

			if !tt.useNilKey {
				privKey, pubKey = GenerateKeyPair()
			}

			// Sign the message
			signature, err := SignMessage(privKey, tt.message)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected an error, but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify signature format
			sigBytes, err := hex.DecodeString(signature)
			if err != nil {
				t.Fatalf("Failed to decode signature hex: %v", err)
			}

			if len(sigBytes) != 65 {
				t.Errorf("Expected signature length 65, got %d", len(sigBytes))
			}

			if sigBytes[0] != 27 && sigBytes[0] != 28 {
				t.Errorf("Invalid recovery byte: expected 27 or 28, got %d", sigBytes[0])
			}

			// Verify the signature using VerifySignature
			pubKeyHex := hex.EncodeToString(pubKey.Bytes())
			isValid, err := VerifySignature(pubKeyHex, tt.message, signature)
			if err != nil {
				t.Fatalf("Signature verification failed with error: %v", err)
			}
			if !isValid {
				t.Error("Signature verification failed")
			}
		})
	}
}
