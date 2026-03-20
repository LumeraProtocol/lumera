//go:build integration
// +build integration

package mempool_test

import (
	"context"
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testjsonrpc "github.com/LumeraProtocol/lumera/testutil/jsonrpc"
	"testing"
	"time"
)

// TestMempoolDisabledWithJSONRPCFailsFast verifies txpool RPC surfaces empty
// state when app-side mempool is disabled.
func TestMempoolDisabledWithJSONRPCFailsFast(t *testing.T) {
	t.Helper()

	// App-side mempool can be disabled while JSON-RPC remains available.
	// txpool namespace should return empty state in this mode.
	node := evmtest.NewEVMNode(t, "lumera-mempool-disabled", 20)
	node.AppendStartArgs("--mempool.max-txs", "-1")
	node.AppendStartArgs("--json-rpc.api", "eth,txpool,net,web3")
	node.StartAndWaitRPC()
	defer node.Stop()

	status := mustTxPoolStatusWithRetry(t, node.RPCURL(), 20*time.Second)
	if status["pending"] != "0x0" || status["queued"] != "0x0" {
		t.Fatalf("expected empty txpool status with mempool disabled, got: %+v", status)
	}

	content := mustTxPoolContentWithRetry(t, node.RPCURL(), 20*time.Second)
	if len(content["pending"]) != 0 || len(content["queued"]) != 0 {
		t.Fatalf("expected empty txpool content with mempool disabled, got: %+v", content)
	}

	// Keep a tiny readiness wait so the test still exercises the startup path.
	time.Sleep(250 * time.Millisecond)
}

// mustTxPoolStatusWithRetry polls txpool_status until node startup races settle.
func mustTxPoolStatusWithRetry(t *testing.T, rpcURL string, timeout time.Duration) map[string]string {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		var status map[string]string
		err := testjsonrpc.Call(context.Background(), rpcURL, "txpool_status", []any{}, &status)
		if err == nil {
			return status
		}
		lastErr = err
		time.Sleep(400 * time.Millisecond)
	}

	t.Fatalf("failed to query txpool_status in %s: %v", timeout, lastErr)
	return nil
}

// mustTxPoolContentWithRetry polls txpool_content until a response is available.
func mustTxPoolContentWithRetry(t *testing.T, rpcURL string, timeout time.Duration) map[string]map[string]map[string]any {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		var content map[string]map[string]map[string]any
		err := testjsonrpc.Call(context.Background(), rpcURL, "txpool_content", []any{}, &content)
		if err == nil {
			return content
		}
		lastErr = err
		time.Sleep(400 * time.Millisecond)
	}

	t.Fatalf("failed to query txpool_content in %s: %v", timeout, lastErr)
	return nil
}
