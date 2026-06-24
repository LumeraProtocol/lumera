//go:build integration
// +build integration

package vm_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/vm"
	evmprogram "github.com/ethereum/go-ethereum/core/vm/program"
)

// TestVMQueryCodeAndStorageMatchJSONRPC validates `query evm code` and
// `query evm storage` against JSON-RPC for a deployed contract with one
// deterministic storage write.
func testVMQueryCodeAndStorageMatchJSONRPC(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	deployTxHash := sendContractCreationTx(t, node, storageSetterContractCreationCode())
	deployReceipt := node.WaitForReceipt(t, deployTxHash, 45*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, deployReceipt, deployTxHash)

	contractAddress := evmtest.MustStringField(t, deployReceipt, "contractAddress")
	if strings.EqualFold(contractAddress, "0x0000000000000000000000000000000000000000") {
		t.Fatalf("unexpected zero contractAddress in deployment receipt: %#v", deployReceipt)
	}

	callTxHash := sendContractMethodTx(t, node, contractAddress, "0x")
	callReceipt := node.WaitForReceipt(t, callTxHash, 45*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, callReceipt, callTxHash)

	outCode := mustRunNodeCommand(t, node,
		"query", "evm", "code", contractAddress,
		"--node", node.CometRPCURL(),
		"--output", "json",
		"--home", node.HomeDir(),
		"--log_no_color",
	)

	var codeResp evmCodeQueryResponse
	if err := decodeCLIJSON(outCode, &codeResp); err != nil {
		t.Fatalf("decode query evm code response: %v\n%s", err, outCode)
	}
	codeFromQuery := mustDecodeCodeBytes(t, codeResp.Code)

	var codeFromRPC string
	node.MustJSONRPC(t, "eth_getCode", []any{contractAddress, "latest"}, &codeFromRPC)
	rpcCodeBytes, err := hexutil.Decode(codeFromRPC)
	if err != nil {
		t.Fatalf("decode eth_getCode %q: %v", codeFromRPC, err)
	}

	if !bytes.Equal(codeFromQuery, rpcCodeBytes) {
		t.Fatalf("query evm code mismatch vs eth_getCode: query=%x rpc=%x", codeFromQuery, rpcCodeBytes)
	}

	outStorage := mustRunNodeCommand(t, node,
		"query", "evm", "storage", contractAddress, "0x0",
		"--node", node.CometRPCURL(),
		"--output", "json",
		"--home", node.HomeDir(),
		"--log_no_color",
	)

	var storageResp evmStorageQueryResponse
	if err := decodeCLIJSON(outStorage, &storageResp); err != nil {
		t.Fatalf("decode query evm storage response: %v\n%s", err, outStorage)
	}

	var storageFromRPC string
	node.MustJSONRPC(t, "eth_getStorageAt", []any{contractAddress, "0x0", "latest"}, &storageFromRPC)

	if !strings.EqualFold(strings.TrimSpace(storageResp.Value), strings.TrimSpace(storageFromRPC)) {
		t.Fatalf("query evm storage mismatch vs eth_getStorageAt: query=%s rpc=%s", storageResp.Value, storageFromRPC)
	}

	wantSlot0 := "0x" + strings.Repeat("0", 62) + "2a"
	if !strings.EqualFold(storageFromRPC, wantSlot0) {
		t.Fatalf("unexpected slot0 value: got=%s want=%s", storageFromRPC, wantSlot0)
	}
}

func storageSetterContractCreationCode() []byte {
	/*
		Runtime:
		- PUSH1 0x2a, PUSH1 0x00, SSTORE, STOP
		  On any call, stores 42 in storage slot 0 and halts successfully.
	*/
	runtime := evmprogram.New().
		Push(42).Push(0).Op(vm.SSTORE).
		Op(vm.STOP).
		Bytes()

	/*
		Init:
		- ReturnViaCodeCopy(runtime)
		  Deploys the runtime above unchanged.
	*/
	return evmprogram.New().
		ReturnViaCodeCopy(runtime).
		Bytes()
}
