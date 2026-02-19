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
	node := evmtest.NewEVMNode(t, "lumera-contracts-suite", 700)
	node.StartAndWaitRPC()
	defer node.Stop()

	run := func(name string, fn func(t *testing.T, node *evmtest.Node)) {
		t.Run(name, func(t *testing.T) {
			latest := evmtest.MustGetBlockNumber(t, node.RPCURL())
			evmtest.WaitForBlockNumberAtLeast(t, node.RPCURL(), latest+1, 20*time.Second)
			fn(t, node)
		})
	}

	run("ContractDeployCallAndLogsE2E", func(t *testing.T, node *evmtest.Node) {
		testContractDeployCallAndLogsE2E(t, node)
	})
	run("ContractRevertTxReceiptAndGasE2E", func(t *testing.T, node *evmtest.Node) {
		testContractRevertTxReceiptAndGasE2E(t, node)
	})
}
