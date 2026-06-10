package config

import (
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Chain-ID prefixes identifying each Lumera network. Chain IDs are of the form
// "<prefix>-<n>" (e.g. "lumera-mainnet-1", "lumera-devnet-3").
const (
	MainnetChainIDPrefix = "lumera-mainnet"
	TestnetChainIDPrefix = "lumera-testnet"
	DevnetChainIDPrefix  = "lumera-devnet"
)

// IsMainnetChainID reports whether chainID belongs to a Lumera mainnet network.
func IsMainnetChainID(chainID string) bool {
	return strings.HasPrefix(chainID, MainnetChainIDPrefix)
}

// IsTestnetChainID reports whether chainID belongs to a Lumera testnet network.
func IsTestnetChainID(chainID string) bool {
	return strings.HasPrefix(chainID, TestnetChainIDPrefix)
}

// IsDevnetChainID reports whether chainID belongs to a Lumera devnet network.
func IsDevnetChainID(chainID string) bool {
	return strings.HasPrefix(chainID, DevnetChainIDPrefix)
}

const (
	// DefaultMaxIBCCallbackGas is the default value of maximum gas that an IBC callback can use.
	// If the callback uses more gas, it will be out of gas and the contract state changes will be reverted,
	// but the transaction will be committed.
	// Pass this to the callbacks middleware or choose a custom value.
	DefaultMaxIBCCallbackGas = uint64(1_000_000)

	// ChainDenom is the denomination of the chain's native token.
	ChainDenom = "ulume"
	// ChainDisplayDenom is the human-readable display denomination.
	ChainDisplayDenom = "lume"
	// ChainEVMExtendedDenom is the 18-decimal EVM denomination used by x/vm and x/precisebank.
	ChainEVMExtendedDenom = "alume"
	// ChainTokenName is the canonical token name used in bank metadata.
	ChainTokenName = "Lumera"
	// ChainTokenSymbol is the canonical token symbol used in bank metadata.
	ChainTokenSymbol = "LUME"
)

func SetupConfig() {
	// Set and seal config
	config := sdk.GetConfig()

	// Keep SDK fallback in sync with chain denom.
	sdk.DefaultBondDenom = ChainDenom

	// Set BIP44 coin type and derivation path.
	SetBip44CoinType(config)

	// Set the Bech32 prefixes for accounts, validators, and consensus nodes
	SetBech32Prefixes(config)

	// Seal the config to prevent further modifications
	config.Seal()
}

func init() {
	SetupConfig()
}
