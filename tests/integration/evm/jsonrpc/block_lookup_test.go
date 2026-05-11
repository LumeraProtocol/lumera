//go:build integration
// +build integration

package jsonrpc_test

import (
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	"testing"
	"time"
)

// TestBlockLookupIncludesTransaction validates block lookup consistency across
// number/hash selectors and hash-only/full-transaction payload modes.
func testBlockLookupIncludesTransaction(t *testing.T, node *evmtest.Node) {
	t.Helper()

	txHash := node.SendOneLegacyTx(t)
	receipt := node.WaitForReceipt(t, txHash, 40*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)

	blockHash := evmtest.MustStringField(t, receipt, "blockHash")
	blockNumber := evmtest.MustStringField(t, receipt, "blockNumber")

	// Verify both block lookup modes (by number/hash and hash-only/full tx payloads).
	blockByNumberHashes := node.MustGetBlock(t, "eth_getBlockByNumber", []any{blockNumber, false})
	evmtest.AssertBlockContainsTxHash(t, blockByNumberHashes, txHash)

	blockByNumberFull := node.MustGetBlock(t, "eth_getBlockByNumber", []any{blockNumber, true})
	evmtest.AssertBlockContainsFullTx(t, blockByNumberFull, txHash)

	blockByHashHashes := node.MustGetBlock(t, "eth_getBlockByHash", []any{blockHash, false})
	evmtest.AssertBlockContainsTxHash(t, blockByHashHashes, txHash)

	blockByHashFull := node.MustGetBlock(t, "eth_getBlockByHash", []any{blockHash, true})
	evmtest.AssertBlockContainsFullTx(t, blockByHashFull, txHash)
}
