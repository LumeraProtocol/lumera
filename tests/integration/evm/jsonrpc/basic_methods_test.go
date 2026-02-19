//go:build integration
// +build integration

package jsonrpc_test

import (
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	"strconv"
	"strings"
	"testing"
)

// TestBasicRPCMethods is a startup/readiness sanity test for core identity APIs.
//
// Workflow:
// 1. Start node and wait for JSON-RPC readiness.
// 2. Validate chain/network identity endpoints.
// 3. Assert indexer + JSON-RPC services were started without panic.
func testBasicRPCMethods(t *testing.T, node *evmtest.Node) {
	t.Helper()

	// Validate identity endpoints exposed by the EVM JSON-RPC server.
	var chainIDHex string
	evmtest.MustJSONRPC(t, node.RPCURL(), "eth_chainId", []any{}, &chainIDHex)
	expectedChainIDHex := "0x" + strconv.FormatUint(evmtest.EVMChainID, 16)
	if strings.ToLower(chainIDHex) != strings.ToLower(expectedChainIDHex) {
		t.Fatalf("unexpected eth_chainId: got %q want %q", chainIDHex, expectedChainIDHex)
	}

	var netVersion string
	evmtest.MustJSONRPC(t, node.RPCURL(), "net_version", []any{}, &netVersion)
	expectedNetVersion := strconv.FormatUint(evmtest.EVMChainID, 10)
	if netVersion != expectedNetVersion {
		t.Fatalf("unexpected net_version: got %q want %q", netVersion, expectedNetVersion)
	}

	var clientVersion string
	evmtest.MustJSONRPC(t, node.RPCURL(), "web3_clientVersion", []any{}, &clientVersion)
	if strings.TrimSpace(clientVersion) == "" {
		t.Fatalf("web3_clientVersion returned empty value")
	}

	// Basic sanity checks to catch early boot/runtime regressions.
	if strings.Contains(node.OutputString(), "panic:") {
		t.Fatalf("unexpected panic while node running:\n%s", node.OutputString())
	}

	evmtest.AssertContains(t, node.OutputString(), "Starting EVMIndexerService service")
	evmtest.AssertContains(t, node.OutputString(), "Starting JSON-RPC server")
}
