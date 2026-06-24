//go:build integration
// +build integration

package vm_test

import (
	"testing"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
)

// TestVMSuite runs vm query coverage against a single node fixture to avoid
// repeated process startup overhead for each file-level test.
func TestVMSuite(t *testing.T) {
	node := evmtest.NewEVMNode(t, "lumera-vm-suite", 600)
	node.StartAndWaitRPC()
	defer node.Stop()

	t.Run("VMQueryParamsAndConfigBasic", func(t *testing.T) {
		testVMQueryParamsAndConfigBasic(t, node)
	})
	t.Run("VMAddressConversionRoundTrip", func(t *testing.T) {
		testVMAddressConversionRoundTrip(t, node)
	})
	t.Run("VMQueryAccountMatchesEthRPC", func(t *testing.T) {
		testVMQueryAccountMatchesEthRPC(t, node)
	})
	t.Run("VMQueryAccountRejectsInvalidAddress", func(t *testing.T) {
		testVMQueryAccountRejectsInvalidAddress(t, node)
	})
	t.Run("VMQueryAccountAcceptsHexAndBech32", func(t *testing.T) {
		testVMQueryAccountAcceptsHexAndBech32(t, node)
	})
	t.Run("VMBalanceBankMatchesBankQuery", func(t *testing.T) {
		testVMBalanceBankMatchesBankQuery(t, node)
	})
	t.Run("VMStorageQueryKeyFormatEquivalence", func(t *testing.T) {
		testVMStorageQueryKeyFormatEquivalence(t, node)
	})
	t.Run("VMQueryCodeAndStorageMatchJSONRPC", func(t *testing.T) {
		testVMQueryCodeAndStorageMatchJSONRPC(t, node)
	})
	t.Run("VMQueryAccountHistoricalHeightNonceProgression", func(t *testing.T) {
		testVMQueryAccountHistoricalHeightNonceProgression(t, node)
	})
	t.Run("VMQueryHistoricalCodeAndStorageSnapshots", func(t *testing.T) {
		testVMQueryHistoricalCodeAndStorageSnapshots(t, node)
	})
	t.Run("VMBalanceERC20MatchesEthCall", func(t *testing.T) {
		testVMBalanceERC20MatchesEthCall(t, node)
	})
	t.Run("VMBalanceERC20RejectsNonERC20Runtime", func(t *testing.T) {
		testVMBalanceERC20RejectsNonERC20Runtime(t, node)
	})
}
