package crypto

import (
	"testing"
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
			name:            "Invalid public key",
			pubKey:          "invalid",
			expectedAddress: "",
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			address, err := GetAddressFromPubKey(tt.pubKey)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected an error, but got none")
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if address != tt.expectedAddress {
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
			name:        "Invalid signature",
			pubKey:      "0309331fc3d23ca17d91eec40ee7711efcd56facf949d46cbfa6393d43f2747e90",
			message:     "PtqHAEacynVd3V821NPhgxu9K4Ab6kAguHi.0309331fc3d23ca17d91eec40ee7711efcd56facf949d46cbfa6393d43f2747e90.pastel139k6camfq63u9gtc4pq8yjw4j7tmwmqeggr4p0",
			signature:   "1f46b3a2129047a0d7a6bf91e2879e940ed3db06a2cafaaaabacc337141146f43e4932d357b435bbf2c48227f5c2f738df23a2ebc221dd11cb14ed4b83bd2a95c8",
			expectValid: false,
			expectError: false,
		},
		{
			name:        "Invalid public key",
			pubKey:      "invalid",
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
