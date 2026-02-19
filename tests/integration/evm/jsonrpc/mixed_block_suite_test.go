//go:build integration
// +build integration

package jsonrpc_test

import (
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
)

// TestJSONRPCMixedBlockSuite runs mixed Cosmos+EVM block coverage with app-side
// mempool disabled so both tx types can be co-included in the same block path.
func TestJSONRPCMixedBlockSuite(t *testing.T) {
	node := evmtest.NewEVMNode(t, "lumera-jsonrpc-mixed-suite", 400)
	node.AppendStartArgs("--mempool.max-txs", "-1")
	node.StartAndWaitRPC()
	defer node.Stop()

	run := func(name string, fn func(t *testing.T, node *evmtest.Node)) {
		t.Run(name, func(t *testing.T) {
			latest := evmtest.MustGetBlockNumber(t, node.RPCURL())
			evmtest.WaitForBlockNumberAtLeast(t, node.RPCURL(), latest+1, 20*time.Second)
			fn(t, node)
		})
	}

	run("MixedCosmosAndEVMTransactionsCanShareBlock", func(t *testing.T, node *evmtest.Node) {
		testMixedCosmosAndEVMTransactionsCanShareBlock(t, node)
	})
	run("MixedBlockOrderingPersistsAcrossRestart", func(t *testing.T, node *evmtest.Node) {
		testMixedBlockOrderingPersistsAcrossRestart(t, node)
	})
}
