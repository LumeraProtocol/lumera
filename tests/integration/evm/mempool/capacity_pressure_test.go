//go:build integration
// +build integration

package mempool_test

import (
	"errors"
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	testjsonrpc "github.com/LumeraProtocol/lumera/testutil/jsonrpc"
)

// testMempoolCapacityRejectsOverflow floods the mempool with transactions from
// many distinct accounts until a send is rejected, confirming the node enforces
// its max-txs limit rather than accepting unbounded pending transactions.
//
// Because the default max-txs is 5000 which is too many for a test, this test
// uses a custom node with a very low capacity so the overflow is fast to reach.
func TestMempoolCapacityRejectsOverflow(t *testing.T) {
	// Standalone node because we need a tiny mempool capacity.
	node := evmtest.NewEVMNode(t, "lumera-mempool-cap", 600)
	evmtest.SetMempoolMaxTxsInAppToml(t, node.HomeDir(), 4)
	node.StartAndWaitRPC()
	defer node.Stop()

	fromAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	privateKey := evmtest.MustDerivePrivateKey(t, node.KeyInfo().Mnemonic)
	gasPrice := node.MustGetGasPriceWithRetry(t, 20*time.Second)
	baseNonce := node.MustGetPendingNonceWithRetry(t, fromAddr.Hex(), 20*time.Second)
	toAddr := fromAddr

	// Submit more txs than the mempool can hold. We deliberately overshoot
	// (send 20) to guarantee at least one rejection.
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

	if rejected == 0 {
		t.Fatal("expected at least one rejection from overflowed mempool, all txs were accepted")
	}
	t.Logf("mempool rejected %d of %d txs as expected", rejected, burst)
}

// testRapidReplacementRace spawns concurrent goroutines that try to replace the
// same nonce with escalating gas prices, verifying no panics/deadlocks and that
// exactly one replacement is ultimately mined.
func testRapidReplacementRace(t *testing.T, node *evmtest.Node) {
	t.Helper()

	fromAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	privateKey := evmtest.MustDerivePrivateKey(t, node.KeyInfo().Mnemonic)
	nonce := node.MustGetPendingNonceWithRetry(t, fromAddr.Hex(), 20*time.Second)
	gasPrice := node.MustGetGasPriceWithRetry(t, 20*time.Second)
	toAddr := fromAddr

	const racers = 5
	type result struct {
		hash string
		err  error
	}
	results := make([]result, racers)

	var wg sync.WaitGroup
	wg.Add(racers)
	for i := 0; i < racers; i++ {
		go func(idx int) {
			defer wg.Done()
			// Each racer uses an escalating gas price so at least one can replace.
			bump := new(big.Int).Mul(gasPrice, big.NewInt(int64(idx+1)*10))
			h, err := evmtest.SendLegacyTxWithParamsResult(node.RPCURL(), evmtest.LegacyTxParams{
				PrivateKey: privateKey,
				Nonce:      nonce,
				To:         &toAddr,
				Value:      big.NewInt(int64(idx + 1)),
				Gas:        21_000,
				GasPrice:   bump,
			})
			results[idx] = result{hash: h, err: err}
		}(i)
	}
	wg.Wait()

	// Collect accepted hashes and verify at least one succeeded.
	var accepted []string
	var rejectedCount int
	for _, r := range results {
		if r.err != nil {
			var rpcErr *testjsonrpc.RPCError
			if errors.As(r.err, &rpcErr) {
				msg := strings.ToLower(rpcErr.Message)
				if strings.Contains(msg, "underpriced") || strings.Contains(msg, "known") ||
					strings.Contains(msg, "replacement") || strings.Contains(msg, "nonce") {
					rejectedCount++
					continue
				}
			}
			t.Fatalf("unexpected error from racer: %v", r.err)
		}
		accepted = append(accepted, r.hash)
	}

	if len(accepted) == 0 {
		t.Fatal("all concurrent replacement attempts were rejected; expected at least one to succeed")
	}
	t.Logf("rapid replacement race: %d accepted, %d rejected", len(accepted), rejectedCount)

	// Wait for the last accepted tx to be mined.
	lastHash := accepted[len(accepted)-1]
	receipt := node.WaitForReceipt(t, lastHash, 40*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, lastHash)
}
