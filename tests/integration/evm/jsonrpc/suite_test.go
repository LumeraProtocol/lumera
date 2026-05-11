//go:build integration
// +build integration

package jsonrpc_test

import (
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
)

// TestJSONRPCSuite runs standard JSON-RPC/indexer integration checks against a
// single node fixture. Tests that require custom startup modes (startup smoke
// and indexer-disabled) remain standalone.
func TestJSONRPCSuite(t *testing.T) {
	node := evmtest.NewEVMNode(t, "lumera-jsonrpc-suite", 900)
	node.StartAndWaitRPC()
	defer node.Stop()

	run := func(name string, fn func(t *testing.T, node *evmtest.Node)) {
		t.Run(name, func(t *testing.T) {
			latest := node.MustGetBlockNumber(t)
			node.WaitForBlockNumberAtLeast(t, latest+1, 20*time.Second)
			fn(t, node)
		})
	}

	run("BasicRPCMethods", func(t *testing.T, node *evmtest.Node) {
		testBasicRPCMethods(t, node)
	})
	run("OpenRPCDiscoverMethodCatalog", func(t *testing.T, node *evmtest.Node) {
		testOpenRPCDiscoverMethodCatalog(t, node)
	})
	run("OpenRPCDiscoverMatchesEmbeddedSpec", func(t *testing.T, node *evmtest.Node) {
		testOpenRPCDiscoverMatchesEmbeddedSpec(t, node)
	})
	run("BackendBlockCountAndUncleSemantics", func(t *testing.T, node *evmtest.Node) {
		testBackendBlockCountAndUncleSemantics(t, node)
	})
	run("BackendNetAndWeb3UtilityMethods", func(t *testing.T, node *evmtest.Node) {
		testBackendNetAndWeb3UtilityMethods(t, node)
	})
	run("BlockLookupIncludesTransaction", func(t *testing.T, node *evmtest.Node) {
		testBlockLookupIncludesTransaction(t, node)
	})
	run("TransactionLookupByBlockAndIndex", func(t *testing.T, node *evmtest.Node) {
		testTransactionLookupByBlockAndIndex(t, node)
	})
	run("MultiTxOrderingSameBlock", func(t *testing.T, node *evmtest.Node) {
		testMultiTxOrderingSameBlock(t, node)
	})
	run("ReceiptIncludesCanonicalFields", func(t *testing.T, node *evmtest.Node) {
		testReceiptIncludesCanonicalFields(t, node)
	})
	run("BatchJSONRPCReturnsAllResponses", func(t *testing.T, node *evmtest.Node) {
		testBatchJSONRPCReturnsAllResponses(t, node)
	})
	run("BatchJSONRPCMixedErrorsAndResults", func(t *testing.T, node *evmtest.Node) {
		testBatchJSONRPCMixedErrorsAndResults(t, node)
	})
	run("BatchJSONRPCSingleElementBatch", func(t *testing.T, node *evmtest.Node) {
		testBatchJSONRPCSingleElementBatch(t, node)
	})
	run("BatchJSONRPCDuplicateMethods", func(t *testing.T, node *evmtest.Node) {
		testBatchJSONRPCDuplicateMethods(t, node)
	})
}
