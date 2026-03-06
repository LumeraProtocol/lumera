package config

// EVMChainID is the EVM chain ID for the Lumera network.
// Each EVM-compatible chain requires a unique chain ID.
const EVMChainID uint64 = 76857769

const (
	// FeeMarketDefaultBaseFee is the default feemarket base fee in `ulume` per gas.
	// With 6-decimal ulume and 18-decimal EVM internals this maps to 2.5 gwei.
	FeeMarketDefaultBaseFee = "0.0025"

	// FeeMarketMinGasPrice is the minimum gas price floor for EIP-1559 base fee
	// decay. Prevents the base fee from reaching zero on low-activity chains.
	// Set to 0.5 gwei equivalent (20% of the default base fee).
	FeeMarketMinGasPrice = "0.0005"

	// FeeMarketBaseFeeChangeDenominator controls the rate at which the base fee
	// adjusts per block. Higher values produce gentler adjustments.
	// Default cosmos/evm value is 8 (~12.5% per block); 16 gives ~6.25%.
	FeeMarketBaseFeeChangeDenominator uint32 = 16

	// ChainDefaultConsensusMaxGas is the default Comet consensus max gas per block.
	// A finite value is required for meaningful EIP-1559 base fee adjustments.
	// 25M aligns with Kava/Cronos and provides headroom for DeFi workloads.
	ChainDefaultConsensusMaxGas int64 = 25_000_000
)
