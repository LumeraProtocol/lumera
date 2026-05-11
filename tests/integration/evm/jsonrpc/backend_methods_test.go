//go:build integration
// +build integration

package jsonrpc_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// TestBackendBlockCountAndUncleSemantics validates backend-facing block count
// and uncle APIs on a committed EVM transaction.
//
// Coverage matrix:
//  1. `eth_getBlockTransactionCountByHash` / `...ByNumber` return the same
//     transaction count as block payload lookup.
//  2. Missing block selectors return `null` for tx-count methods.
//  3. Uncle methods keep CometBFT semantics (no uncles).
func testBackendBlockCountAndUncleSemantics(t *testing.T, node *evmtest.Node) {
	t.Helper()

	txHash := node.SendOneLegacyTx(t)
	receipt := node.WaitForReceipt(t, txHash, 40*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)

	blockHash := evmtest.MustStringField(t, receipt, "blockHash")
	blockNumber := evmtest.MustStringField(t, receipt, "blockNumber")

	// Cross-check tx-count methods against block payload transaction array length.
	blockByHash := node.MustGetBlock(t, "eth_getBlockByHash", []any{blockHash, false})
	blockTxs, ok := blockByHash["transactions"].([]any)
	if !ok {
		t.Fatalf("unexpected transactions payload in block: %#v", blockByHash["transactions"])
	}

	var countByHashHex string
	node.MustJSONRPC(t, "eth_getBlockTransactionCountByHash", []any{blockHash}, &countByHashHex)
	countByHash := mustDecodeHexUint64(t, countByHashHex, "countByHash")

	var countByNumberHex string
	node.MustJSONRPC(t, "eth_getBlockTransactionCountByNumber", []any{blockNumber}, &countByNumberHex)
	countByNumber := mustDecodeHexUint64(t, countByNumberHex, "countByNumber")

	if countByHash != uint64(len(blockTxs)) {
		t.Fatalf("tx count by hash mismatch: got=%d want=%d", countByHash, len(blockTxs))
	}
	if countByNumber != countByHash {
		t.Fatalf("tx count by number/hash mismatch: byNumber=%d byHash=%d", countByNumber, countByHash)
	}

	// Unknown blocks should produce `null` (decoded as nil interface).
	const missingHash = "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	var missingCountByHash any
	node.MustJSONRPC(t, "eth_getBlockTransactionCountByHash", []any{missingHash}, &missingCountByHash)
	if missingCountByHash != nil {
		t.Fatalf("expected nil tx count for missing hash, got %#v", missingCountByHash)
	}

	var missingCountByNumber any
	node.MustJSONRPC(t, "eth_getBlockTransactionCountByNumber", []any{"0x7fffffff"}, &missingCountByNumber)
	if missingCountByNumber != nil {
		t.Fatalf("expected nil tx count for missing number, got %#v", missingCountByNumber)
	}

	// CometBFT backend never has uncles.
	var uncleCountByHashHex string
	node.MustJSONRPC(t, "eth_getUncleCountByBlockHash", []any{blockHash}, &uncleCountByHashHex)
	if mustDecodeHexUint64(t, uncleCountByHashHex, "uncleCountByHash") != 0 {
		t.Fatalf("expected zero uncle count by hash, got %s", uncleCountByHashHex)
	}

	var uncleCountByNumberHex string
	node.MustJSONRPC(t, "eth_getUncleCountByBlockNumber", []any{blockNumber}, &uncleCountByNumberHex)
	if mustDecodeHexUint64(t, uncleCountByNumberHex, "uncleCountByNumber") != 0 {
		t.Fatalf("expected zero uncle count by number, got %s", uncleCountByNumberHex)
	}

	var uncleByHash any
	node.MustJSONRPC(t, "eth_getUncleByBlockHashAndIndex", []any{blockHash, "0x0"}, &uncleByHash)
	if uncleByHash != nil {
		t.Fatalf("expected nil uncle by hash+index, got %#v", uncleByHash)
	}

	var uncleByNumber any
	node.MustJSONRPC(t, "eth_getUncleByBlockNumberAndIndex", []any{blockNumber, "0x0"}, &uncleByNumber)
	if uncleByNumber != nil {
		t.Fatalf("expected nil uncle by number+index, got %#v", uncleByNumber)
	}
}

// TestBackendNetAndWeb3UtilityMethods checks utility RPC namespaces that are
// served via the backend wiring.
//
// Coverage matrix:
// 1. `net_listening` returns a healthy boolean.
// 2. `net_peerCount` is parseable and non-negative.
// 3. `web3_sha3` is deterministic and returns 32-byte hashes.
func testBackendNetAndWeb3UtilityMethods(t *testing.T, node *evmtest.Node) {
	t.Helper()

	var listening bool
	node.MustJSONRPC(t, "net_listening", []any{}, &listening)
	if !listening {
		t.Fatalf("expected net_listening=true on started local node")
	}

	// Keep parsing flexible for backend variations (numeric JSON or quantity hex string).
	var peerCountRaw any
	node.MustJSONRPC(t, "net_peerCount", []any{}, &peerCountRaw)
	peerCount := mustParsePeerCount(t, peerCountRaw)
	if peerCount < 0 {
		t.Fatalf("peer count must be non-negative, got %d", peerCount)
	}

	payloadA := "lumera-rpc-backend"
	payloadB := "lumera-rpc-backend-2"

	var hashA1 string
	node.MustJSONRPC(t, "web3_sha3", []any{payloadA}, &hashA1)
	mustAssertHexBytesLen(t, hashA1, 32, "web3_sha3(payloadA)")

	var hashA2 string
	node.MustJSONRPC(t, "web3_sha3", []any{payloadA}, &hashA2)
	if !strings.EqualFold(hashA1, hashA2) {
		t.Fatalf("web3_sha3 must be deterministic: first=%s second=%s", hashA1, hashA2)
	}

	var hashB string
	node.MustJSONRPC(t, "web3_sha3", []any{payloadB}, &hashB)
	mustAssertHexBytesLen(t, hashB, 32, "web3_sha3(payloadB)")
	if strings.EqualFold(hashA1, hashB) {
		t.Fatalf("web3_sha3 should differ for different payloads: A=%s B=%s", hashA1, hashB)
	}
}

func mustDecodeHexUint64(t *testing.T, hexValue string, field string) uint64 {
	t.Helper()

	n, err := hexutil.DecodeUint64(strings.TrimSpace(hexValue))
	if err != nil {
		t.Fatalf("decode %s %q: %v", field, hexValue, err)
	}
	return n
}

func mustParsePeerCount(t *testing.T, v any) int64 {
	t.Helper()

	switch typed := v.(type) {
	case float64:
		return int64(typed)
	case string:
		s := strings.TrimSpace(typed)
		if strings.HasPrefix(strings.ToLower(s), "0x") {
			return int64(mustDecodeHexUint64(t, s, "net_peerCount"))
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			t.Fatalf("parse net_peerCount %q: %v", s, err)
		}
		return n
	default:
		t.Fatalf("unexpected net_peerCount type %T (%#v)", v, v)
		return 0
	}
}

func mustAssertHexBytesLen(t *testing.T, value string, wantBytes int, field string) {
	t.Helper()

	decoded, err := hexutil.Decode(strings.TrimSpace(value))
	if err != nil {
		t.Fatalf("decode %s %q: %v", field, value, err)
	}
	if len(decoded) != wantBytes {
		t.Fatalf("unexpected %s byte length: got=%d want=%d", field, len(decoded), wantBytes)
	}
}
