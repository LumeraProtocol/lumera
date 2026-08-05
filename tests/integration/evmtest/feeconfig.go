//go:build integration
// +build integration

package evmtest

import (
	"math"
	"math/big"
	"testing"

	lcfg "github.com/LumeraProtocol/lumera/config"
)

// uLumeToWeiScale converts a `ulume`-denominated decimal gas price (6 decimals)
// into the 18-decimal wei space the EVM JSON-RPC surface reports.
const uLumeToWeiScale = 1_000_000_000_000

// MustULumeDecToWei converts a decimal `ulume`-per-gas string (e.g. the
// config.FeeMarket* constants) into wei.
//
// Tests must derive fee expectations from the config constants rather than
// hardcoding gwei literals: the default base fee is a tuning parameter, and
// baking its current value into assertions makes every fee change look like a
// test regression.
func MustULumeDecToWei(t *testing.T, decValue string) *big.Int {
	t.Helper()

	parsed, ok := new(big.Rat).SetString(decValue)
	if !ok {
		t.Fatalf("invalid decimal value %q", decValue)
	}

	scaled := new(big.Rat).Mul(parsed, new(big.Rat).SetInt(big.NewInt(uLumeToWeiScale)))
	if scaled.Denom().Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("decimal value %q is not convertible to exact wei integer: %s", decValue, scaled.RatString())
	}

	return new(big.Int).Set(scaled.Num())
}

// DefaultBaseFeeWei is the configured feemarket starting base fee, in wei.
func DefaultBaseFeeWei(t *testing.T) *big.Int {
	t.Helper()
	return MustULumeDecToWei(t, lcfg.FeeMarketDefaultBaseFee)
}

// MinGasPriceWei is the configured feemarket decay floor, in wei.
func MinGasPriceWei(t *testing.T) *big.Int {
	t.Helper()
	return MustULumeDecToWei(t, lcfg.FeeMarketMinGasPrice)
}

// BaseFeeDrainBlocks returns how many empty blocks are required for the base
// fee to decay from the configured default down to the configured floor.
//
// Each empty block multiplies the base fee by (den-1)/den, so the count is
// log(default/floor) / log(den/(den-1)). Callers that wait for a "cheap" chain
// must size their budget from this rather than a fixed literal, otherwise
// raising FeeMarketDefaultBaseFee silently breaks them.
func BaseFeeDrainBlocks(t *testing.T) int {
	t.Helper()

	def := new(big.Float).SetInt(DefaultBaseFeeWei(t))
	floor := new(big.Float).SetInt(MinGasPriceWei(t))
	if floor.Sign() <= 0 || def.Cmp(floor) <= 0 {
		return 1
	}

	den := float64(lcfg.FeeMarketBaseFeeChangeDenominator)
	if den <= 1 {
		return 1
	}

	ratio, _ := new(big.Float).Quo(def, floor).Float64()
	blocks := int(math.Ceil(math.Log(ratio) / math.Log(den/(den-1))))
	if blocks < 1 {
		return 1
	}
	return blocks
}

// MinCosmosGasPriceWithHeadroom returns a `ulume` gas price string safe to pass
// to `--gas-prices` for Cosmos-side txs in EVM tests.
//
// The global minimum fee scales with the feemarket base fee, so a flat
// `--fees` literal (e.g. 1000ulume at 200k gas = 0.005ulume/gas) silently falls
// below the floor whenever the base fee is raised. Using the configured default
// base fee with headroom keeps these txs accepted across tuning changes.
func MinCosmosGasPriceWithHeadroom() string {
	return lcfg.FeeMarketDefaultBaseFee
}
