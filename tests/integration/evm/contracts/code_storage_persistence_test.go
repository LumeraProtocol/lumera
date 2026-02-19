//go:build integration
// +build integration

package contracts_test

import (
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/core/vm"
	evmprogram "github.com/ethereum/go-ethereum/core/vm/program"
)

// TestContractCodePersistsAcrossRestart verifies deployed runtime bytecode is
// queryable via eth_getCode before and after process restart.
func TestContractCodePersistsAcrossRestart(t *testing.T) {
	t.Helper()

	node := evmtest.NewEVMNode(t, "lumera-contract-code-persistence", 280)
	node.StartAndWaitRPC()
	defer node.Stop()

	testContractCodePersistsAcrossRestart(t, node)
}

func testContractCodePersistsAcrossRestart(t *testing.T, node *evmtest.Node) {
	t.Helper()

	deployTxHash := sendLoggingConstantContractCreationTx(
		t,
		node.RPCURL(),
		node.KeyInfo(),
		"0x"+strings.Repeat("33", 32),
	)
	deployReceipt := evmtest.WaitForReceipt(t, node.RPCURL(), deployTxHash, node.WaitCh(), node.OutputBuffer(), 45*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, deployReceipt, deployTxHash)
	assertReceiptBasics(t, deployReceipt)

	contractAddress := evmtest.MustStringField(t, deployReceipt, "contractAddress")
	if strings.EqualFold(contractAddress, "0x0000000000000000000000000000000000000000") {
		t.Fatalf("unexpected zero contractAddress in deployment receipt: %#v", deployReceipt)
	}

	var codeBefore string
	evmtest.MustJSONRPC(t, node.RPCURL(), "eth_getCode", []any{contractAddress, "latest"}, &codeBefore)
	if strings.EqualFold(strings.TrimSpace(codeBefore), "0x") {
		t.Fatalf("expected non-empty runtime code, got %q", codeBefore)
	}

	var codeAtDeployBlock string
	evmtest.MustJSONRPC(
		t,
		node.RPCURL(),
		"eth_getCode",
		[]any{contractAddress, evmtest.MustStringField(t, deployReceipt, "blockNumber")},
		&codeAtDeployBlock,
	)
	if !strings.EqualFold(codeBefore, codeAtDeployBlock) {
		t.Fatalf("eth_getCode mismatch latest vs deploy block: latest=%s deploy=%s", codeBefore, codeAtDeployBlock)
	}

	node.RestartAndWaitRPC()

	var codeAfter string
	evmtest.MustJSONRPC(t, node.RPCURL(), "eth_getCode", []any{contractAddress, "latest"}, &codeAfter)
	if !strings.EqualFold(codeBefore, codeAfter) {
		t.Fatalf("contract bytecode changed across restart: before=%s after=%s", codeBefore, codeAfter)
	}
}

// TestContractStoragePersistsAcrossRestart verifies writes performed by a
// state-changing tx are visible via eth_getStorageAt before and after restart.
func TestContractStoragePersistsAcrossRestart(t *testing.T) {
	t.Helper()

	node := evmtest.NewEVMNode(t, "lumera-contract-storage-persistence", 300)
	node.StartAndWaitRPC()
	defer node.Stop()

	testContractStoragePersistsAcrossRestart(t, node)
}

func testContractStoragePersistsAcrossRestart(t *testing.T, node *evmtest.Node) {
	t.Helper()

	deployTxHash := sendContractCreationTx(t, node.RPCURL(), node.KeyInfo(), storageSetterContractCreationCode())
	deployReceipt := evmtest.WaitForReceipt(t, node.RPCURL(), deployTxHash, node.WaitCh(), node.OutputBuffer(), 45*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, deployReceipt, deployTxHash)
	assertReceiptBasics(t, deployReceipt)

	contractAddress := evmtest.MustStringField(t, deployReceipt, "contractAddress")
	if strings.EqualFold(contractAddress, "0x0000000000000000000000000000000000000000") {
		t.Fatalf("unexpected zero contractAddress in deployment receipt: %#v", deployReceipt)
	}

	callTxHash := sendContractMethodTx(t, node.RPCURL(), node.KeyInfo(), contractAddress, "0x")
	callReceipt := evmtest.WaitForReceipt(t, node.RPCURL(), callTxHash, node.WaitCh(), node.OutputBuffer(), 45*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, callReceipt, callTxHash)
	assertReceiptBasics(t, callReceipt)

	var slotBefore string
	evmtest.MustJSONRPC(t, node.RPCURL(), "eth_getStorageAt", []any{contractAddress, "0x0", "latest"}, &slotBefore)
	assertStorageSlot0Equals42(t, slotBefore)

	node.RestartAndWaitRPC()

	var slotAfter string
	evmtest.MustJSONRPC(t, node.RPCURL(), "eth_getStorageAt", []any{contractAddress, "0x0", "latest"}, &slotAfter)
	assertStorageSlot0Equals42(t, slotAfter)
	if !strings.EqualFold(slotBefore, slotAfter) {
		t.Fatalf("slot0 changed across restart: before=%s after=%s", slotBefore, slotAfter)
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

func assertStorageSlot0Equals42(t *testing.T, slotHex string) {
	t.Helper()

	normalized := strings.ToLower(strings.TrimSpace(slotHex))
	want := "0x" + strings.Repeat("0", 62) + "2a"
	if !strings.EqualFold(normalized, want) {
		t.Fatalf("unexpected slot0 value: got %s want %s", slotHex, want)
	}
}
