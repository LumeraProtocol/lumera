//go:build integration
// +build integration

package feemarket_test

import (
	"testing"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
)

// TestFeeMarketSuite runs feemarket integration coverage against one node
// fixture to avoid repeated chain startup per test.
func TestFeeMarketSuite(t *testing.T) {
	node := evmtest.NewEVMNode(t, "lumera-feemarket-suite", 700)
	node.StartAndWaitRPC()
	defer node.Stop()

	t.Run("FeeHistoryReportsCanonicalShape", func(t *testing.T) {
		testFeeHistoryReportsCanonicalShape(t, node)
	})
	t.Run("ReceiptEffectiveGasPriceRespectsBlockBaseFee", func(t *testing.T) {
		testReceiptEffectiveGasPriceRespectsBlockBaseFee(t, node)
	})
	t.Run("FeeHistoryRewardPercentilesShape", func(t *testing.T) {
		testFeeHistoryRewardPercentilesShape(t, node)
	})
	t.Run("MaxPriorityFeePerGasReturnsValidHex", func(t *testing.T) {
		testMaxPriorityFeePerGasReturnsValidHex(t, node)
	})
	t.Run("GasPriceIsAtLeastLatestBaseFee", func(t *testing.T) {
		testGasPriceIsAtLeastLatestBaseFee(t, node)
	})
	t.Run("DynamicFeeType2EffectiveGasPriceFormula", func(t *testing.T) {
		testDynamicFeeType2EffectiveGasPriceFormula(t, node)
	})
	t.Run("DynamicFeeType2RejectsFeeCapBelowBaseFee", func(t *testing.T) {
		testDynamicFeeType2RejectsFeeCapBelowBaseFee(t, node)
	})
	t.Run("BaseFeeProgressesAcrossMultiBlockLoadPattern", func(t *testing.T) {
		testBaseFeeProgressesAcrossMultiBlockLoadPattern(t, node)
	})
}
