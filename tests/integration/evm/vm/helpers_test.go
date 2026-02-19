//go:build integration
// +build integration

package vm_test

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// evmAccountQueryResponse mirrors `query evm account --output json` fields.
type evmAccountQueryResponse struct {
	Balance  string `json:"balance"`
	CodeHash string `json:"code_hash"`
	Nonce    string `json:"nonce"`
}

// evmCodeQueryResponse mirrors `query evm code --output json` fields.
type evmCodeQueryResponse struct {
	Code string `json:"code"`
}

// evmStorageQueryResponse mirrors `query evm storage --output json` fields.
type evmStorageQueryResponse struct {
	Value string `json:"value"`
}

func mustQueryEVMAccount(t *testing.T, node *evmtest.Node, address string, height int64) evmAccountQueryResponse {
	t.Helper()

	args := []string{
		"query", "evm", "account", address,
		"--node", node.CometRPCURL(),
		"--output", "json",
		"--home", node.HomeDir(),
		"--log_no_color",
	}
	if height > 0 {
		args = append(args, "--height", strconv.FormatInt(height, 10))
	}

	out := mustRunNodeCommand(t, node, args...)
	var resp evmAccountQueryResponse
	if err := decodeCLIJSON(out, &resp); err != nil {
		t.Fatalf("decode query evm account response: %v\n%s", err, out)
	}
	return resp
}

func mustQueryEVMCode(t *testing.T, node *evmtest.Node, address string, height int64) []byte {
	t.Helper()

	args := []string{
		"query", "evm", "code", address,
		"--node", node.CometRPCURL(),
		"--output", "json",
		"--home", node.HomeDir(),
		"--log_no_color",
	}
	if height > 0 {
		args = append(args, "--height", strconv.FormatInt(height, 10))
	}

	out := mustRunNodeCommand(t, node, args...)
	var resp evmCodeQueryResponse
	if err := decodeCLIJSON(out, &resp); err != nil {
		t.Fatalf("decode query evm code response: %v\n%s", err, out)
	}
	return mustDecodeCodeBytes(t, resp.Code)
}

func mustQueryEVMStorage(t *testing.T, node *evmtest.Node, address, key string, height int64) string {
	t.Helper()

	args := []string{
		"query", "evm", "storage", address, key,
		"--node", node.CometRPCURL(),
		"--output", "json",
		"--home", node.HomeDir(),
		"--log_no_color",
	}
	if height > 0 {
		args = append(args, "--height", strconv.FormatInt(height, 10))
	}

	out := mustRunNodeCommand(t, node, args...)
	var resp evmStorageQueryResponse
	if err := decodeCLIJSON(out, &resp); err != nil {
		t.Fatalf("decode query evm storage response: %v\n%s", err, out)
	}
	return strings.TrimSpace(resp.Value)
}

func mustQueryEVMParams(t *testing.T, node *evmtest.Node) map[string]any {
	t.Helper()

	out := mustRunNodeCommand(t, node,
		"query", "evm", "params",
		"--node", node.CometRPCURL(),
		"--output", "json",
		"--home", node.HomeDir(),
		"--log_no_color",
	)
	var resp map[string]any
	if err := decodeCLIJSON(out, &resp); err != nil {
		t.Fatalf("decode query evm params response: %v\n%s", err, out)
	}
	return resp
}

func mustQueryEVMConfig(t *testing.T, node *evmtest.Node) map[string]any {
	t.Helper()

	out := mustRunNodeCommand(t, node,
		"query", "evm", "config",
		"--node", node.CometRPCURL(),
		"--output", "json",
		"--home", node.HomeDir(),
		"--log_no_color",
	)
	var resp map[string]any
	if err := decodeCLIJSON(out, &resp); err != nil {
		t.Fatalf("decode query evm config response: %v\n%s", err, out)
	}
	return resp
}

func mustParseUint64Dec(t *testing.T, s string, field string) uint64 {
	t.Helper()

	n, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	if err != nil {
		t.Fatalf("parse %s %q: %v", field, s, err)
	}
	return n
}

// runNodeCommand executes a lumerad command against the test node.
func runNodeCommand(t *testing.T, node *evmtest.Node, args ...string) (string, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	return evmtest.RunCommand(ctx, node.RepoRoot(), node.BinPath(), args...)
}

// mustRunNodeCommand is a fail-fast wrapper around runNodeCommand.
func mustRunNodeCommand(t *testing.T, node *evmtest.Node, args ...string) string {
	t.Helper()

	out, err := runNodeCommand(t, node, args...)
	if err != nil {
		t.Fatalf("command failed: %v\nargs=%v\n%s", err, args, out)
	}
	return out
}

// decodeCLIJSON unmarshals query output and supports trailing non-JSON lines.
func decodeCLIJSON(out string, v any) error {
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return fmt.Errorf("empty output")
	}

	if err := json.Unmarshal([]byte(trimmed), v); err == nil {
		return nil
	}

	lines := strings.Split(trimmed, "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	if last == "" {
		return fmt.Errorf("empty last output line")
	}
	if err := json.Unmarshal([]byte(last), v); err != nil {
		return fmt.Errorf("failed to parse JSON output")
	}
	return nil
}

// lastNonEmptyLine returns the last non-empty trimmed output line.
func lastNonEmptyLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line
		}
	}
	return ""
}

// mustDecodeCodeBytes parses code payload from query output.
//
// Depending on output codec, bytes can be rendered as base64 or 0x-hex.
func mustDecodeCodeBytes(t *testing.T, value string) []byte {
	t.Helper()

	v := strings.TrimSpace(value)
	if v == "" {
		return nil
	}

	if strings.HasPrefix(strings.ToLower(v), "0x") {
		bz, err := hexutil.Decode(v)
		if err == nil {
			return bz
		}
	}

	if bz, err := base64.StdEncoding.DecodeString(v); err == nil {
		return bz
	}
	if bz, err := base64.RawStdEncoding.DecodeString(v); err == nil {
		return bz
	}

	if bz, err := hex.DecodeString(v); err == nil {
		return bz
	}

	t.Fatalf("unable to decode code bytes from %q", value)
	return nil
}

func mustGetEthBalance(t *testing.T, rpcURL, addressHex string) *big.Int {
	t.Helper()

	var balanceHex string
	evmtest.MustJSONRPC(t, rpcURL, "eth_getBalance", []any{addressHex, "latest"}, &balanceHex)

	bal, err := hexutil.DecodeBig(balanceHex)
	if err != nil {
		t.Fatalf("decode eth_getBalance %q: %v", balanceHex, err)
	}
	return bal
}

func mustGetEthTxCount(t *testing.T, rpcURL, addressHex string) uint64 {
	t.Helper()

	var nonceHex string
	evmtest.MustJSONRPC(t, rpcURL, "eth_getTransactionCount", []any{addressHex, "latest"}, &nonceHex)

	nonce, err := hexutil.DecodeUint64(nonceHex)
	if err != nil {
		t.Fatalf("decode eth_getTransactionCount %q: %v", nonceHex, err)
	}
	return nonce
}

func sendContractCreationTx(t *testing.T, node *evmtest.Node, creationCode []byte) string {
	t.Helper()

	fromAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	privateKey := evmtest.MustDerivePrivateKey(t, node.KeyInfo().Mnemonic)
	nonce := evmtest.MustGetPendingNonceWithRetry(t, node.RPCURL(), fromAddr.Hex(), 20*time.Second)
	gasPrice := evmtest.MustGetGasPriceWithRetry(t, node.RPCURL(), 20*time.Second)

	return evmtest.SendLegacyTxWithParams(t, node.RPCURL(), evmtest.LegacyTxParams{
		PrivateKey: privateKey,
		Nonce:      nonce,
		To:         nil,
		Value:      big.NewInt(0),
		Gas:        500_000,
		GasPrice:   gasPrice,
		Data:       creationCode,
	})
}

func sendContractMethodTx(t *testing.T, node *evmtest.Node, contractHex string, inputHex string) string {
	t.Helper()

	fromAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	privateKey := evmtest.MustDerivePrivateKey(t, node.KeyInfo().Mnemonic)
	nonce := evmtest.MustGetPendingNonceWithRetry(t, node.RPCURL(), fromAddr.Hex(), 20*time.Second)
	gasPrice := evmtest.MustGetGasPriceWithRetry(t, node.RPCURL(), 20*time.Second)

	inputBz, err := hexutil.Decode(inputHex)
	if err != nil {
		t.Fatalf("decode input hex %q: %v", inputHex, err)
	}

	to := common.HexToAddress(contractHex)
	return evmtest.SendLegacyTxWithParams(t, node.RPCURL(), evmtest.LegacyTxParams{
		PrivateKey: privateKey,
		Nonce:      nonce,
		To:         &to,
		Value:      big.NewInt(0),
		Gas:        200_000,
		GasPrice:   gasPrice,
		Data:       inputBz,
	})
}
