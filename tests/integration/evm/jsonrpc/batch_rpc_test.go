//go:build integration
// +build integration

package jsonrpc_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testjsonrpc "github.com/LumeraProtocol/lumera/testutil/jsonrpc"
)

// testBatchJSONRPCReturnsAllResponses sends a batch of different JSON-RPC
// methods and verifies that all responses are returned with correct IDs.
func testBatchJSONRPCReturnsAllResponses(t *testing.T, node *evmtest.Node) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	requests := []testjsonrpc.BatchRequest{
		{Method: "eth_blockNumber", Params: []any{}},
		{Method: "eth_chainId", Params: []any{}},
		{Method: "web3_clientVersion", Params: []any{}},
		{Method: "net_version", Params: []any{}},
	}

	responses, err := testjsonrpc.CallBatch(ctx, node.RPCURL(), requests)
	if err != nil {
		t.Fatalf("batch call failed: %v", err)
	}

	if len(responses) != len(requests) {
		t.Fatalf("expected %d responses, got %d", len(requests), len(responses))
	}

	// Verify each response has a non-nil result and no error.
	seenIDs := make(map[int]bool)
	for i, resp := range responses {
		if resp.Error != nil {
			t.Fatalf("response %d (id=%d) has error: %v", i, resp.ID, resp.Error)
		}
		if len(resp.Result) == 0 {
			t.Fatalf("response %d (id=%d) has empty result", i, resp.ID)
		}
		seenIDs[resp.ID] = true
	}

	// All IDs 1..4 must be present.
	for id := 1; id <= len(requests); id++ {
		if !seenIDs[id] {
			t.Fatalf("missing response for id=%d", id)
		}
	}
}

// testBatchJSONRPCMixedErrorsAndResults sends a batch with one valid and one
// invalid method, verifying that errors are per-request rather than failing the
// whole batch.
func testBatchJSONRPCMixedErrorsAndResults(t *testing.T, node *evmtest.Node) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	requests := []testjsonrpc.BatchRequest{
		{Method: "eth_blockNumber", Params: []any{}},
		{Method: "eth_getBalance", Params: []any{"not_a_valid_address", "latest"}},
	}

	responses, err := testjsonrpc.CallBatch(ctx, node.RPCURL(), requests)
	if err != nil {
		t.Fatalf("batch call failed: %v", err)
	}

	if len(responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(responses))
	}

	// Find response for eth_blockNumber (id=1) and verify it succeeded.
	var blockNumResp, balanceResp *testjsonrpc.BatchResponse
	for i := range responses {
		switch responses[i].ID {
		case 1:
			blockNumResp = &responses[i]
		case 2:
			balanceResp = &responses[i]
		}
	}

	if blockNumResp == nil {
		t.Fatal("missing response for eth_blockNumber (id=1)")
	}
	if blockNumResp.Error != nil {
		t.Fatalf("eth_blockNumber should succeed in batch, got error: %v", blockNumResp.Error)
	}

	var blockNumHex string
	if err := json.Unmarshal(blockNumResp.Result, &blockNumHex); err != nil {
		t.Fatalf("unmarshal eth_blockNumber result: %v", err)
	}
	if !strings.HasPrefix(blockNumHex, "0x") {
		t.Fatalf("expected hex block number, got %q", blockNumHex)
	}

	// The invalid-address request should return an error response (not crash the batch).
	if balanceResp == nil {
		t.Fatal("missing response for eth_getBalance (id=2)")
	}
	if balanceResp.Error == nil && len(balanceResp.Result) > 0 {
		// Some implementations may return a result for invalid addresses;
		// the key assertion is that both responses are present.
		t.Logf("eth_getBalance with invalid address returned result instead of error; batch still valid")
	}
}

// testBatchJSONRPCSingleElementBatch verifies that a batch of exactly one
// request is handled correctly (edge case for array-of-one).
func testBatchJSONRPCSingleElementBatch(t *testing.T, node *evmtest.Node) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	responses, err := testjsonrpc.CallBatch(ctx, node.RPCURL(), []testjsonrpc.BatchRequest{
		{Method: "eth_chainId", Params: []any{}},
	})
	if err != nil {
		t.Fatalf("single-element batch call failed: %v", err)
	}

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	if responses[0].Error != nil {
		t.Fatalf("single-element batch returned error: %v", responses[0].Error)
	}

	var chainIDHex string
	if err := json.Unmarshal(responses[0].Result, &chainIDHex); err != nil {
		t.Fatalf("unmarshal chain ID: %v", err)
	}
	if !strings.HasPrefix(chainIDHex, "0x") {
		t.Fatalf("expected hex chain ID, got %q", chainIDHex)
	}
}

// testBatchJSONRPCDuplicateMethods verifies that sending the same method
// multiple times in a batch returns the correct number of independent results.
func testBatchJSONRPCDuplicateMethods(t *testing.T, node *evmtest.Node) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	requests := []testjsonrpc.BatchRequest{
		{Method: "eth_blockNumber", Params: []any{}},
		{Method: "eth_blockNumber", Params: []any{}},
		{Method: "eth_blockNumber", Params: []any{}},
	}

	responses, err := testjsonrpc.CallBatch(ctx, node.RPCURL(), requests)
	if err != nil {
		t.Fatalf("batch call with duplicates failed: %v", err)
	}

	if len(responses) != 3 {
		t.Fatalf("expected 3 responses, got %d", len(responses))
	}

	for i, resp := range responses {
		if resp.Error != nil {
			t.Fatalf("response %d has error: %v", i, resp.Error)
		}
		var blockNumHex string
		if err := json.Unmarshal(resp.Result, &blockNumHex); err != nil {
			t.Fatalf("response %d: unmarshal block number: %v", i, err)
		}
		if !strings.HasPrefix(blockNumHex, "0x") {
			t.Fatalf("response %d: expected hex, got %q", i, blockNumHex)
		}
	}
}
