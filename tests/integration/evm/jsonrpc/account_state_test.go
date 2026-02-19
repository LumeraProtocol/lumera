//go:build integration
// +build integration

package jsonrpc_test

import (
	"math/big"
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// testEOANonceByBlockTagAndRestart verifies eth_getTransactionCount semantics
// for latest and explicit block-tag queries, and ensures the result persists
// across restart.
func TestEOANonceByBlockTagAndRestart(t *testing.T) {
	t.Helper()

	node := evmtest.NewEVMNode(t, "lumera-account-nonce-state", 260)
	node.StartAndWaitRPC()
	defer node.Stop()

	testEOANonceByBlockTagAndRestart(t, node)
}

func testEOANonceByBlockTagAndRestart(t *testing.T, node *evmtest.Node) {
	t.Helper()
	evmtest.WaitForBlockNumberAtLeast(t, node.RPCURL(), 1, 20*time.Second)

	sender := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())

	initialLatest := mustGetTxCount(t, node.RPCURL(), sender.Hex(), "latest")
	initialPending := mustGetTxCount(t, node.RPCURL(), sender.Hex(), "pending")
	if initialLatest != initialPending {
		t.Fatalf("unexpected initial nonce mismatch latest=%d pending=%d", initialLatest, initialPending)
	}

	txHash := evmtest.SendOneLegacyTx(t, node.RPCURL(), node.KeyInfo())
	receipt := evmtest.WaitForReceipt(t, node.RPCURL(), txHash, node.WaitCh(), node.OutputBuffer(), 40*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)

	blockHex := evmtest.MustStringField(t, receipt, "blockNumber")
	blockNumber, err := hexutil.DecodeUint64(blockHex)
	if err != nil {
		t.Fatalf("decode receipt blockNumber %q: %v", blockHex, err)
	}

	if blockNumber > 0 {
		prevCount := mustGetTxCount(t, node.RPCURL(), sender.Hex(), hexutil.EncodeUint64(blockNumber-1))
		if prevCount != initialLatest {
			t.Fatalf("unexpected tx count at block %d: got %d want %d", blockNumber-1, prevCount, initialLatest)
		}
	}

	countAtBlock := mustGetTxCount(t, node.RPCURL(), sender.Hex(), blockHex)
	if countAtBlock != initialLatest+1 {
		t.Fatalf("unexpected tx count at inclusion block %s: got %d want %d", blockHex, countAtBlock, initialLatest+1)
	}

	latestAfter := mustGetTxCount(t, node.RPCURL(), sender.Hex(), "latest")
	if latestAfter != initialLatest+1 {
		t.Fatalf("unexpected latest tx count after one tx: got %d want %d", latestAfter, initialLatest+1)
	}

	node.RestartAndWaitRPC()

	latestAfterRestart := mustGetTxCount(t, node.RPCURL(), sender.Hex(), "latest")
	if latestAfterRestart != initialLatest+1 {
		t.Fatalf("unexpected latest tx count after restart: got %d want %d", latestAfterRestart, initialLatest+1)
	}
}

// testSelfTransferFeeAccounting verifies account-balance deduction equals
// gasUsed * effectiveGasPrice for a self-transfer tx and remains stable after
// restart.
func TestSelfTransferFeeAccounting(t *testing.T) {
	t.Helper()

	node := evmtest.NewEVMNode(t, "lumera-account-fee-accounting", 260)
	node.StartAndWaitRPC()
	defer node.Stop()

	testSelfTransferFeeAccounting(t, node)
}

func testSelfTransferFeeAccounting(t *testing.T, node *evmtest.Node) {
	t.Helper()
	evmtest.WaitForBlockNumberAtLeast(t, node.RPCURL(), 1, 20*time.Second)

	sender := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())

	balanceBefore := mustGetBalance(t, node.RPCURL(), sender.Hex(), "latest")

	txHash := evmtest.SendOneLegacyTx(t, node.RPCURL(), node.KeyInfo())
	receipt := evmtest.WaitForReceipt(t, node.RPCURL(), txHash, node.WaitCh(), node.OutputBuffer(), 40*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)

	gasUsed := evmtest.MustUint64HexField(t, receipt, "gasUsed")
	effectiveGasPriceHex := evmtest.MustStringField(t, receipt, "effectiveGasPrice")
	effectiveGasPrice, err := hexutil.DecodeBig(effectiveGasPriceHex)
	if err != nil {
		t.Fatalf("decode effectiveGasPrice %q: %v", effectiveGasPriceHex, err)
	}

	expectedDelta := new(big.Int).Mul(new(big.Int).SetUint64(gasUsed), effectiveGasPrice)
	balanceAfter := mustGetBalance(t, node.RPCURL(), sender.Hex(), "latest")
	actualDelta := new(big.Int).Sub(balanceBefore, balanceAfter)
	if actualDelta.Cmp(expectedDelta) != 0 {
		t.Fatalf(
			"unexpected sender balance delta: got=%s want=%s (gasUsed=%d effectiveGasPrice=%s)",
			actualDelta.String(),
			expectedDelta.String(),
			gasUsed,
			effectiveGasPrice.String(),
		)
	}

	node.RestartAndWaitRPC()

	balanceAfterRestart := mustGetBalance(t, node.RPCURL(), sender.Hex(), "latest")
	if balanceAfterRestart.Cmp(balanceAfter) != 0 {
		t.Fatalf("sender balance changed across restart: before=%s after=%s", balanceAfter, balanceAfterRestart)
	}
}

func mustGetTxCount(t *testing.T, rpcURL, addressHex, blockTag string) uint64 {
	t.Helper()

	var nonceHex string
	evmtest.MustJSONRPC(t, rpcURL, "eth_getTransactionCount", []any{addressHex, blockTag}, &nonceHex)

	nonce, err := hexutil.DecodeUint64(nonceHex)
	if err != nil {
		t.Fatalf("decode eth_getTransactionCount %q: %v", nonceHex, err)
	}
	return nonce
}

func mustGetBalance(t *testing.T, rpcURL, addressHex, blockTag string) *big.Int {
	t.Helper()

	var balanceHex string
	evmtest.MustJSONRPC(t, rpcURL, "eth_getBalance", []any{addressHex, blockTag}, &balanceHex)

	balance, err := hexutil.DecodeBig(balanceHex)
	if err != nil {
		t.Fatalf("decode eth_getBalance %q: %v", balanceHex, err)
	}
	return balance
}
