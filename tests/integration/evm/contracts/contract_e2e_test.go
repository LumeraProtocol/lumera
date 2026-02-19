//go:build integration
// +build integration

package contracts_test

import (
	"encoding/hex"
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	"math/big"
	"strings"
	"testing"
	"time"

	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	evmprogram "github.com/ethereum/go-ethereum/core/vm/program"
	"github.com/ethereum/go-ethereum/crypto"
)

// TestContractDeployCallAndLogsE2E validates the happy-path EVM contract flow.
//
// Workflow:
// 1. Deploy contract via eth_sendRawTransaction.
// 2. Read state via eth_call.
// 3. Send state-changing tx and verify receipt + log emission.
func testContractDeployCallAndLogsE2E(t *testing.T, node *evmtest.Node) {
	t.Helper()

	logTopic := "0x" + strings.Repeat("22", 32)

	// 1) Deploy contract through eth_sendRawTransaction and verify deployment receipt.
	deployTxHash := sendLoggingConstantContractCreationTx(t, node.RPCURL(), node.KeyInfo(), logTopic)
	deployReceipt := evmtest.WaitForReceipt(t, node.RPCURL(), deployTxHash, node.WaitCh(), node.OutputBuffer(), 45*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, deployReceipt, deployTxHash)
	assertReceiptBasics(t, deployReceipt)

	contractAddress := evmtest.MustStringField(t, deployReceipt, "contractAddress")
	if strings.EqualFold(contractAddress, "0x0000000000000000000000000000000000000000") {
		t.Fatalf("unexpected zero contractAddress in deployment receipt: %#v", deployReceipt)
	}

	// 2) Read-only method call via eth_call should return uint256(42).
	var callResultHex string
	evmtest.MustJSONRPC(t, node.RPCURL(), "eth_call", []any{
		map[string]any{
			"to":   contractAddress,
			"data": methodSelectorHex("getValue()"),
		},
		"latest",
	}, &callResultHex)
	assertEthCallReturnsUint256(t, callResultHex, 42)

	// 3) Stateful method call via transaction should emit a log and produce receipt/gas fields.
	callTxHash := sendContractMethodTx(t, node.RPCURL(), node.KeyInfo(), contractAddress, methodSelectorHex("emitEvent()"))
	callReceipt := evmtest.WaitForReceipt(t, node.RPCURL(), callTxHash, node.WaitCh(), node.OutputBuffer(), 45*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, callReceipt, callTxHash)
	assertReceiptBasics(t, callReceipt)
	assertReceiptHasTopic(t, callReceipt, logTopic)
}

// TestContractRevertTxReceiptAndGasE2E validates failed-call behavior.
//
// Workflow:
// 1. Deploy a contract that always reverts at runtime.
// 2. Execute a call tx and assert failed receipt semantics and gas usage.
func testContractRevertTxReceiptAndGasE2E(t *testing.T, node *evmtest.Node) {
	t.Helper()

	// 1) Deploy a contract whose runtime always REVERTs.
	deployTxHash := sendContractCreationTx(t, node.RPCURL(), node.KeyInfo(), alwaysRevertContractCreationCode())
	deployReceipt := evmtest.WaitForReceipt(t, node.RPCURL(), deployTxHash, node.WaitCh(), node.OutputBuffer(), 45*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, deployReceipt, deployTxHash)
	assertReceiptBasics(t, deployReceipt)

	contractAddress := evmtest.MustStringField(t, deployReceipt, "contractAddress")
	if strings.EqualFold(contractAddress, "0x0000000000000000000000000000000000000000") {
		t.Fatalf("unexpected zero contractAddress in deployment receipt: %#v", deployReceipt)
	}

	// 2) Send a stateful call tx that should fail and verify failed receipt + gas accounting.
	callTxHash := sendContractMethodTx(t, node.RPCURL(), node.KeyInfo(), contractAddress, methodSelectorHex("revertNow()"))
	callReceipt := evmtest.WaitForReceipt(t, node.RPCURL(), callTxHash, node.WaitCh(), node.OutputBuffer(), 45*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, callReceipt, callTxHash)
	assertFailedReceiptBasics(t, callReceipt)
}

// sendLoggingConstantContractCreationTx deploys a contract that logs and returns 42.
func sendLoggingConstantContractCreationTx(t *testing.T, rpcURL string, keyInfo testaccounts.TestKeyInfo, topicHex string) string {
	t.Helper()
	return sendContractCreationTx(t, rpcURL, keyInfo, loggingConstantContractCreationCode(topicHex))
}

func sendContractCreationTx(t *testing.T, rpcURL string, keyInfo testaccounts.TestKeyInfo, creationCode []byte) string {
	t.Helper()

	fromAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, keyInfo)
	privateKey := evmtest.MustDerivePrivateKey(t, keyInfo.Mnemonic)
	nonce := evmtest.MustGetPendingNonceWithRetry(t, rpcURL, fromAddr.Hex(), 20*time.Second)
	gasPrice := evmtest.MustGetGasPriceWithRetry(t, rpcURL, 20*time.Second)

	return evmtest.SendLegacyTxWithParams(t, rpcURL, evmtest.LegacyTxParams{
		PrivateKey: privateKey,
		Nonce:      nonce,
		To:         nil,
		Value:      big.NewInt(0),
		Gas:        500_000,
		GasPrice:   gasPrice,
		Data:       creationCode,
	})
}

// sendContractMethodTx sends a transaction that calls contract bytecode with provided calldata.
func sendContractMethodTx(t *testing.T, rpcURL string, keyInfo testaccounts.TestKeyInfo, toAddressHex, calldataHex string) string {
	t.Helper()

	fromAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, keyInfo)
	privateKey := evmtest.MustDerivePrivateKey(t, keyInfo.Mnemonic)
	nonce := evmtest.MustGetPendingNonceWithRetry(t, rpcURL, fromAddr.Hex(), 20*time.Second)
	gasPrice := evmtest.MustGetGasPriceWithRetry(t, rpcURL, 20*time.Second)
	toAddr := mustHexAddress(t, toAddressHex)

	return evmtest.SendLegacyTxWithParams(t, rpcURL, evmtest.LegacyTxParams{
		PrivateKey: privateKey,
		Nonce:      nonce,
		To:         &toAddr,
		Value:      big.NewInt(0),
		Gas:        250_000,
		GasPrice:   gasPrice,
		Data:       mustHexData(t, calldataHex),
	})
}

func loggingConstantContractCreationCode(topicHex string) []byte {
	topic := evmtest.TopicWordBytes(topicHex)

	/*
		Runtime (deployed contract code):
		- PUSH32 <topic>, PUSH1 0, PUSH1 0, LOG1
		  Emits one event with zero-length data and a single indexed topic.
		- PUSH1 42, PUSH1 0, MSTORE
		  Writes uint256(42) into memory slot [0:32].
		- PUSH1 32, PUSH1 0, RETURN
		  Returns 32 bytes so eth_call(getValue()) resolves to 42.
	*/
	runtime := evmprogram.New().
		Push(topic).Push(0).Push(0).Op(vm.LOG1).
		Push(42).Push(0).Op(vm.MSTORE).
		Return(0, 32).
		Bytes()

	/*
		Init/constructor code (runs only at deployment):
		- PUSH32 <topic>, PUSH1 0, PUSH1 0, LOG1
		  Emits one deployment-time event so receipt/log checks can validate
		  deploy-path log emission.
		- ReturnViaCodeCopy(runtime)
		  Equivalent to CODECOPY + RETURN pattern:
		  copy runtime bytes into memory and return them as the contract code.
	*/
	return evmprogram.New().
		Push(topic).Push(0).Push(0).Op(vm.LOG1).
		ReturnViaCodeCopy(runtime).
		Bytes()
}

func alwaysRevertContractCreationCode() []byte {
	/*
		Runtime:
		- PUSH1 0, PUSH1 0, REVERT
		  Always reverts immediately with empty revert data.
	*/
	runtime := evmprogram.New().
		Push(0).Push(0).Op(vm.REVERT).
		Bytes()

	/*
		Init:
		- ReturnViaCodeCopy(runtime)
		  Standard constructor that deploys the runtime above unchanged.
	*/
	return evmprogram.New().
		ReturnViaCodeCopy(runtime).
		Bytes()
}

func methodSelectorHex(signature string) string {
	sum := crypto.Keccak256([]byte(signature))
	return "0x" + hex.EncodeToString(sum[:4])
}

func assertEthCallReturnsUint256(t *testing.T, hexValue string, want uint64) {
	t.Helper()

	got := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(hexValue)), "0x")
	if got == "" {
		t.Fatalf("eth_call returned empty result")
	}
	if len(got)%2 != 0 {
		got = "0" + got
	}

	// Compare only the low 8 bytes to keep assertion simple and deterministic.
	if len(got) < 16 {
		got = strings.Repeat("0", 16-len(got)) + got
	}
	low64 := got[len(got)-16:]
	wantLow64 := hex.EncodeToString([]byte{
		byte(want >> 56), byte(want >> 48), byte(want >> 40), byte(want >> 32),
		byte(want >> 24), byte(want >> 16), byte(want >> 8), byte(want),
	})

	if low64 != wantLow64 {
		t.Fatalf("unexpected eth_call return: got %s want ...%s (full=%s)", low64, wantLow64, hexValue)
	}
}

func assertReceiptBasics(t *testing.T, receipt map[string]any) {
	t.Helper()

	status := evmtest.MustStringField(t, receipt, "status")
	if !strings.EqualFold(status, "0x1") {
		t.Fatalf("expected successful receipt status=0x1, got %q (%#v)", status, receipt)
	}

	gasUsed := evmtest.MustUint64HexField(t, receipt, "gasUsed")
	if gasUsed == 0 {
		t.Fatalf("expected non-zero gasUsed: %#v", receipt)
	}

	if _, ok := receipt["blockHash"].(string); !ok {
		t.Fatalf("receipt missing blockHash: %#v", receipt)
	}
	if _, ok := receipt["blockNumber"].(string); !ok {
		t.Fatalf("receipt missing blockNumber: %#v", receipt)
	}
}

func assertFailedReceiptBasics(t *testing.T, receipt map[string]any) {
	t.Helper()

	status := evmtest.MustStringField(t, receipt, "status")
	if !strings.EqualFold(status, "0x0") {
		t.Fatalf("expected failed receipt status=0x0, got %q (%#v)", status, receipt)
	}

	gasUsed := evmtest.MustUint64HexField(t, receipt, "gasUsed")
	if gasUsed == 0 {
		t.Fatalf("expected non-zero gasUsed for failed tx: %#v", receipt)
	}

	logs, ok := receipt["logs"].([]any)
	if !ok {
		t.Fatalf("failed receipt has unexpected logs field type: %#v", receipt["logs"])
	}
	if len(logs) != 0 {
		t.Fatalf("expected no logs for revert tx, got %#v", logs)
	}
}

func assertReceiptHasTopic(t *testing.T, receipt map[string]any, topicHex string) {
	t.Helper()

	wantTopic := "0x" + hex.EncodeToString(evmtest.TopicWordBytes(topicHex))
	rawLogs, ok := receipt["logs"].([]any)
	if !ok || len(rawLogs) == 0 {
		t.Fatalf("expected logs in receipt, got %#v", receipt["logs"])
	}

	for _, item := range rawLogs {
		logObj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		topics, ok := logObj["topics"].([]any)
		if !ok || len(topics) == 0 {
			continue
		}
		firstTopic, ok := topics[0].(string)
		if ok && strings.EqualFold(firstTopic, wantTopic) {
			return
		}
	}

	t.Fatalf("no log topic %s in receipt logs: %#v", wantTopic, receipt["logs"])
}

func mustHexAddress(t *testing.T, addrHex string) common.Address {
	t.Helper()

	trimmed := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(addrHex)), "0x")
	if len(trimmed) != 40 {
		t.Fatalf("invalid address hex %q", addrHex)
	}
	bz, err := hex.DecodeString(trimmed)
	if err != nil {
		t.Fatalf("invalid address hex %q: %v", addrHex, err)
	}
	return common.BytesToAddress(bz)
}

func mustHexData(t *testing.T, hexData string) []byte {
	t.Helper()

	trimmed := strings.TrimPrefix(strings.TrimSpace(hexData), "0x")
	bz, err := hex.DecodeString(trimmed)
	if err != nil {
		t.Fatalf("invalid hex data %q: %v", hexData, err)
	}
	return bz
}
