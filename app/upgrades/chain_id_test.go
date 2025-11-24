package upgrades

import "testing"

func TestChainIDHelpers(t *testing.T) {
	type testCase struct {
		chainID                                    string
		expectMainnet, expectTestnet, expectDevnet bool
	}

	tests := []testCase{
		{chainID: "lumera-mainnet-1", expectMainnet: true},
		{chainID: "lumera-testnet-2", expectTestnet: true},
		{chainID: "lumera-devnet-3", expectDevnet: true},
		{chainID: "lumera-unknown-4"},
	}

	for _, tc := range tests {
		if IsMainnet(tc.chainID) != tc.expectMainnet {
			t.Fatalf("IsMainnet(%q) = %v, want %v", tc.chainID, IsMainnet(tc.chainID), tc.expectMainnet)
		}
		if IsTestnet(tc.chainID) != tc.expectTestnet {
			t.Fatalf("IsTestnet(%q) = %v, want %v", tc.chainID, IsTestnet(tc.chainID), tc.expectTestnet)
		}
		if IsDevnet(tc.chainID) != tc.expectDevnet {
			t.Fatalf("IsDevnet(%q) = %v, want %v", tc.chainID, IsDevnet(tc.chainID), tc.expectDevnet)
		}
	}
}
