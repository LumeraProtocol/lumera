package upgrades

import "strings"

const (
	chainPrefixMainnet = "lumera-mainnet"
	chainPrefixTestnet = "lumera-testnet"
	chainPrefixDevnet  = "lumera-devnet"
)

// IsMainnet returns true if the chain ID corresponds to mainnet.
func IsMainnet(chainID string) bool {
	return strings.HasPrefix(chainID, chainPrefixMainnet)
}

// IsTestnet returns true if the chain ID corresponds to testnet.
func IsTestnet(chainID string) bool {
	return strings.HasPrefix(chainID, chainPrefixTestnet)
}

// IsDevnet returns true if the chain ID corresponds to devnet.
func IsDevnet(chainID string) bool {
	return strings.HasPrefix(chainID, chainPrefixDevnet)
}
