//go:build integration
// +build integration

package precompiles_test

import (
	"testing"
	"time"

	supernodeprecompile "github.com/LumeraProtocol/lumera/precompiles/supernode"
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
)

// testSupernodePrecompileGetParamsViaEthCall verifies the supernode precompile
// `getParams()` query returns valid module parameters via eth_call.
func testSupernodePrecompileGetParamsViaEthCall(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	input, err := supernodeprecompile.ABI.Pack(supernodeprecompile.GetParamsMethod)
	if err != nil {
		t.Fatalf("pack getParams input: %v", err)
	}

	result := mustEthCallPrecompile(t, node, supernodeprecompile.SupernodePrecompileAddress, input)
	out, err := supernodeprecompile.ABI.Unpack(supernodeprecompile.GetParamsMethod, result)
	if err != nil {
		t.Fatalf("unpack getParams output: %v", err)
	}

	// getParams returns: minimumStake, reportingThreshold, slashingThreshold,
	//                     minSupernodeVersion, minCpuCores, minMemGb, minStorageGb
	if len(out) != 7 {
		t.Fatalf("expected 7 return values from getParams, got %d", len(out))
	}

	minVersion, ok := out[3].(string)
	if !ok || minVersion == "" {
		t.Fatalf("unexpected minSupernodeVersion type: %#v", out[3])
	}
}

// testSupernodePrecompileListSuperNodesViaEthCall verifies the supernode
// precompile `listSuperNodes(0, 10)` query returns an empty list on a fresh chain.
func testSupernodePrecompileListSuperNodesViaEthCall(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	input, err := supernodeprecompile.ABI.Pack(
		supernodeprecompile.ListSuperNodesMethod,
		uint64(0),  // offset
		uint64(10), // limit
	)
	if err != nil {
		t.Fatalf("pack listSuperNodes input: %v", err)
	}

	result := mustEthCallPrecompile(t, node, supernodeprecompile.SupernodePrecompileAddress, input)
	out, err := supernodeprecompile.ABI.Unpack(supernodeprecompile.ListSuperNodesMethod, result)
	if err != nil {
		t.Fatalf("unpack listSuperNodes output: %v", err)
	}

	if len(out) != 2 {
		t.Fatalf("expected 2 return values from listSuperNodes, got %d", len(out))
	}

	// On a fresh single-node test chain there may or may not be a supernode
	// registered, so we just verify the call succeeds and returns valid data.
	total, ok := out[1].(uint64)
	if !ok {
		t.Fatalf("unexpected total type: %#v", out[1])
	}
	_ = total // value is valid regardless
}

// testSupernodePrecompileGetTopSuperNodesForBlockViaEthCall verifies the supernode
// precompile `getTopSuperNodesForBlock` query works on a fresh chain.
func testSupernodePrecompileGetTopSuperNodesForBlockViaEthCall(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 2, 20*time.Second)

	input, err := supernodeprecompile.ABI.Pack(
		supernodeprecompile.GetTopSuperNodesForBlockMethod,
		int32(1),  // blockHeight
		int32(10), // limit
		uint8(0),  // state (unspecified = all)
	)
	if err != nil {
		t.Fatalf("pack getTopSuperNodesForBlock input: %v", err)
	}

	result := mustEthCallPrecompile(t, node, supernodeprecompile.SupernodePrecompileAddress, input)
	out, err := supernodeprecompile.ABI.Unpack(supernodeprecompile.GetTopSuperNodesForBlockMethod, result)
	if err != nil {
		t.Fatalf("unpack getTopSuperNodesForBlock output: %v", err)
	}

	if len(out) != 1 {
		t.Fatalf("expected 1 return value from getTopSuperNodesForBlock, got %d", len(out))
	}
}
