//go:build integration
// +build integration

package vm_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	testtext "github.com/LumeraProtocol/lumera/pkg/text"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// TestVMAddressConversionRoundTrip verifies `query evm bech32-to-0x` and
// `query evm 0x-to-bech32` conversion parity for the validator key.
func testVMAddressConversionRoundTrip(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	bech32Addr := node.KeyInfo().Address
	hexAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo()).Hex()

	outHex := mustRunNodeCommand(t, node,
		"query", "evm", "bech32-to-0x", bech32Addr,
		"--node", node.CometRPCURL(),
		"--home", node.HomeDir(),
		"--log_no_color",
	)
	gotHex := testtext.LastNonEmptyLine(outHex)
	if !strings.EqualFold(gotHex, hexAddr) {
		t.Fatalf("bech32->hex mismatch: got=%q want=%q", gotHex, hexAddr)
	}

	outBech32 := mustRunNodeCommand(t, node,
		"query", "evm", "0x-to-bech32", hexAddr,
		"--node", node.CometRPCURL(),
		"--home", node.HomeDir(),
		"--log_no_color",
	)
	gotBech32 := testtext.LastNonEmptyLine(outBech32)
	if gotBech32 != bech32Addr {
		t.Fatalf("hex->bech32 mismatch: got=%q want=%q", gotBech32, bech32Addr)
	}
}

// TestVMQueryAccountMatchesEthRPC ensures `query evm account` is consistent
// with JSON-RPC nonce/balance after a state-changing tx.
func testVMQueryAccountMatchesEthRPC(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	txHash := node.SendOneLegacyTx(t)
	receipt := node.WaitForReceipt(t, txHash, 40*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)

	bech32Addr := node.KeyInfo().Address
	hexAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo()).Hex()

	out := mustRunNodeCommand(t, node,
		"query", "evm", "account", bech32Addr,
		"--node", node.CometRPCURL(),
		"--output", "json",
		"--home", node.HomeDir(),
		"--log_no_color",
	)

	var resp evmAccountQueryResponse
	if err := decodeCLIJSON(out, &resp); err != nil {
		t.Fatalf("decode query evm account response: %v\n%s", err, out)
	}

	rpcNonce := mustGetEthTxCount(t, node, hexAddr)
	queryNonce, err := strconv.ParseUint(strings.TrimSpace(resp.Nonce), 10, 64)
	if err != nil {
		t.Fatalf("parse query nonce %q: %v", resp.Nonce, err)
	}
	if queryNonce != rpcNonce {
		t.Fatalf("nonce mismatch: query=%d rpc=%d", queryNonce, rpcNonce)
	}

	rpcBalance := mustGetEthBalance(t, node, hexAddr)
	if strings.TrimSpace(resp.Balance) != rpcBalance.String() {
		t.Fatalf("balance mismatch: query=%s rpc=%s", resp.Balance, rpcBalance.String())
	}

	emptyCodeHash := common.BytesToHash(crypto.Keccak256(nil)).Hex()
	if !strings.EqualFold(resp.CodeHash, emptyCodeHash) {
		t.Fatalf("unexpected EOA code hash: got=%q want=%q", resp.CodeHash, emptyCodeHash)
	}
}

// TestVMQueryAccountRejectsInvalidAddress checks defensive input handling in
// the CLI query path.
func testVMQueryAccountRejectsInvalidAddress(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	out, err := runNodeCommand(t, node,
		"query", "evm", "account", "0x0000",
		"--node", node.CometRPCURL(),
		"--output", "json",
		"--home", node.HomeDir(),
		"--log_no_color",
	)
	if err == nil {
		t.Fatalf("expected invalid-address query to fail, got success output:\n%s", out)
	}

	if !testtext.ContainsAny(strings.ToLower(out), "invalid", "address", "hex") {
		t.Fatalf("unexpected invalid-address output:\n%s", out)
	}
}
