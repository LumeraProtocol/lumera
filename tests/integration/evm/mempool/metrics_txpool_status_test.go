//go:build integration
// +build integration

package mempool_test

import (
	"context"
	"math/big"
	"strconv"
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	testjsonrpc "github.com/LumeraProtocol/lumera/testutil/jsonrpc"
	"github.com/stretchr/testify/require"
)

// TestTxPoolStatusReflectsPendingAndQueued verifies that the txpool_status
// JSON-RPC endpoint accurately reports pending and queued transaction counts.
// This validates the same underlying txpool.Stats() data that the Prometheus
// metrics (lumera_evm_mempool_pending, lumera_evm_mempool_queued) expose.
func TestTxPoolStatusReflectsPendingAndQueued(t *testing.T) {
	node := evmtest.NewEVMNode(t, "lumera-mempool-metrics", 600)
	node.AppendStartArgs("--json-rpc.api", "eth,txpool,net,web3")
	node.StartAndWaitRPC()
	defer node.Stop()

	fromAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	privateKey := evmtest.MustDerivePrivateKey(t, node.KeyInfo().Mnemonic)
	gasPrice := node.MustGetGasPriceWithRetry(t, 20*time.Second)
	baseNonce := node.MustGetPendingNonceWithRetry(t, fromAddr.Hex(), 20*time.Second)
	toAddr := fromAddr

	// --- Baseline: txpool should be empty ---
	status := mustTxPoolStatusWithRetry(t, node.RPCURL(), 20*time.Second)
	pendingBefore := mustParseHexInt(t, status["pending"])
	queuedBefore := mustParseHexInt(t, status["queued"])
	t.Logf("baseline txpool_status: pending=%d, queued=%d", pendingBefore, queuedBefore)

	// --- Submit 3 sequential txs (nonce 0,1,2) → all should become pending ---
	for i := uint64(0); i < 3; i++ {
		_, err := evmtest.SendLegacyTxWithParamsResult(node.RPCURL(), evmtest.LegacyTxParams{
			PrivateKey: privateKey,
			Nonce:      baseNonce + i,
			To:         &toAddr,
			Value:      big.NewInt(1),
			Gas:        21_000,
			GasPrice:   gasPrice,
		})
		require.NoError(t, err, "tx nonce=%d should be accepted", baseNonce+i)
	}

	// Poll txpool_status until pending increases (txs may take a moment to
	// propagate through CheckTx into the pool).
	require.Eventually(t, func() bool {
		s := mustTxPoolStatusWithRetry(t, node.RPCURL(), 5*time.Second)
		p := mustParseHexInt(t, s["pending"])
		return p > pendingBefore
	}, 15*time.Second, 500*time.Millisecond, "pending count should increase after submitting txs")

	status = mustTxPoolStatusWithRetry(t, node.RPCURL(), 5*time.Second)
	pendingAfter := mustParseHexInt(t, status["pending"])
	t.Logf("after 3 txs: pending=%d (was %d)", pendingAfter, pendingBefore)
	require.Greater(t, pendingAfter, pendingBefore, "pending count must increase after submitting sequential txs")

	// --- Submit a tx with a nonce gap → should become queued ---
	gapNonce := baseNonce + 100 // large gap
	_, err := evmtest.SendLegacyTxWithParamsResult(node.RPCURL(), evmtest.LegacyTxParams{
		PrivateKey: privateKey,
		Nonce:      gapNonce,
		To:         &toAddr,
		Value:      big.NewInt(1),
		Gas:        21_000,
		GasPrice:   gasPrice,
	})
	require.NoError(t, err, "nonce-gap tx should be accepted into queued pool")

	// Poll for queued count increase.
	require.Eventually(t, func() bool {
		s := mustTxPoolStatusWithRetry(t, node.RPCURL(), 5*time.Second)
		q := mustParseHexInt(t, s["queued"])
		return q > queuedBefore
	}, 15*time.Second, 500*time.Millisecond, "queued count should increase after submitting nonce-gap tx")

	status = mustTxPoolStatusWithRetry(t, node.RPCURL(), 5*time.Second)
	queuedAfter := mustParseHexInt(t, status["queued"])
	t.Logf("after nonce-gap tx: queued=%d (was %d)", queuedAfter, queuedBefore)
	require.Greater(t, queuedAfter, queuedBefore, "queued count must increase after submitting nonce-gap tx")
}

// TestTxPoolStatusOverflowKeepsPoolBounded verifies that flooding a
// low-capacity mempool results in RPC-level rejections and that
// txpool_status reflects the bounded pool size (not all burst txs accepted).
func TestTxPoolStatusOverflowKeepsPoolBounded(t *testing.T) {
	node := evmtest.NewEVMNode(t, "lumera-mempool-rej", 600)
	evmtest.SetMempoolMaxTxsInAppToml(t, node.HomeDir(), 4)
	evmtest.SetCometMempoolSize(t, node.HomeDir(), 4)
	node.AppendStartArgs("--json-rpc.api", "eth,txpool,net,web3")
	node.StartAndWaitRPC()
	defer node.Stop()

	fromAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	privateKey := evmtest.MustDerivePrivateKey(t, node.KeyInfo().Mnemonic)
	gasPrice := node.MustGetGasPriceWithRetry(t, 20*time.Second)
	baseNonce := node.MustGetPendingNonceWithRetry(t, fromAddr.Hex(), 20*time.Second)
	toAddr := fromAddr

	const burst = 20
	var rejected int
	for i := uint64(0); i < burst; i++ {
		_, err := evmtest.SendLegacyTxWithParamsResult(node.RPCURL(), evmtest.LegacyTxParams{
			PrivateKey: privateKey,
			Nonce:      baseNonce + i,
			To:         &toAddr,
			Value:      big.NewInt(1),
			Gas:        21_000,
			GasPrice:   gasPrice,
		})
		if err != nil {
			rejected++
		}
	}

	require.Greater(t, rejected, 0, "at least one tx must be rejected by the bounded mempool")
	t.Logf("mempool rejected %d of %d txs", rejected, burst)

	// txpool_status should show the pool is bounded (not all burst txs accepted).
	status := mustTxPoolStatusWithRetry(t, node.RPCURL(), 10*time.Second)
	total := mustParseHexInt(t, status["pending"]) + mustParseHexInt(t, status["queued"])
	require.Less(t, total, uint64(burst), "mempool total must be less than burst size due to rejections")
}

func mustParseHexInt(t *testing.T, hex string) uint64 {
	t.Helper()
	// Strip 0x prefix if present.
	s := hex
	if len(s) >= 2 && s[:2] == "0x" {
		s = s[2:]
	}
	v, err := strconv.ParseUint(s, 16, 64)
	require.NoError(t, err, "failed to parse hex %q", hex)
	return v
}

// mustTxPoolInspect calls txpool_inspect and returns the result for diagnostic logging.
func mustTxPoolInspect(t *testing.T, rpcURL string) map[string]map[string]map[string]string {
	t.Helper()
	var result map[string]map[string]map[string]string
	err := testjsonrpc.Call(context.Background(), rpcURL, "txpool_inspect", []any{}, &result)
	if err != nil {
		t.Logf("txpool_inspect failed (non-fatal): %v", err)
		return nil
	}
	return result
}
