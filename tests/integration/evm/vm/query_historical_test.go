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
)

// TestVMQueryAccountHistoricalHeightNonceProgression verifies that
// `query evm account --height` returns the nonce snapshot at the requested
// height before and after a successful transaction.
func testVMQueryAccountHistoricalHeightNonceProgression(t *testing.T, node *evmtest.Node) {
	t.Helper()

	// Ensure height-1 and height-2 queries are always valid snapshot targets.
	node.WaitForBlockNumberAtLeast(t, 3, 20*time.Second)

	bech32Addr := node.KeyInfo().Address
	hexAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo()).Hex()

	before := mustQueryEVMAccount(t, node, bech32Addr, 0)
	beforeNonce := mustParseUint64Dec(t, before.Nonce, "nonce")

	txHash := node.SendOneLegacyTx(t)
	receipt := node.WaitForReceipt(t, txHash, 45*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)
	txHeight := evmtest.MustUint64HexField(t, receipt, "blockNumber")
	if txHeight < 2 {
		t.Fatalf("unexpected tx height %d; need at least one prior height", txHeight)
	}

	atHeight := mustQueryEVMAccount(t, node, bech32Addr, int64(txHeight))
	atHeightNonce := mustParseUint64Dec(t, atHeight.Nonce, "nonce")
	if atHeightNonce != beforeNonce+1 {
		t.Fatalf("nonce at tx height mismatch: got=%d want=%d", atHeightNonce, beforeNonce+1)
	}

	beforeHeight := mustQueryEVMAccount(t, node, bech32Addr, int64(txHeight-1))
	beforeHeightNonce := mustParseUint64Dec(t, beforeHeight.Nonce, "nonce")
	if beforeHeightNonce != beforeNonce {
		t.Fatalf("nonce at height before tx mismatch: got=%d want=%d", beforeHeightNonce, beforeNonce)
	}

	// Cross-check latest nonce against JSON-RPC to ensure query/RPC parity.
	rpcNonce := mustGetEthTxCount(t, node, hexAddr)
	latest := mustQueryEVMAccount(t, node, bech32Addr, 0)
	latestNonce := mustParseUint64Dec(t, latest.Nonce, "nonce")
	if latestNonce != rpcNonce {
		t.Fatalf("latest nonce mismatch: query=%d rpc=%d", latestNonce, rpcNonce)
	}
}

// TestVMQueryHistoricalCodeAndStorageSnapshots verifies `query evm code` and
// `query evm storage --height` snapshots across contract deployment and a later
// storage write in a separate block.
func testVMQueryHistoricalCodeAndStorageSnapshots(t *testing.T, node *evmtest.Node) {
	t.Helper()

	node.WaitForBlockNumberAtLeast(t, 3, 20*time.Second)

	deployTxHash := sendContractCreationTx(t, node, storageSetterContractCreationCode())
	deployReceipt := node.WaitForReceipt(t, deployTxHash, 45*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, deployReceipt, deployTxHash)

	contractAddress := evmtest.MustStringField(t, deployReceipt, "contractAddress")
	if strings.EqualFold(contractAddress, "0x0000000000000000000000000000000000000000") {
		t.Fatalf("unexpected zero contractAddress in deployment receipt: %#v", deployReceipt)
	}

	deployHeight := evmtest.MustUint64HexField(t, deployReceipt, "blockNumber")
	if deployHeight < 2 {
		t.Fatalf("unexpected deploy height %d; need prior snapshot", deployHeight)
	}

	codeBeforeDeploy := mustQueryEVMCode(t, node, contractAddress, int64(deployHeight-1))
	if len(codeBeforeDeploy) != 0 {
		t.Fatalf("expected empty code before deployment, got %x", codeBeforeDeploy)
	}

	codeAtDeploy := mustQueryEVMCode(t, node, contractAddress, int64(deployHeight))
	if len(codeAtDeploy) == 0 {
		t.Fatalf("expected runtime code at deployment height")
	}

	// Wait for the next block so storage write lands strictly after deploy height.
	node.WaitForBlockNumberAtLeast(t, deployHeight+1, 20*time.Second)

	callTxHash := sendContractMethodTx(t, node, contractAddress, "0x")
	callReceipt := node.WaitForReceipt(t, callTxHash, 45*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, callReceipt, callTxHash)

	callHeight := evmtest.MustUint64HexField(t, callReceipt, "blockNumber")
	if callHeight <= deployHeight {
		t.Fatalf("expected storage write in later block: deploy=%d call=%d", deployHeight, callHeight)
	}

	wantZero := "0x" + strings.Repeat("0", 64)
	storageAtDeploy := mustQueryEVMStorage(t, node, contractAddress, "0x0", int64(deployHeight))
	if !strings.EqualFold(storageAtDeploy, wantZero) {
		t.Fatalf("unexpected storage at deploy height: got=%s want=%s", storageAtDeploy, wantZero)
	}

	storageBeforeCall := mustQueryEVMStorage(t, node, contractAddress, "0x0", int64(callHeight-1))
	if !strings.EqualFold(storageBeforeCall, wantZero) {
		t.Fatalf("unexpected storage before write tx: got=%s want=%s", storageBeforeCall, wantZero)
	}

	wantWritten := "0x" + strings.Repeat("0", 62) + "2a"
	storageAtCall := mustQueryEVMStorage(t, node, contractAddress, "0x0", int64(callHeight))
	if !strings.EqualFold(storageAtCall, wantWritten) {
		t.Fatalf("unexpected storage at write height: got=%s want=%s", storageAtCall, wantWritten)
	}

	latest := mustQueryEVMStorage(t, node, contractAddress, "0x0", 0)
	if !strings.EqualFold(latest, wantWritten) {
		t.Fatalf("unexpected latest storage value: got=%s want=%s", latest, wantWritten)
	}

	balanceAtCall := mustQueryEVMAccount(t, node, contractAddress, int64(callHeight)).Balance
	if _, ok := new(big.Int).SetString(strings.TrimSpace(balanceAtCall), 10); !ok {
		t.Fatalf("contract balance is not decimal at call height: %q", balanceAtCall)
	}
}
