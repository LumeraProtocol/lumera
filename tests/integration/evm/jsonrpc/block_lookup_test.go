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

	txHash := evmtest.SendOneLegacyTx(t, node.RPCURL(), node.KeyInfo())
	receipt := evmtest.WaitForReceipt(t, node.RPCURL(), txHash, node.WaitCh(), node.OutputBuffer(), 40*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)

	blockHash := evmtest.MustStringField(t, receipt, "blockHash")
	blockNumber := evmtest.MustStringField(t, receipt, "blockNumber")

	// Verify both block lookup modes (by number/hash and hash-only/full tx payloads).
	blockByNumberHashes := evmtest.MustGetBlock(t, node.RPCURL(), "eth_getBlockByNumber", []any{blockNumber, false})
	evmtest.AssertBlockContainsTxHash(t, blockByNumberHashes, txHash)

	blockByNumberFull := evmtest.MustGetBlock(t, node.RPCURL(), "eth_getBlockByNumber", []any{blockNumber, true})
	evmtest.AssertBlockContainsFullTx(t, blockByNumberFull, txHash)

	blockByHashHashes := evmtest.MustGetBlock(t, node.RPCURL(), "eth_getBlockByHash", []any{blockHash, false})
	evmtest.AssertBlockContainsTxHash(t, blockByHashHashes, txHash)

	blockByHashFull := evmtest.MustGetBlock(t, node.RPCURL(), "eth_getBlockByHash", []any{blockHash, true})
	evmtest.AssertBlockContainsFullTx(t, blockByHashFull, txHash)
}
