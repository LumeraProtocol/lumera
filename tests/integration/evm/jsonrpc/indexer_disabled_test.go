//go:build integration
// +build integration

package jsonrpc_test

import (
	"context"
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	"strings"
	"testing"
	"time"

	testjsonrpc "github.com/LumeraProtocol/lumera/testutil/jsonrpc"
)

// TestIndexerDisabledLookupUnavailable verifies that tx/receipt lookups are
// unavailable when both EVM and Comet indexers are explicitly disabled.
func TestIndexerDisabledLookupUnavailable(t *testing.T) {
	t.Helper()

	// Disable both EVM indexer and Comet tx indexer to avoid lookup fallbacks.
	node := evmtest.NewEVMNode(t, "lumera-indexer-disabled", 240)
	evmtest.SetIndexerEnabledInAppToml(t, node.HomeDir(), false)
	evmtest.SetCometTxIndexer(t, node.HomeDir(), "null")
	node.StartAndWaitRPC()
	defer node.Stop()

	startBlock := evmtest.MustGetBlockNumber(t, node.RPCURL())
	txHash := evmtest.SendOneLegacyTx(t, node.RPCURL(), node.KeyInfo())
	evmtest.WaitForBlockNumberAtLeast(t, node.RPCURL(), startBlock+2, 30*time.Second)

	// Receipt/tx-by-hash should be unavailable when indexers are disabled.
	assertLookupNilOrError(t, node.RPCURL(), "eth_getTransactionReceipt", []any{txHash})
	assertLookupNilOrError(t, node.RPCURL(), "eth_getTransactionByHash", []any{txHash})

	if strings.Contains(node.OutputString(), "Starting EVMIndexerService service") {
		t.Fatalf("EVM indexer service unexpectedly started while disabled:\n%s", node.OutputString())
	}
}

// assertLookupNilOrError accepts either a transport error or nil result, as
// upstream behavior varies by version when indexers are off.
func assertLookupNilOrError(t *testing.T, rpcURL, method string, params []any) {
	t.Helper()

	// Accept either RPC error or nil result depending on upstream behavior.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var out map[string]any
		err := testjsonrpc.Call(context.Background(), rpcURL, method, params, &out)
		if err != nil {
			return
		}
		if out == nil {
			return
		}

		t.Fatalf("expected %s to return nil or error with indexer disabled, got %#v", method, out)
	}

	t.Fatalf("timed out waiting for %s nil/error behavior", method)
}
