//go:build integration
// +build integration

package jsonrpc_test

import (
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	"testing"
	"time"
)

// testReceiptPersistsAcrossRestart verifies receipt lookup durability across
// clean node restart when indexer is enabled.
func TestReceiptPersistsAcrossRestart(t *testing.T) {
	t.Helper()

	node := evmtest.NewEVMNode(t, "lumera-receipt", 200)
	node.StartAndWaitRPC()
	defer node.Stop()

	testReceiptPersistsAcrossRestart(t, node)
}

func testReceiptPersistsAcrossRestart(t *testing.T, node *evmtest.Node) {
	t.Helper()

	txHash := node.SendOneLegacyTx(t)
	receiptBefore := node.WaitForReceipt(t, txHash, 40*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receiptBefore, txHash)
	firstStartOutput := node.OutputString()

	node.RestartAndWaitRPC()

	receiptAfter := node.WaitForReceipt(t, txHash, 30*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receiptAfter, txHash)

	evmtest.AssertContains(t, firstStartOutput, "Starting EVMIndexerService service")
	evmtest.AssertContains(t, node.OutputString(), "Starting EVMIndexerService service")
}
