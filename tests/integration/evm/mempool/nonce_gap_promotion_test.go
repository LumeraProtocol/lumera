//go:build integration
// +build integration

package mempool_test

import (
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	"math/big"
	"testing"
	"time"

	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
)

// TestNonceGapPromotionAfterGapFilled verifies queued nonce-gap tx promotion.
//
// Workflow:
// 1. Submit nonce N and nonce N+2.
// 2. Confirm N+2 is not mined while gap exists.
// 3. Submit nonce N+1 and verify eventual ordered inclusion of N+1 then N+2.
func testNonceGapPromotionAfterGapFilled(t *testing.T, node *evmtest.Node) {
	t.Helper()

	fromAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	privateKey := evmtest.MustDerivePrivateKey(t, node.KeyInfo().Mnemonic)
	baseNonce := node.MustGetPendingNonceWithRetry(t, fromAddr.Hex(), 20*time.Second)
	toAddr := fromAddr
	// Use node-reported gas price (already base-fee aware) with headroom.
	gasPrice := new(big.Int).Mul(node.MustGetGasPriceWithRetry(t, 20*time.Second), big.NewInt(2))

	tx0 := node.SendLegacyTxWithParams(t, evmtest.LegacyTxParams{
		PrivateKey: privateKey,
		Nonce:      baseNonce,
		To:         &toAddr,
		Value:      big.NewInt(1),
		Gas:        21_000,
		GasPrice:   gasPrice,
	})
	tx2 := node.SendLegacyTxWithParams(t, evmtest.LegacyTxParams{
		PrivateKey: privateKey,
		Nonce:      baseNonce + 2,
		To:         &toAddr,
		Value:      big.NewInt(2),
		Gas:        21_000,
		GasPrice:   gasPrice,
	})

	receipt0 := node.WaitForReceipt(t, tx0, 40*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt0, tx0)

	block0 := evmtest.MustUint64HexField(t, receipt0, "blockNumber")
	node.WaitForBlockNumberAtLeast(t, block0+2, 20*time.Second)
	assertReceiptStaysUnavailable(t, node.RPCURL(), tx2, 5*time.Second)

	tx1 := node.SendLegacyTxWithParams(t, evmtest.LegacyTxParams{
		PrivateKey: privateKey,
		Nonce:      baseNonce + 1,
		To:         &toAddr,
		Value:      big.NewInt(3),
		Gas:        21_000,
		GasPrice:   gasPrice,
	})

	receipt1 := node.WaitForReceipt(t, tx1, 40*time.Second)
	receipt2 := node.WaitForReceipt(t, tx2, 40*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt1, tx1)
	evmtest.AssertReceiptMatchesTxHash(t, receipt2, tx2)

	block1 := evmtest.MustUint64HexField(t, receipt1, "blockNumber")
	block2 := evmtest.MustUint64HexField(t, receipt2, "blockNumber")
	index1 := evmtest.MustUint64HexField(t, receipt1, "transactionIndex")
	index2 := evmtest.MustUint64HexField(t, receipt2, "transactionIndex")

	if block2 < block1 || (block2 == block1 && index2 <= index1) {
		t.Fatalf("nonce ordering violated after promotion: nonce+1 at %d/%d nonce+2 at %d/%d", block1, index1, block2, index2)
	}
}
