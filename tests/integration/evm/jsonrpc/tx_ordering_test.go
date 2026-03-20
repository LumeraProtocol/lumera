//go:build integration
// +build integration

package jsonrpc_test

import (
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	"math/big"
	"strings"
	"testing"
	"time"

	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
)

// TestMultiTxOrderingSameBlock verifies deterministic transactionIndex ordering
// for a same-sender, nonce-sequential tx burst included in one block.
func testMultiTxOrderingSameBlock(t *testing.T, node *evmtest.Node) {
	t.Helper()

	fromAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	privateKey := evmtest.MustDerivePrivateKey(t, node.KeyInfo().Mnemonic)
	baseNonce := node.MustGetPendingNonceWithRetry(t, fromAddr.Hex(), 20*time.Second)
	gasPrice := node.MustGetGasPriceWithRetry(t, 20*time.Second)

	toAddr := fromAddr
	txHashes := make([]string, 0, 3)
	// Send quickly with explicit nonces to bias inclusion in one block.
	for i := 0; i < 3; i++ {
		txHash := node.SendLegacyTxWithParams(t, evmtest.LegacyTxParams{
			PrivateKey: privateKey,
			Nonce:      baseNonce + uint64(i),
			To:         &toAddr,
			Value:      big.NewInt(int64(i + 1)),
			Gas:        21_000,
			GasPrice:   gasPrice,
			Data:       nil,
		})
		txHashes = append(txHashes, txHash)
	}

	receipts := make([]map[string]any, len(txHashes))
	for i, txHash := range txHashes {
		receipts[i] = node.WaitForReceipt(t, txHash, 40*time.Second)
		evmtest.AssertReceiptMatchesTxHash(t, receipts[i], txHash)
	}

	expectedBlock := evmtest.MustStringField(t, receipts[0], "blockNumber")
	for i := 1; i < len(receipts); i++ {
		got := evmtest.MustStringField(t, receipts[i], "blockNumber")
		if !strings.EqualFold(got, expectedBlock) {
			t.Fatalf("transactions were not in the same block: expected %s got %s (tx %d)", expectedBlock, got, i)
		}
	}

	indices := make([]uint64, len(receipts))
	for i, receipt := range receipts {
		indices[i] = evmtest.MustUint64HexField(t, receipt, "transactionIndex")
	}

	if !(indices[0] < indices[1] && indices[1] < indices[2]) {
		t.Fatalf("unexpected transactionIndex ordering: %v", indices)
	}
}
