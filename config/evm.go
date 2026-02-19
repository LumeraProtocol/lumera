package config

// EVMChainID is the EVM chain ID for the Lumera network.
// Each EVM-compatible chain requires a unique chain ID.
const EVMChainID uint64 = 76857769

const (
	// FeeMarketDefaultBaseFee is the default feemarket base fee in `ulume` per gas.
	// With 6-decimal ulume and 18-decimal EVM internals this maps to 2.5 gwei.
	FeeMarketDefaultBaseFee = "0.0025"

	// ChainDefaultConsensusMaxGas is the default Comet consensus max gas per block.
	// A finite value is required for meaningful EIP-1559 base fee adjustments.
	ChainDefaultConsensusMaxGas int64 = 10_000_000
)
