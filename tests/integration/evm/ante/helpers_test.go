//go:build integration
// +build integration

package ante_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
)

// mustAddKeyAddress creates a local keyring key and returns its bech32 address.
func mustAddKeyAddress(t *testing.T, node *evmtest.Node, keyName string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	out, err := evmtest.RunCommand(
		ctx,
		node.RepoRoot(),
		node.BinPath(),
		"keys", "add", keyName,
		"--home", node.HomeDir(),
		"--keyring-backend", "test",
		"--output", "json",
		"--log_no_color",
	)
	if err != nil {
		t.Fatalf("keys add %s failed: %v\n%s", keyName, err, out)
	}

	var keyInfo testaccounts.TestKeyInfo
	if err := json.Unmarshal([]byte(out), &keyInfo); err != nil {
		t.Fatalf("decode keys add output: %v\n%s", err, out)
	}
	testaccounts.MustNormalizeAndValidateTestKeyInfo(t, &keyInfo)

	return keyInfo.Address
}

// mustBroadcastBankSend runs `tx bank send` with explicit fees and returns the
// parsed CLI JSON response.
func mustBroadcastBankSend(t *testing.T, node *evmtest.Node, to, amount, fees string) map[string]any {
	t.Helper()

	return mustBroadcastTxCommand(
		t,
		node,
		"tx", "bank", "send", "validator", to, amount,
		"--home", node.HomeDir(),
		"--keyring-backend", "test",
		"--chain-id", node.ChainID(),
		"--node", node.CometRPCURL(),
		"--broadcast-mode", "sync",
		"--gas", "200000",
		"--fees", fees,
		"--yes",
		"--output", "json",
		"--log_no_color",
	)
}

// mustBroadcastAuthzGenericGrant runs `tx authz grant ... generic` with explicit
// fees and returns parsed CLI JSON response.
func mustBroadcastAuthzGenericGrant(
	t *testing.T,
	node *evmtest.Node,
	from, grantee, msgType, fees string,
) map[string]any {
	t.Helper()

	return mustBroadcastTxCommand(
		t,
		node,
		"tx", "authz", "grant", grantee, "generic",
		"--msg-type", msgType,
		"--from", from,
		"--home", node.HomeDir(),
		"--keyring-backend", "test",
		"--chain-id", node.ChainID(),
		"--node", node.CometRPCURL(),
		"--broadcast-mode", "sync",
		"--gas", "250000",
		"--fees", fees,
		"--yes",
		"--output", "json",
		"--log_no_color",
	)
}

// mustBroadcastTxCommand is a fail-fast wrapper for tx CLI commands that are
// expected to produce valid JSON output.
func mustBroadcastTxCommand(t *testing.T, node *evmtest.Node, args ...string) map[string]any {
	t.Helper()

	resp, out, err := broadcastTxCommandResult(t, node, args...)
	if err != nil {
		t.Fatalf("tx command failed: %v\nargs=%v\n%s", err, args, out)
	}
	return resp
}

// runTxCommand executes a `lumerad` command with timeout and returns raw output.
func runTxCommand(t *testing.T, node *evmtest.Node, args ...string) (string, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	return evmtest.RunCommand(ctx, node.RepoRoot(), node.BinPath(), args...)
}

// broadcastTxCommandResult runs a tx command and attempts to decode JSON even
// if the process returned non-zero (common for rejected CheckTx results).
func broadcastTxCommandResult(t *testing.T, node *evmtest.Node, args ...string) (map[string]any, string, error) {
	t.Helper()

	out, runErr := runTxCommand(t, node, args...)
	resp, decodeErr := decodeCLIJSON(out)
	if decodeErr != nil {
		if runErr != nil {
			return nil, out, fmt.Errorf("command failed (%v) and output was not json: %w", runErr, decodeErr)
		}
		return nil, out, decodeErr
	}
	return resp, out, runErr
}

// decodeCLIJSON parses tx CLI output that may include non-JSON prelude lines.
func decodeCLIJSON(out string) (map[string]any, error) {
	var resp map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err == nil {
		return resp, nil
	}

	// Some CLI paths can print informational lines before JSON.
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty output")
	}
	last := strings.TrimSpace(lines[len(lines)-1])
	if err := json.Unmarshal([]byte(last), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse output as JSON")
	}
	return resp, nil
}

// txResponseCode extracts the top-level `code` field from CLI tx response JSON.
// A missing code is treated as 0 (success-like shape).
func txResponseCode(resp map[string]any) uint64 {
	if resp == nil {
		return 0
	}
	rawCode, ok := resp["code"]
	if !ok {
		return 0
	}

	switch v := rawCode.(type) {
	case float64:
		if v < 0 {
			return 0
		}
		return uint64(v)
	case int:
		if v < 0 {
			return 0
		}
		return uint64(v)
	case int64:
		if v < 0 {
			return 0
		}
		return uint64(v)
	case uint64:
		return v
	case string:
		n, err := strconv.ParseUint(strings.TrimSpace(v), 10, 64)
		if err != nil {
			return 0
		}
		return n
	default:
		return 0
	}
}

// txResponseRawLog extracts `raw_log` from either top-level or nested
// `tx_response` output variants.
func txResponseRawLog(resp map[string]any) string {
	if resp == nil {
		return ""
	}
	if log, ok := resp["raw_log"].(string); ok {
		return strings.TrimSpace(log)
	}
	if txResp, ok := resp["tx_response"].(map[string]any); ok {
		if log, ok := txResp["raw_log"].(string); ok {
			return strings.TrimSpace(log)
		}
	}
	return ""
}

// mustTxHash returns tx hash from either top-level or nested `tx_response`.
func mustTxHash(t *testing.T, resp map[string]any) string {
	t.Helper()

	if resp == nil {
		t.Fatal("nil tx response")
	}
	if txHash, ok := resp["txhash"].(string); ok && strings.TrimSpace(txHash) != "" {
		return strings.TrimSpace(txHash)
	}
	if txResp, ok := resp["tx_response"].(map[string]any); ok {
		if txHash, ok := txResp["txhash"].(string); ok && strings.TrimSpace(txHash) != "" {
			return strings.TrimSpace(txHash)
		}
	}
	t.Fatalf("missing txhash in response: %#v", resp)
	return ""
}
