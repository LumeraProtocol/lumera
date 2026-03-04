//go:build integration
// +build integration

package vm_test

import (
	"math/big"
	"strings"
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	testtext "github.com/LumeraProtocol/lumera/pkg/text"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/vm"
	evmprogram "github.com/ethereum/go-ethereum/core/vm/program"
	"github.com/ethereum/go-ethereum/crypto"
)

// TestVMBalanceERC20MatchesEthCall verifies that `query evm balance-erc20`
// returns the same amount as direct `eth_call` for `balanceOf(address)`.
//
// Workflow:
// 1. Deploy a deterministic contract that always returns uint256(42).
// 2. Query balance via CLI `query evm balance-erc20`.
// 3. Query balance via JSON-RPC `eth_call` and compare amounts.
func testVMBalanceERC20MatchesEthCall(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	deployTxHash := sendContractCreationTx(t, node, erc20ConstantBalanceCreationCode())
	deployReceipt := node.WaitForReceipt(t, deployTxHash, 45*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, deployReceipt, deployTxHash)

	contractAddress := evmtest.MustStringField(t, deployReceipt, "contractAddress")
	if strings.EqualFold(contractAddress, "0x0000000000000000000000000000000000000000") {
		t.Fatalf("unexpected zero contractAddress in deployment receipt: %#v", deployReceipt)
	}

	holderHex := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo()).Hex()
	queryOut := mustRunNodeCommand(t, node,
		"query", "evm", "balance-erc20", holderHex, contractAddress,
		"--node", node.CometRPCURL(),
		"--home", node.HomeDir(),
		"--log_no_color",
	)
	cliAmount, cliERC20 := mustParseBalanceERC20Output(t, queryOut)
	if !strings.EqualFold(cliERC20, contractAddress) {
		t.Fatalf("cli erc20 address mismatch: got=%s want=%s", cliERC20, contractAddress)
	}

	callData := balanceOfCallData(holderHex)
	var rpcRet string
	node.MustJSONRPC(t, "eth_call", []any{
		map[string]any{
			"to":   contractAddress,
			"data": callData,
		},
		"latest",
	}, &rpcRet)

	rpcAmount := mustDecodeUint256Hex(t, rpcRet)
	if cliAmount.Cmp(rpcAmount) != 0 {
		t.Fatalf("balance mismatch cli vs eth_call: cli=%s rpc=%s", cliAmount.String(), rpcAmount.String())
	}
	if cliAmount.Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("unexpected deterministic balance: got=%s want=42", cliAmount.String())
	}
}

// TestVMBalanceERC20RejectsNonERC20Runtime verifies failure semantics when
// `balance-erc20` is called against runtime code that does not implement
// `balanceOf(address)` ABI return data.
func testVMBalanceERC20RejectsNonERC20Runtime(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	deployTxHash := sendContractCreationTx(t, node, storageSetterContractCreationCode())
	deployReceipt := node.WaitForReceipt(t, deployTxHash, 45*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, deployReceipt, deployTxHash)

	contractAddress := evmtest.MustStringField(t, deployReceipt, "contractAddress")
	holderHex := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo()).Hex()

	out, err := runNodeCommand(t, node,
		"query", "evm", "balance-erc20", holderHex, contractAddress,
		"--node", node.CometRPCURL(),
		"--home", node.HomeDir(),
		"--log_no_color",
	)
	if err == nil {
		t.Fatalf("expected balance-erc20 to fail for non-ERC20 runtime, got success output:\n%s", out)
	}

	lower := strings.ToLower(out)
	if !testtext.ContainsAny(lower, "unpack", "abi", "empty", "output", "marshal") {
		t.Fatalf("unexpected balance-erc20 error output:\n%s", out)
	}
}

func erc20ConstantBalanceCreationCode() []byte {
	/*
		Runtime:
		- Store uint256(42) at memory [0:32] and return it.
		- Ignores calldata, so any method selector (including balanceOf) returns 42.
	*/
	runtime := evmprogram.New().
		Push(42).Push(0).Op(vm.MSTORE).
		Return(0, 32).
		Bytes()

	/*
		Init:
		- Return deployed runtime unchanged.
	*/
	return evmprogram.New().
		ReturnViaCodeCopy(runtime).
		Bytes()
}

func balanceOfCallData(holderHex string) string {
	selector := crypto.Keccak256([]byte("balanceOf(address)"))[:4]
	addr := common.HexToAddress(holderHex)
	data := append([]byte{}, selector...)
	data = append(data, common.LeftPadBytes(addr.Bytes(), 32)...)
	return hexutil.Encode(data)
}

func mustDecodeUint256Hex(t *testing.T, value string) *big.Int {
	t.Helper()

	v := strings.TrimSpace(strings.ToLower(value))
	v = strings.TrimPrefix(v, "0x")
	if v == "" {
		t.Fatalf("decode uint256 hex %q: empty value", value)
	}

	n, ok := new(big.Int).SetString(v, 16)
	if !ok || n == nil {
		t.Fatalf("decode uint256 hex %q: invalid hex value", value)
	}
	return n
}

func mustParseBalanceERC20Output(t *testing.T, out string) (*big.Int, string) {
	t.Helper()

	var amountStr string
	var erc20Addr string

	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "amount:") {
			amountStr = strings.TrimSpace(strings.TrimPrefix(trimmed, "amount:"))
		}
		if strings.HasPrefix(trimmed, "erc20_address:") {
			erc20Addr = strings.TrimSpace(strings.TrimPrefix(trimmed, "erc20_address:"))
		}
	}

	if amountStr == "" {
		t.Fatalf("missing amount in balance-erc20 output:\n%s", out)
	}
	if erc20Addr == "" {
		t.Fatalf("missing erc20_address in balance-erc20 output:\n%s", out)
	}

	amount, ok := new(big.Int).SetString(amountStr, 10)
	if !ok {
		t.Fatalf("invalid decimal amount %q in output:\n%s", amountStr, out)
	}

	return amount, erc20Addr
}
