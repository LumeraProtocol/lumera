//go:build integration
// +build integration

package evmtest

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	testjsonrpc "github.com/LumeraProtocol/lumera/testutil/jsonrpc"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// waitForReceipt polls eth_getTransactionReceipt until non-nil receipt appears.
func waitForReceipt(
	t *testing.T,
	rpcURL, txHash string,
	waitCh <-chan error,
	output *bytes.Buffer,
	timeout time.Duration,
) map[string]any {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case err := <-waitCh:
			t.Fatalf("node exited while waiting for receipt (%s): %v\n%s", txHash, err, output.String())
		default:
		}

		var receipt map[string]any
		err := testjsonrpc.Call(context.Background(), rpcURL, "eth_getTransactionReceipt", []any{txHash}, &receipt)
		if err == nil && receipt != nil {
			return receipt
		}
		time.Sleep(400 * time.Millisecond)
	}

	t.Fatalf("receipt not found within %s for tx %s\n%s", timeout, txHash, output.String())
	return nil
}

// waitForTransactionByHash polls eth_getTransactionByHash until tx appears.
func waitForTransactionByHash(
	t *testing.T,
	rpcURL, txHash string,
	waitCh <-chan error,
	output *bytes.Buffer,
	timeout time.Duration,
) map[string]any {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case err := <-waitCh:
			t.Fatalf("node exited while waiting for tx-by-hash (%s): %v\n%s", txHash, err, output.String())
		default:
		}

		var txObj map[string]any
		err := testjsonrpc.Call(context.Background(), rpcURL, "eth_getTransactionByHash", []any{txHash}, &txObj)
		if err == nil && txObj != nil {
			return txObj
		}
		time.Sleep(400 * time.Millisecond)
	}

	t.Fatalf("transaction not found within %s for tx %s\n%s", timeout, txHash, output.String())
	return nil
}

// mustGetBlock fetches a block and fails if the RPC returns nil.
func mustGetBlock(t *testing.T, rpcURL, method string, params []any) map[string]any {
	t.Helper()

	var block map[string]any
	mustJSONRPC(t, rpcURL, method, params, &block)
	if block == nil {
		t.Fatalf("%s returned nil block for params %v", method, params)
	}
	return block
}

// mustGetLogs queries logs with retry to tolerate early post-start readiness races.
func mustGetLogs(t *testing.T, rpcURL string, filter map[string]any) []map[string]any {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		var logs []map[string]any
		err := testjsonrpc.Call(context.Background(), rpcURL, "eth_getLogs", []any{filter}, &logs)
		if err == nil {
			return logs
		}
		lastErr = err
		time.Sleep(300 * time.Millisecond)
	}

	t.Fatalf("failed to query eth_getLogs within timeout: %v", lastErr)
	return nil
}

// assertReceiptMatchesTxHash verifies receipt identity for the expected tx hash.
func assertReceiptMatchesTxHash(t *testing.T, receipt map[string]any, txHash string) {
	t.Helper()

	gotTxHash, ok := receipt["transactionHash"].(string)
	if !ok || strings.TrimSpace(gotTxHash) == "" {
		t.Fatalf("receipt missing transactionHash: %#v", receipt)
	}
	if !strings.EqualFold(gotTxHash, txHash) {
		t.Fatalf("receipt transactionHash mismatch: got %q want %q", gotTxHash, txHash)
	}
}

// assertTxObjectMatchesHash verifies tx object identity for expected hash.
func assertTxObjectMatchesHash(t *testing.T, txObj map[string]any, txHash string) {
	t.Helper()

	gotTxHash, ok := txObj["hash"].(string)
	if !ok || strings.TrimSpace(gotTxHash) == "" {
		t.Fatalf("tx object missing hash: %#v", txObj)
	}
	if !strings.EqualFold(gotTxHash, txHash) {
		t.Fatalf("tx hash mismatch: got %q want %q", gotTxHash, txHash)
	}
}

// assertTxFieldStable asserts a tx field value is unchanged across restart.
func assertTxFieldStable(t *testing.T, field string, before, after map[string]any) {
	t.Helper()

	beforeV, beforeOK := before[field]
	afterV, afterOK := after[field]
	if !beforeOK || !afterOK {
		t.Fatalf("tx field %q missing before/after: before=%#v after=%#v", field, before, after)
	}
	if fmt.Sprint(beforeV) != fmt.Sprint(afterV) {
		t.Fatalf("tx field %q changed across restart: before=%v after=%v", field, beforeV, afterV)
	}
}

// assertBlockContainsTxHash checks hash-only block payload includes the tx hash.
func assertBlockContainsTxHash(t *testing.T, block map[string]any, txHash string) {
	t.Helper()

	txs, ok := block["transactions"].([]any)
	if !ok {
		t.Fatalf("block missing transactions array: %#v", block)
	}
	for _, tx := range txs {
		txStr, ok := tx.(string)
		if ok && strings.EqualFold(txStr, txHash) {
			return
		}
	}
	t.Fatalf("tx %s not found in block transaction hashes: %#v", txHash, txs)
}

// assertBlockContainsFullTx checks full transaction payload includes tx hash.
func assertBlockContainsFullTx(t *testing.T, block map[string]any, txHash string) {
	t.Helper()

	txs, ok := block["transactions"].([]any)
	if !ok {
		t.Fatalf("block missing transactions array: %#v", block)
	}
	for _, tx := range txs {
		txObj, ok := tx.(map[string]any)
		if !ok {
			continue
		}
		hash, ok := txObj["hash"].(string)
		if ok && strings.EqualFold(hash, txHash) {
			return
		}
	}
	t.Fatalf("tx %s not found in full block transactions: %#v", txHash, txs)
}

// mustStringField extracts a non-empty string field from a generic map payload.
func mustStringField(t *testing.T, m map[string]any, field string) string {
	t.Helper()

	v, ok := m[field]
	if !ok {
		t.Fatalf("missing field %q in map: %#v", field, m)
	}
	s, ok := v.(string)
	if !ok || strings.TrimSpace(s) == "" {
		t.Fatalf("field %q is not a non-empty string: %#v", field, v)
	}
	return s
}

// mustUint64HexField parses a `0x` hex numeric field into uint64.
func mustUint64HexField(t *testing.T, m map[string]any, field string) uint64 {
	t.Helper()

	v := mustStringField(t, m, field)
	n, err := hexutil.DecodeUint64(v)
	if err != nil {
		t.Fatalf("failed to decode hex field %q=%q: %v", field, v, err)
	}
	return n
}

// startProcess starts a child process and returns async wait channel + combined output.
func startProcess(t *testing.T, ctx context.Context, workDir, bin string, args ...string) (*exec.Cmd, <-chan error, *bytes.Buffer) {
	t.Helper()

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = workDir
	// Prevent depinject from blocking on D-Bus keyring in WSL2/headless.
	cmd.Env = append(os.Environ(), "LUMERA_KEYRING_BACKEND=test")

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	return cmd, waitCh, &output
}

// stopProcess cancels context and force-kills process on slow shutdown.
func stopProcess(t *testing.T, cancel context.CancelFunc, cmd *exec.Cmd, waitCh <-chan error) {
	t.Helper()

	cancel()
	select {
	case <-waitCh:
		return
	case <-time.After(10 * time.Second):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		<-waitCh
	}
}

// waitForJSONRPC waits until web3_clientVersion returns a non-empty value.
func waitForJSONRPC(t *testing.T, rpcURL string, waitCh <-chan error, output *bytes.Buffer) {
	t.Helper()

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-waitCh:
			t.Fatalf("node exited before json-rpc became ready: %v\n%s", err, output.String())
		default:
		}

		var clientVersion string
		err := testjsonrpc.Call(context.Background(), rpcURL, "web3_clientVersion", []any{}, &clientVersion)
		if err == nil && strings.TrimSpace(clientVersion) != "" {
			return
		}
		time.Sleep(300 * time.Millisecond)
	}

	t.Fatalf("json-rpc server did not become ready in time\n%s", output.String())
}

// mustJSONRPC is a fail-fast wrapper around JSON-RPC calls.
func mustJSONRPC(t *testing.T, rpcURL, method string, params []any, out any) {
	t.Helper()

	if err := testjsonrpc.Call(context.Background(), rpcURL, method, params, out); err != nil {
		t.Fatalf("json-rpc call %s failed: %v", method, err)
	}
}

// mustGetBlockNumber returns latest block number with retry during startup races.
func mustGetBlockNumber(t *testing.T, rpcURL string) uint64 {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		var blockNumberHex string
		err := testjsonrpc.Call(context.Background(), rpcURL, "eth_blockNumber", []any{}, &blockNumberHex)
		if err == nil {
			blockNumber, decodeErr := hexutil.DecodeUint64(blockNumberHex)
			if decodeErr == nil {
				return blockNumber
			}
			lastErr = fmt.Errorf("decode eth_blockNumber %q: %w", blockNumberHex, decodeErr)
		} else {
			lastErr = err
		}

		time.Sleep(300 * time.Millisecond)
	}

	t.Fatalf("failed to query eth_blockNumber within timeout: %v", lastErr)
	return 0
}

// waitForBlockNumberAtLeast blocks until chain height reaches minBlock.
func waitForBlockNumberAtLeast(t *testing.T, rpcURL string, minBlock uint64, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		blockNumber := mustGetBlockNumber(t, rpcURL)
		if blockNumber >= minBlock {
			return
		}
		time.Sleep(300 * time.Millisecond)
	}

	t.Fatalf("block number did not reach %d within %s", minBlock, timeout)
}

// waitForCosmosTxHeight polls `query tx` until tx is indexed and returns block height.
func waitForCosmosTxHeight(t *testing.T, node *evmNode, txHash string, timeout time.Duration) uint64 {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var (
		lastErr      error
		lastScanned  uint64
		searchHeight uint64 = 1
		targetHash          = strings.ToUpper(strings.TrimPrefix(txHash, "0x"))
	)
	for time.Now().Before(deadline) {
		latestHeight := mustGetBlockNumber(t, node.rpcURL)
		for h := searchHeight; h <= latestHeight; h++ {
			txs, err := getCometBlockTxs(node, h)
			if err != nil {
				lastErr = err
				continue
			}

			hashes := cometTxHashesFromBase64(t, txs)
			for _, hash := range hashes {
				if strings.EqualFold(hash, targetHash) {
					return h
				}
			}
			lastScanned = h
		}

		searchHeight = latestHeight + 1
		time.Sleep(300 * time.Millisecond)
	}

	if lastErr != nil {
		t.Fatalf("failed to query cosmos tx %s within %s (scanned through height %d): %v", txHash, timeout, lastScanned, lastErr)
	}
	t.Fatalf("cosmos tx %s was not included within %s (scanned through height %d)", txHash, timeout, lastScanned)
	return 0
}

// mustGetCometBlockTxs returns `block.data.txs` base64 entries for a block height.
func mustGetCometBlockTxs(t *testing.T, node *evmNode, height uint64) []string {
	t.Helper()

	txs, err := getCometBlockTxs(node, height)
	if err != nil {
		t.Fatalf("query block %d failed: %v", height, err)
	}
	return txs
}

func getCometBlockTxs(node *evmNode, height uint64) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	out, err := run(ctx, node.repoRoot, node.binPath,
		"query", "block", "--type=height", strconv.FormatUint(height, 10),
		"--node", node.cometRPCURL,
		"--output", "json",
		"--log_no_color",
	)
	if err != nil {
		return nil, fmt.Errorf("query block command failed: %w: %s", err, out)
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return nil, fmt.Errorf("decode query block response: %w: %s", err, out)
	}

	// CLI output shape differs across SDK/Comet versions:
	// - old: {"block": {...}}
	// - new: {"header": {...}, "data": {...}, ...}
	block, ok := resp["block"].(map[string]any)
	if !ok {
		if _, hasData := resp["data"]; hasData {
			block = resp
		} else {
			return nil, fmt.Errorf("missing block in query response: %#v", resp)
		}
	}
	data, ok := block["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing block.data in query response: %#v", block)
	}
	txsRaw, ok := data["txs"].([]any)
	if !ok {
		return nil, fmt.Errorf("missing block.data.txs in query response: %#v", data)
	}

	txs := make([]string, 0, len(txsRaw))
	for _, tx := range txsRaw {
		txB64, ok := tx.(string)
		if !ok || strings.TrimSpace(txB64) == "" {
			return nil, fmt.Errorf("invalid block tx entry: %#v", tx)
		}
		txs = append(txs, txB64)
	}

	return txs, nil
}

// cometTxHashesFromBase64 computes Comet tx hashes (upper hex) from base64 tx bytes.
func cometTxHashesFromBase64(t *testing.T, txs []string) []string {
	t.Helper()

	hashes := make([]string, 0, len(txs))
	for _, txB64 := range txs {
		txBz, err := base64.StdEncoding.DecodeString(txB64)
		if err != nil {
			t.Fatalf("decode block tx base64: %v", err)
		}
		hashes = append(hashes, strings.ToUpper(hex.EncodeToString(cmttypes.Tx(txBz).Hash())))
	}

	return hashes
}

// freePort reserves one ephemeral local TCP port.
func freePort(t *testing.T) int {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to allocate port: %v", err)
	}
	defer l.Close()

	return l.Addr().(*net.TCPAddr).Port
}

// mustFindRepoRoot walks upward from CWD until go.mod is found.
func mustFindRepoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	t.Fatal("could not find repo root (go.mod)")
	return ""
}

// mustRun executes command with timeout and fails test on non-zero exit.
func mustRun(t *testing.T, workDir string, timeout time.Duration, bin string, args ...string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	out, err := run(ctx, workDir, bin, args...)
	if err != nil {
		t.Fatalf("command failed: %s %s: %v\n%s", bin, strings.Join(args, " "), err, out)
	}
	return out
}

// run executes command and returns merged stdout/stderr plus error.
func run(ctx context.Context, workDir, bin string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = workDir
	// Prevent depinject from blocking on D-Bus keyring in WSL2/headless.
	cmd.Env = append(os.Environ(), "LUMERA_KEYRING_BACKEND=test")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
	if err != nil && ctx.Err() != nil {
		return output, fmt.Errorf("%w: %v", ctx.Err(), err)
	}
	return output, err
}

// assertContains is a tiny helper for log-output assertions.
func assertContains(t *testing.T, output, needle string) {
	t.Helper()

	if strings.Contains(output, needle) {
		return
	}

	t.Fatalf("expected output to contain %q\n%s", needle, output)
}
