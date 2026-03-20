//go:build integration
// +build integration

package mempool_test

import (
	"context"
	"errors"
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	"math/big"
	"strings"
	"testing"
	"time"

	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	testjsonrpc "github.com/LumeraProtocol/lumera/testutil/jsonrpc"
)

// TestNonceReplacementRequiresPriceBump verifies same-nonce replacement policy:
// unchanged fee is rejected, sufficiently bumped fee is accepted.
func TestNonceReplacementRequiresPriceBump(t *testing.T) {
	t.Helper()

	// Force deterministic replacement rule in the test node config.
	node := evmtest.NewEVMNode(t, "lumera-nonce-replacement", 240)
	evmtest.SetEVMMempoolPriceBumpInAppToml(t, node.HomeDir(), 15)
	node.StartAndWaitRPC()
	defer node.Stop()

	fromAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	privateKey := evmtest.MustDerivePrivateKey(t, node.KeyInfo().Mnemonic)
	nonce := node.MustGetPendingNonceWithRetry(t, fromAddr.Hex(), 20*time.Second)
	// Use node-reported gas price so tx fee clears the current base fee floor.
	gasPrice := node.MustGetGasPriceWithRetry(t, 20*time.Second)
	toAddr := fromAddr

	// First tx enters the pool with nonce N.
	firstHash := node.SendLegacyTxWithParams(t, evmtest.LegacyTxParams{
		PrivateKey: privateKey,
		Nonce:      nonce,
		To:         &toAddr,
		Value:      big.NewInt(1),
		Gas:        21_000,
		GasPrice:   gasPrice,
	})

	// Same nonce + same gas price must be rejected by replacement policy.
	_, err := evmtest.SendLegacyTxWithParamsResult(node.RPCURL(), evmtest.LegacyTxParams{
		PrivateKey: privateKey,
		Nonce:      nonce,
		To:         &toAddr,
		Value:      big.NewInt(2),
		Gas:        21_000,
		GasPrice:   gasPrice,
	})
	assertUnderpricedReplacementError(t, err)

	// Bumped fee replacement with same nonce should be accepted and mined.
	bumpedGasPrice := new(big.Int).Mul(gasPrice, big.NewInt(100))
	replacementHash := node.SendLegacyTxWithParams(t, evmtest.LegacyTxParams{
		PrivateKey: privateKey,
		Nonce:      nonce,
		To:         &toAddr,
		Value:      big.NewInt(3),
		Gas:        21_000,
		GasPrice:   bumpedGasPrice,
	})
	if strings.EqualFold(firstHash, replacementHash) {
		t.Fatalf("replacement tx hash unexpectedly equals original hash: %s", firstHash)
	}

	replacementReceipt := node.WaitForReceipt(t, replacementHash, 40*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, replacementReceipt, replacementHash)
	assertReceiptStaysUnavailable(t, node.RPCURL(), firstHash, 10*time.Second)
}

// assertUnderpricedReplacementError validates the JSON-RPC error shape for a
// replacement attempt that does not satisfy price-bump requirements.
func assertUnderpricedReplacementError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected replacement with non-bumped fee to fail, got nil error")
	}

	var rpcErr *testjsonrpc.RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected JSON-RPC error, got %T: %v", err, err)
	}

	msg := strings.ToLower(rpcErr.Message)
	if strings.Contains(msg, "underpriced") || strings.Contains(msg, "price bump") || strings.Contains(msg, "replacement") {
		return
	}

	t.Fatalf("unexpected replacement error message: %q", rpcErr.Message)
}

// assertReceiptStaysUnavailable ensures a replaced tx never reaches receipt status.
func assertReceiptStaysUnavailable(t *testing.T, rpcURL, txHash string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var receipt map[string]any
		err := testjsonrpc.Call(context.Background(), rpcURL, "eth_getTransactionReceipt", []any{txHash}, &receipt)
		if err == nil && receipt != nil {
			t.Fatalf("expected no receipt for replaced tx %s, got %#v", txHash, receipt)
		}
		time.Sleep(300 * time.Millisecond)
	}
}
