//go:build integration
// +build integration

package jsonrpc_test

import (
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	"testing"
	"time"
)

// testTransactionByHashPersistsAcrossRestart verifies tx-by-hash lookup and key
// positional fields remain stable across node restart.
func TestTransactionByHashPersistsAcrossRestart(t *testing.T) {
	t.Helper()

	node := evmtest.NewEVMNode(t, "lumera-txhash", 220)
	node.StartAndWaitRPC()
	defer node.Stop()

	testTransactionByHashPersistsAcrossRestart(t, node)
}

func testTransactionByHashPersistsAcrossRestart(t *testing.T, node *evmtest.Node) {
	t.Helper()

	txHash := node.SendOneLegacyTx(t)
	node.WaitForReceipt(t, txHash, 40*time.Second)
	txBefore := node.WaitForTransactionByHash(t, txHash, 20*time.Second)
	evmtest.AssertTxObjectMatchesHash(t, txBefore, txHash)

	node.RestartAndWaitRPC()

	txAfter := node.WaitForTransactionByHash(t, txHash, 20*time.Second)
	evmtest.AssertTxObjectMatchesHash(t, txAfter, txHash)

	evmtest.AssertTxFieldStable(t, "blockHash", txBefore, txAfter)
	evmtest.AssertTxFieldStable(t, "blockNumber", txBefore, txAfter)
	evmtest.AssertTxFieldStable(t, "transactionIndex", txBefore, txAfter)
}
