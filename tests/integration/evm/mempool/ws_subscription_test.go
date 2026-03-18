//go:build integration
// +build integration

package mempool_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	"github.com/ethereum/go-ethereum/rpc"
)

// testNewHeadsSubscriptionEmitsBlocks subscribes to newHeads and verifies that
// at least one block header is emitted with the expected fields.
func testNewHeadsSubscriptionEmitsBlocks(t *testing.T, node *evmtest.Node) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	wsClient, err := rpc.DialContext(ctx, node.WSURL())
	if err != nil {
		t.Fatalf("dial websocket rpc: %v", err)
	}
	defer wsClient.Close()

	headers := make(chan json.RawMessage, 16)
	sub, err := wsClient.Subscribe(ctx, "eth", headers, "newHeads")
	if err != nil {
		t.Fatalf("eth_subscribe newHeads: %v", err)
	}
	defer sub.Unsubscribe()

	// Wait for at least one block header.
	select {
	case err := <-sub.Err():
		t.Fatalf("newHeads subscription error: %v", err)
	case raw := <-headers:
		var header map[string]any
		if err := json.Unmarshal(raw, &header); err != nil {
			t.Fatalf("unmarshal block header: %v", err)
		}

		// Verify essential header fields are present.
		for _, field := range []string{"number", "hash", "parentHash", "timestamp"} {
			v, ok := header[field].(string)
			if !ok || strings.TrimSpace(v) == "" {
				t.Fatalf("block header missing or empty field %q: %#v", field, header)
			}
		}

		number, ok := header["number"].(string)
		if !ok || !strings.HasPrefix(number, "0x") {
			t.Fatalf("block number not hex-encoded: %v", number)
		}

		t.Logf("received newHeads block %s hash %s", number, header["hash"])
	case <-time.After(25 * time.Second):
		t.Fatal("timed out waiting for newHeads block header")
	}
}

// testLogsSubscriptionEmitsEvents subscribes to logs for a deployed contract
// and verifies that the emitted event is delivered via WebSocket.
func testLogsSubscriptionEmitsEvents(t *testing.T, node *evmtest.Node) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	wsClient, err := rpc.DialContext(ctx, node.WSURL())
	if err != nil {
		t.Fatalf("dial websocket rpc: %v", err)
	}
	defer wsClient.Close()

	// Subscribe to all logs (no address filter) before broadcasting.
	logs := make(chan json.RawMessage, 32)
	sub, err := wsClient.Subscribe(ctx, "eth", logs, "logs", map[string]any{})
	if err != nil {
		t.Fatalf("eth_subscribe logs: %v", err)
	}
	defer sub.Unsubscribe()

	// Deploy a tiny contract that emits LOG1 during creation.
	topicHex := "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	txHash := node.SendLogEmitterCreationTx(t, topicHex)
	receipt := node.WaitForReceipt(t, txHash, 40*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)

	// Wait for the log event via WebSocket.
	deadline := time.After(20 * time.Second)
	for {
		select {
		case err := <-sub.Err():
			t.Fatalf("logs subscription error: %v", err)
		case raw := <-logs:
			var logEntry map[string]any
			if err := json.Unmarshal(raw, &logEntry); err != nil {
				t.Fatalf("unmarshal log entry: %v", err)
			}

			logTxHash, ok := logEntry["transactionHash"].(string)
			if ok && strings.EqualFold(logTxHash, txHash) {
				topics, ok := logEntry["topics"].([]any)
				if !ok || len(topics) == 0 {
					t.Fatalf("log entry has no topics: %#v", logEntry)
				}
				t.Logf("received log event for tx %s with %d topics", txHash, len(topics))
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for log event from tx %s", txHash)
		}
	}
}

// testNewHeadsSubscriptionMultipleBlocks subscribes to newHeads and verifies
// that block numbers are monotonically increasing across 3 consecutive headers.
func testNewHeadsSubscriptionMultipleBlocks(t *testing.T, node *evmtest.Node) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	wsClient, err := rpc.DialContext(ctx, node.WSURL())
	if err != nil {
		t.Fatalf("dial websocket rpc: %v", err)
	}
	defer wsClient.Close()

	headers := make(chan json.RawMessage, 16)
	sub, err := wsClient.Subscribe(ctx, "eth", headers, "newHeads")
	if err != nil {
		t.Fatalf("eth_subscribe newHeads: %v", err)
	}
	defer sub.Unsubscribe()

	const wantBlocks = 3
	var prevNumber uint64
	for i := 0; i < wantBlocks; i++ {
		select {
		case err := <-sub.Err():
			t.Fatalf("newHeads subscription error on block %d: %v", i, err)
		case raw := <-headers:
			var header map[string]any
			if err := json.Unmarshal(raw, &header); err != nil {
				t.Fatalf("unmarshal header %d: %v", i, err)
			}

			numHex, ok := header["number"].(string)
			if !ok || !strings.HasPrefix(numHex, "0x") {
				t.Fatalf("block %d: invalid number field: %v", i, header["number"])
			}

			var num uint64
			if _, err := json.Number(numHex).Int64(); err != nil {
				// Parse as hex manually.
				for _, c := range numHex[2:] {
					num = num*16 + uint64(hexVal(c))
				}
			}

			if i > 0 && num <= prevNumber {
				t.Fatalf("block numbers not monotonically increasing: prev=%d current=%d", prevNumber, num)
			}
			prevNumber = num
		case <-time.After(30 * time.Second):
			t.Fatalf("timed out waiting for block %d of %d", i+1, wantBlocks)
		}
	}

	t.Logf("received %d consecutive blocks with monotonically increasing numbers", wantBlocks)
}

// hexVal returns the numeric value of a hex digit character.
func hexVal(c rune) uint64 {
	switch {
	case c >= '0' && c <= '9':
		return uint64(c - '0')
	case c >= 'a' && c <= 'f':
		return uint64(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return uint64(c-'A') + 10
	default:
		return 0
	}
}
