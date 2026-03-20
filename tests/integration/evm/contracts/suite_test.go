//go:build integration
// +build integration

package contracts_test

import (
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
)

// TestContractsSuite runs contract integration coverage against one node
// fixture to avoid repeated startup overhead.
func TestContractsSuite(t *testing.T) {
	node := evmtest.NewEVMNode(t, "lumera-contracts-suite", 900)
	node.StartAndWaitRPC()
	defer node.Stop()

	run := func(name string, fn func(t *testing.T, node *evmtest.Node)) {
		t.Run(name, func(t *testing.T) {
			latest := node.MustGetBlockNumber(t)
			node.WaitForBlockNumberAtLeast(t, latest+1, 20*time.Second)
			fn(t, node)
		})
	}

	run("ContractDeployCallAndLogsE2E", func(t *testing.T, node *evmtest.Node) {
		testContractDeployCallAndLogsE2E(t, node)
	})
	run("ContractRevertTxReceiptAndGasE2E", func(t *testing.T, node *evmtest.Node) {
		testContractRevertTxReceiptAndGasE2E(t, node)
	})
	run("CALLBetweenContracts", func(t *testing.T, node *evmtest.Node) {
		testCALLBetweenContracts(t, node)
	})
	run("DELEGATECALLPreservesContext", func(t *testing.T, node *evmtest.Node) {
		testDELEGATECALLPreservesContext(t, node)
	})
	run("CREATE2DeterministicAddress", func(t *testing.T, node *evmtest.Node) {
		testCREATE2DeterministicAddress(t, node)
	})
	run("STATICCALLCannotModifyState", func(t *testing.T, node *evmtest.Node) {
		testSTATICCALLCannotModifyState(t, node)
	})

	// Precompile proxy tests — contracts that STATICCALL custom precompiles
	run("ContractProxiesActionGetParams", func(t *testing.T, node *evmtest.Node) {
		testContractProxiesActionGetParams(t, node)
	})
	run("ContractProxiesSupernodeGetParams", func(t *testing.T, node *evmtest.Node) {
		testContractProxiesSupernodeGetParams(t, node)
	})
	run("ContractProxiesActionGetActionFee", func(t *testing.T, node *evmtest.Node) {
		testContractProxiesActionGetActionFee(t, node)
	})
	run("ContractQueriesBothPrecompiles", func(t *testing.T, node *evmtest.Node) {
		testContractQueriesBothPrecompiles(t, node)
	})
}
