package upgrades

import lcfg "github.com/LumeraProtocol/lumera/config"

// These thin wrappers delegate to the canonical chain-ID detection in the
// config package so the prefixes live in a single leaf package that upgrade
// version subpackages (e.g. v1_20_0) can import without an import cycle.

// IsMainnet returns true if the chain ID corresponds to mainnet.
func IsMainnet(chainID string) bool {
	return lcfg.IsMainnetChainID(chainID)
}

// IsTestnet returns true if the chain ID corresponds to testnet.
func IsTestnet(chainID string) bool {
	return lcfg.IsTestnetChainID(chainID)
}

// IsDevnet returns true if the chain ID corresponds to devnet.
func IsDevnet(chainID string) bool {
	return lcfg.IsDevnetChainID(chainID)
}
