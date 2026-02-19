//go:build integration
// +build integration

package jsonrpc_test

import (
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	"testing"
	"time"
)

// TestTransactionLookupByBlockAndIndex validates both
// eth_getTransactionByBlockHashAndIndex and
// eth_getTransactionByBlockNumberAndIndex return consistent tx identity fields.
func testTransactionLookupByBlockAndIndex(t *testing.T, node *evmtest.Node) {
	t.Helper()

	txHash := evmtest.SendOneLegacyTx(t, node.RPCURL(), node.KeyInfo())
	receipt := evmtest.WaitForReceipt(t, node.RPCURL(), txHash, node.WaitCh(), node.OutputBuffer(), 40*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)

	blockHash := evmtest.MustStringField(t, receipt, "blockHash")
	blockNumber := evmtest.MustStringField(t, receipt, "blockNumber")
	txIndex := evmtest.MustStringField(t, receipt, "transactionIndex")

	var txByBlockHash map[string]any
	evmtest.MustJSONRPC(t, node.RPCURL(), "eth_getTransactionByBlockHashAndIndex", []any{blockHash, txIndex}, &txByBlockHash)
	if txByBlockHash == nil {
		t.Fatalf("eth_getTransactionByBlockHashAndIndex returned nil for block=%s index=%s", blockHash, txIndex)
	}
	evmtest.AssertTxObjectMatchesHash(t, txByBlockHash, txHash)

	var txByBlockNumber map[string]any
	evmtest.MustJSONRPC(t, node.RPCURL(), "eth_getTransactionByBlockNumberAndIndex", []any{blockNumber, txIndex}, &txByBlockNumber)
	if txByBlockNumber == nil {
		t.Fatalf("eth_getTransactionByBlockNumberAndIndex returned nil for block=%s index=%s", blockNumber, txIndex)
	}
	evmtest.AssertTxObjectMatchesHash(t, txByBlockNumber, txHash)

	evmtest.AssertTxFieldStable(t, "blockHash", txByBlockHash, txByBlockNumber)
	evmtest.AssertTxFieldStable(t, "blockNumber", txByBlockHash, txByBlockNumber)
	evmtest.AssertTxFieldStable(t, "transactionIndex", txByBlockHash, txByBlockNumber)
}
