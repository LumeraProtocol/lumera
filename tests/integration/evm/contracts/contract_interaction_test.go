//go:build integration
// +build integration

package contracts_test

import (
	"encoding/hex"
	"math/big"
	"strings"
	"testing"
	"time"

	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	evmprogram "github.com/ethereum/go-ethereum/core/vm/program"
	"github.com/ethereum/go-ethereum/crypto"
)

// testCALLBetweenContracts validates the CALL opcode for cross-contract
// invocation. A caller contract reads the target address from calldata,
// CALLs into a callee that returns uint256(99), and forwards that result.
func testCALLBetweenContracts(t *testing.T, node *evmtest.Node) {
	t.Helper()

	// 1) Deploy callee: returns uint256(99) on any call.
	calleeDeploy := sendContractCreationTx(t, node, calleeReturns99CreationCode())
	calleeReceipt := node.WaitForReceipt(t, calleeDeploy, 45*time.Second)
	assertReceiptBasics(t, calleeReceipt)
	calleeAddr := evmtest.MustStringField(t, calleeReceipt, "contractAddress")

	// 2) Deploy caller: reads address from calldata[0:32], CALLs it, returns output.
	callerDeploy := sendContractCreationTx(t, node, callerViaCALLCreationCode())
	callerReceipt := node.WaitForReceipt(t, callerDeploy, 45*time.Second)
	assertReceiptBasics(t, callerReceipt)
	callerAddr := evmtest.MustStringField(t, callerReceipt, "contractAddress")

	// 3) eth_call the caller with the callee address as calldata.
	calldata := abiEncodeAddress(t, mustHexAddress(t, calleeAddr))
	var resultHex string
	node.MustJSONRPC(t, "eth_call", []any{
		map[string]any{
			"to":   callerAddr,
			"data": "0x" + hex.EncodeToString(calldata),
		},
		"latest",
	}, &resultHex)

	assertEthCallReturnsUint256(t, resultHex, 99)
}

// testDELEGATECALLPreservesContext validates that DELEGATECALL executes the
// target's code in the caller's storage context. A proxy DELEGATECALLs a
// storage-writer that stores CALLER into slot 0. The write must land in the
// proxy's storage (not the writer's).
func testDELEGATECALLPreservesContext(t *testing.T, node *evmtest.Node) {
	t.Helper()

	senderAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())

	// 1) Deploy writer: stores CALLER into slot 0 on any call.
	writerDeploy := sendContractCreationTx(t, node, callerStorageWriterCreationCode())
	writerReceipt := node.WaitForReceipt(t, writerDeploy, 45*time.Second)
	assertReceiptBasics(t, writerReceipt)
	writerAddr := evmtest.MustStringField(t, writerReceipt, "contractAddress")

	// 2) Deploy proxy: DELEGATECALLs target address from calldata[0:32].
	proxyDeploy := sendContractCreationTx(t, node, delegateCallProxyCreationCode())
	proxyReceipt := node.WaitForReceipt(t, proxyDeploy, 45*time.Second)
	assertReceiptBasics(t, proxyReceipt)
	proxyAddr := evmtest.MustStringField(t, proxyReceipt, "contractAddress")

	// 3) Send tx to proxy with writer address as calldata.
	calldata := "0x" + hex.EncodeToString(abiEncodeAddress(t, mustHexAddress(t, writerAddr)))
	callTxHash := sendContractMethodTx(t, node, proxyAddr, calldata)
	callReceipt := node.WaitForReceipt(t, callTxHash, 45*time.Second)
	assertReceiptBasics(t, callReceipt)

	// 4) Proxy's slot 0 should contain the EOA sender address.
	var proxySlot0 string
	node.MustJSONRPC(t, "eth_getStorageAt", []any{proxyAddr, "0x0", "latest"}, &proxySlot0)
	expectedSlot := "0x" + hex.EncodeToString(common.LeftPadBytes(senderAddr.Bytes(), 32))
	if !strings.EqualFold(strings.TrimSpace(proxySlot0), expectedSlot) {
		t.Fatalf("proxy slot0 mismatch: got %s want %s", proxySlot0, expectedSlot)
	}

	// 5) Writer's slot 0 should be zero — its storage was never touched.
	var writerSlot0 string
	node.MustJSONRPC(t, "eth_getStorageAt", []any{writerAddr, "0x0", "latest"}, &writerSlot0)
	assertStorageSlotIsZero(t, writerSlot0)
}

// testCREATE2DeterministicAddress validates the CREATE2 opcode via a factory
// contract. The factory deploys a child contract with a fixed salt. The test
// computes the expected address off-chain and verifies the child's code and
// return value match.
func testCREATE2DeterministicAddress(t *testing.T, node *evmtest.Node) {
	t.Helper()

	childInit := childReturns42CreationCode()

	// 1) Deploy factory: CREATE2-deploys child with salt=1, returns child address.
	factoryDeploy := sendContractCreationTx(t, node, create2FactoryCreationCode(childInit))
	factoryReceipt := node.WaitForReceipt(t, factoryDeploy, 45*time.Second)
	assertReceiptBasics(t, factoryReceipt)
	factoryAddr := mustHexAddress(t, evmtest.MustStringField(t, factoryReceipt, "contractAddress"))

	// 2) Call factory to deploy the child (needs real tx for state persistence).
	fromAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	privateKey := evmtest.MustDerivePrivateKey(t, node.KeyInfo().Mnemonic)
	nonce := node.MustGetPendingNonceWithRetry(t, fromAddr.Hex(), 20*time.Second)
	gasPrice := node.MustGetGasPriceWithRetry(t, 20*time.Second)
	factoryCallHash := node.SendLegacyTxWithParams(t, evmtest.LegacyTxParams{
		PrivateKey: privateKey,
		Nonce:      nonce,
		To:         &factoryAddr,
		Value:      big.NewInt(0),
		Gas:        500_000, // higher gas for CREATE2
		GasPrice:   gasPrice,
		Data:       nil,
	})
	factoryCallReceipt := node.WaitForReceipt(t, factoryCallHash, 45*time.Second)
	assertReceiptBasics(t, factoryCallReceipt)

	// 3) Compute expected CREATE2 address.
	var salt [32]byte
	salt[31] = 1
	expectedChildAddr := crypto.CreateAddress2(factoryAddr, salt, crypto.Keccak256(childInit))

	// 4) Verify child contract code exists at expected address.
	var code string
	node.MustJSONRPC(t, "eth_getCode", []any{expectedChildAddr.Hex(), "latest"}, &code)
	if strings.EqualFold(strings.TrimSpace(code), "0x") || strings.TrimSpace(code) == "" {
		t.Fatalf("no code at expected CREATE2 address %s", expectedChildAddr.Hex())
	}

	// 5) eth_call to child should return uint256(42).
	var childResult string
	node.MustJSONRPC(t, "eth_call", []any{
		map[string]any{
			"to":   expectedChildAddr.Hex(),
			"data": "0x",
		},
		"latest",
	}, &childResult)
	assertEthCallReturnsUint256(t, childResult, 42)
}

// testSTATICCALLCannotModifyState validates that STATICCALL enforces read-only
// semantics. A static-caller contract STATICCALLs a state-writer. Because the
// writer attempts SSTORE, the STATICCALL must fail (return 0).
func testSTATICCALLCannotModifyState(t *testing.T, node *evmtest.Node) {
	t.Helper()

	// 1) Deploy writer: stores value 1 into slot 0 on any call.
	writerDeploy := sendContractCreationTx(t, node, slotWriterCreationCode())
	writerReceipt := node.WaitForReceipt(t, writerDeploy, 45*time.Second)
	assertReceiptBasics(t, writerReceipt)
	writerAddr := evmtest.MustStringField(t, writerReceipt, "contractAddress")

	// 2) Deploy static caller: STATICCALLs target from calldata, returns success flag.
	staticDeploy := sendContractCreationTx(t, node, staticCallWrapperCreationCode())
	staticReceipt := node.WaitForReceipt(t, staticDeploy, 45*time.Second)
	assertReceiptBasics(t, staticReceipt)
	staticAddr := evmtest.MustStringField(t, staticReceipt, "contractAddress")

	// 3) eth_call the static caller with writer address → expect 0 (STATICCALL failed).
	calldata := abiEncodeAddress(t, mustHexAddress(t, writerAddr))
	var resultHex string
	node.MustJSONRPC(t, "eth_call", []any{
		map[string]any{
			"to":   staticAddr,
			"data": "0x" + hex.EncodeToString(calldata),
		},
		"latest",
	}, &resultHex)
	assertEthCallReturnsUint256(t, resultHex, 0)
}

// ---------------------------------------------------------------------------
// Bytecode generators
// ---------------------------------------------------------------------------

// calleeReturns99CreationCode returns init code for a contract whose runtime
// returns uint256(99) on any call.
func calleeReturns99CreationCode() []byte {
	runtime := evmprogram.New().
		Push(99).Push(0).Op(vm.MSTORE).
		Return(0, 32).
		Bytes()

	return evmprogram.New().
		ReturnViaCodeCopy(runtime).
		Bytes()
}

// callerViaCALLCreationCode returns init code for a contract that reads an
// address from calldata[0:32], CALLs it with no input, and returns the 32-byte
// output.
func callerViaCALLCreationCode() []byte {
	// Runtime: CALL(gas, addr_from_calldata, 0, 0, 0, 0, 32) then RETURN(0,32)
	//
	// Stack layout for CALL (consumed top-first):
	//   gas, addr, value, argsOffset, argsSize, retOffset, retSize
	// Push in reverse (bottom-of-stack first):
	runtime := evmprogram.New().
		Push(32).                    // retSize = 32
		Push(0).                     // retOffset = 0
		Push(0).                     // argsSize = 0
		Push(0).                     // argsOffset = 0
		Push(0).                     // value = 0
		Push(0).Op(vm.CALLDATALOAD). // addr from calldata[0]
		Op(vm.GAS).                  // gas = all remaining
		Op(vm.CALL).                 // CALL → success flag on stack
		Op(vm.POP).                  // discard success flag
		Return(0, 32).              // return memory[0:32] (callee's output)
		Bytes()

	return evmprogram.New().
		ReturnViaCodeCopy(runtime).
		Bytes()
}

// callerStorageWriterCreationCode returns init code for a contract that stores
// CALLER (msg.sender) into storage slot 0 on any call.
func callerStorageWriterCreationCode() []byte {
	runtime := evmprogram.New().
		Op(vm.CALLER).  // push msg.sender
		Push(0).        // slot 0
		Op(vm.SSTORE).  // SSTORE(0, caller)
		Op(vm.STOP).
		Bytes()

	return evmprogram.New().
		ReturnViaCodeCopy(runtime).
		Bytes()
}

// delegateCallProxyCreationCode returns init code for a proxy contract that
// DELEGATECALLs the target address read from calldata[0:32].
func delegateCallProxyCreationCode() []byte {
	// DELEGATECALL stack (consumed top-first):
	//   gas, addr, argsOffset, argsSize, retOffset, retSize
	runtime := evmprogram.New().
		Push(0).                     // retSize = 0
		Push(0).                     // retOffset = 0
		Push(0).                     // argsSize = 0
		Push(0).                     // argsOffset = 0
		Push(0).Op(vm.CALLDATALOAD). // addr from calldata[0]
		Op(vm.GAS).                  // gas = all remaining
		Op(vm.DELEGATECALL).         // DELEGATECALL
		Op(vm.POP).                  // discard success flag
		Op(vm.STOP).
		Bytes()

	return evmprogram.New().
		ReturnViaCodeCopy(runtime).
		Bytes()
}

// childReturns42CreationCode returns the init code for a child contract whose
// runtime returns uint256(42) on any call.
func childReturns42CreationCode() []byte {
	runtime := evmprogram.New().
		Push(42).Push(0).Op(vm.MSTORE).
		Return(0, 32).
		Bytes()

	return evmprogram.New().
		ReturnViaCodeCopy(runtime).
		Bytes()
}

// create2FactoryCreationCode returns init code for a factory contract that
// CREATE2-deploys the given childInit with salt=1, then returns the new
// contract address.
func create2FactoryCreationCode(childInit []byte) []byte {
	runtime := evmprogram.New().
		Create2(childInit, 1).  // CREATE2 → child address on stack
		Push(0).Op(vm.MSTORE). // store address at memory[0]
		Return(0, 32).         // return memory[0:32]
		Bytes()

	return evmprogram.New().
		ReturnViaCodeCopy(runtime).
		Bytes()
}

// slotWriterCreationCode returns init code for a contract that writes value 1
// to storage slot 0 on any call. Used to test STATICCALL enforcement.
func slotWriterCreationCode() []byte {
	runtime := evmprogram.New().
		Push(1).Push(0).Op(vm.SSTORE). // SSTORE(0, 1)
		Op(vm.STOP).
		Bytes()

	return evmprogram.New().
		ReturnViaCodeCopy(runtime).
		Bytes()
}

// staticCallWrapperCreationCode returns init code for a contract that
// STATICCALLs the target address read from calldata[0:32] and returns the
// success flag as uint256 (1=success, 0=failure).
func staticCallWrapperCreationCode() []byte {
	// STATICCALL stack (consumed top-first):
	//   gas, addr, argsOffset, argsSize, retOffset, retSize
	runtime := evmprogram.New().
		Push(0).                     // retSize = 0
		Push(0).                     // retOffset = 0
		Push(0).                     // argsSize = 0
		Push(0).                     // argsOffset = 0
		Push(0).Op(vm.CALLDATALOAD). // addr from calldata[0]
		Op(vm.GAS).                  // gas = all remaining
		Op(vm.STATICCALL).           // STATICCALL → 0 (fail) or 1 (success)
		Push(0).Op(vm.MSTORE).      // store result at memory[0]
		Return(0, 32).              // return memory[0:32]
		Bytes()

	return evmprogram.New().
		ReturnViaCodeCopy(runtime).
		Bytes()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// abiEncodeAddress returns a 32-byte left-padded representation of an address
// suitable for use as EVM calldata.
func abiEncodeAddress(t *testing.T, addr common.Address) []byte {
	t.Helper()
	return common.LeftPadBytes(addr.Bytes(), 32)
}

// assertStorageSlotIsZero verifies a storage slot value is the zero word.
func assertStorageSlotIsZero(t *testing.T, slotHex string) {
	t.Helper()

	normalized := strings.ToLower(strings.TrimSpace(slotHex))
	zero := "0x" + strings.Repeat("0", 64)
	if !strings.EqualFold(normalized, zero) {
		t.Fatalf("expected zero storage slot, got %s", slotHex)
	}
}
