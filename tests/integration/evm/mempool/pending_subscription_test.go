//go:build integration
// +build integration

package mempool_test

import (
	"context"
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
)

// TestPendingTxSubscriptionEmitsHash verifies WS pending subscription path by
// matching emitted hash with a newly broadcast transaction.
func testPendingTxSubscriptionEmitsHash(t *testing.T, node *evmtest.Node) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	wsClient, err := rpc.DialContext(ctx, node.WSURL())
	if err != nil {
		t.Fatalf("dial websocket rpc: %v", err)
	}
	defer wsClient.Close()

	pendingHashes := make(chan common.Hash, 32)
	sub, err := wsClient.EthSubscribe(ctx, pendingHashes, "newPendingTransactions")
	if err != nil {
		t.Fatalf("eth_subscribe newPendingTransactions: %v", err)
	}
	defer sub.Unsubscribe()

	txHash := evmtest.SendOneLegacyTx(t, node.RPCURL(), node.KeyInfo())

	deadline := time.After(20 * time.Second)
	for {
		select {
		case err := <-sub.Err():
			t.Fatalf("pending subscription failed: %v", err)
		case hash := <-pendingHashes:
			if strings.EqualFold(hash.Hex(), txHash) {
				evmtest.WaitForReceipt(t, node.RPCURL(), txHash, node.WaitCh(), node.OutputBuffer(), 40*time.Second)
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for pending tx hash %s", txHash)
		}
	}
}
