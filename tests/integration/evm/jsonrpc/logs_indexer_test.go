//go:build integration
// +build integration

package jsonrpc_test

import (
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	"strings"
	"testing"
	"time"
)

// testLogsIndexerPathAcrossRestart verifies log indexer behavior across process
// restart using address and topic filters.
//
// Workflow:
// 1. Deploy a log-emitting contract creation tx.
// 2. Query logs by address and by address+topic.
// 3. Restart node and re-run the same queries.
func TestLogsIndexerPathAcrossRestart(t *testing.T) {
	t.Helper()

	node := evmtest.NewEVMNode(t, "lumera-logs-indexer", 240)
	node.StartAndWaitRPC()
	defer node.Stop()

	testLogsIndexerPathAcrossRestart(t, node)
}

func testLogsIndexerPathAcrossRestart(t *testing.T, node *evmtest.Node) {
	t.Helper()

	logTopic := "0x" + strings.Repeat("11", 32)
	txHash := evmtest.SendLogEmitterCreationTx(t, node.RPCURL(), node.KeyInfo(), logTopic)
	receipt := evmtest.WaitForReceipt(t, node.RPCURL(), txHash, node.WaitCh(), node.OutputBuffer(), 40*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)

	blockNumber := evmtest.MustStringField(t, receipt, "blockNumber")
	contractAddress := evmtest.MustStringField(t, receipt, "contractAddress")
	if strings.EqualFold(contractAddress, "0x0000000000000000000000000000000000000000") {
		t.Fatalf("unexpected zero contractAddress in receipt: %#v", receipt)
	}

	// Validate log queries by address and by address+topic before restart.
	addressFilter := map[string]any{
		"fromBlock": blockNumber,
		"toBlock":   blockNumber,
		"address":   contractAddress,
	}
	logsByAddressBefore := evmtest.MustGetLogs(t, node.RPCURL(), addressFilter)
	assertLogsContainTxAndAddress(t, logsByAddressBefore, txHash, contractAddress)

	addressAndTopicFilter := map[string]any{
		"fromBlock": blockNumber,
		"toBlock":   blockNumber,
		"address":   contractAddress,
		"topics":    []any{logTopic},
	}
	logsByTopicBefore := evmtest.MustGetLogs(t, node.RPCURL(), addressAndTopicFilter)
	assertLogsContainTxAndAddress(t, logsByTopicBefore, txHash, contractAddress)

	// Restart and verify indexed logs are still queryable.
	firstStartOutput := node.OutputString()
	node.RestartAndWaitRPC()

	logsByAddressAfter := evmtest.MustGetLogs(t, node.RPCURL(), addressFilter)
	assertLogsContainTxAndAddress(t, logsByAddressAfter, txHash, contractAddress)

	logsByTopicAfter := evmtest.MustGetLogs(t, node.RPCURL(), addressAndTopicFilter)
	assertLogsContainTxAndAddress(t, logsByTopicAfter, txHash, contractAddress)

	evmtest.AssertContains(t, firstStartOutput, "Starting EVMIndexerService service")
	evmtest.AssertContains(t, node.OutputString(), "Starting EVMIndexerService service")
}

// assertLogsContainTxAndAddress ensures at least one log entry matches expected
// tx hash and emitting address in a filtered result set.
func assertLogsContainTxAndAddress(t *testing.T, logs []map[string]any, txHash, address string) {
	t.Helper()

	// We only care that at least one matching log entry survived filtering/indexing.
	for _, logEntry := range logs {
		gotTxHash, ok := logEntry["transactionHash"].(string)
		if !ok || !strings.EqualFold(gotTxHash, txHash) {
			continue
		}

		gotAddress, ok := logEntry["address"].(string)
		if !ok || !strings.EqualFold(gotAddress, address) {
			t.Fatalf("log has matching tx hash but unexpected address: %#v", logEntry)
		}
		return
	}

	t.Fatalf("no log found for tx %s and address %s in %#v", txHash, address, logs)
}
