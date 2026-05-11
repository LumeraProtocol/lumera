//go:build integration
// +build integration

package jsonrpc_test

import (
	"context"
	"errors"
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	"strings"
	"testing"
	"time"
)

// TestIndexerStartupSmoke is a short-lived process smoke test for JSON-RPC,
// websocket RPC, and indexer startup logs.
func TestIndexerStartupSmoke(t *testing.T) {
	t.Helper()

	// Short-run smoke: verify JSON-RPC + indexer services boot without panic.
	node := evmtest.NewEVMNode(t, "lumera-smoke", 3)

	startCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	startOutput, startErr := evmtest.RunCommand(startCtx, node.RepoRoot(), node.BinPath(), node.StartArgs()...)

	timedOut := errors.Is(startCtx.Err(), context.DeadlineExceeded)
	if startErr != nil && !timedOut {
		t.Fatalf("start failed: %v\n%s", startErr, startOutput)
	}

	evmtest.AssertContains(t, startOutput, "Starting JSON-RPC server")
	evmtest.AssertContains(t, startOutput, "Starting JSON WebSocket server")
	evmtest.AssertContains(t, startOutput, "Starting EVMIndexerService service")

	if strings.Contains(startOutput, "panic:") {
		t.Fatalf("unexpected panic during start:\n%s", startOutput)
	}
	if strings.Contains(startOutput, "error initializing evm coin info") {
		t.Fatalf("unexpected EVM coin info init failure:\n%s", startOutput)
	}
}
