//go:build integration
// +build integration

package contracts_test

import (
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
)

// TestConcurrentMixedEVMOperations spawns goroutines that simultaneously
// perform different EVM operations (simple transfers, contract deployments,
// contract calls) from the same sender account and verifies:
// - No panics or deadlocks in the node
// - All operations eventually complete (either mined or rejected)
// - Final nonce is consistent with the number of mined txs
func TestConcurrentMixedEVMOperations(t *testing.T) {
	node := evmtest.NewEVMNode(t, "lumera-concurrent-ops", 600)
	node.StartAndWaitRPC()
	defer node.Stop()

	fromAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	privateKey := evmtest.MustDerivePrivateKey(t, node.KeyInfo().Mnemonic)
	gasPrice := node.MustGetGasPriceWithRetry(t, 20*time.Second)
	baseNonce := node.MustGetPendingNonceWithRetry(t, fromAddr.Hex(), 20*time.Second)
	toAddr := fromAddr

	type opResult struct {
		kind  string
		nonce uint64
		hash  string
		err   error
	}

	// 3 simple transfers + 2 contract deployments = 5 concurrent operations.
	const numOps = 5
	results := make([]opResult, numOps)
	var wg sync.WaitGroup
	start := make(chan struct{})

	// Simple transfers (nonces 0, 1, 2).
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			nonce := baseNonce + uint64(idx)
			hash, err := evmtest.SendLegacyTxWithParamsResult(node.RPCURL(), evmtest.LegacyTxParams{
				PrivateKey: privateKey,
				Nonce:      nonce,
				To:         &toAddr,
				Value:      big.NewInt(int64(idx + 1)),
				Gas:        21_000,
				GasPrice:   gasPrice,
			})
			results[idx] = opResult{kind: "transfer", nonce: nonce, hash: hash, err: err}
		}(i)
	}

	// Contract deployments (nonces 3, 4).
	for i := 3; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			nonce := baseNonce + uint64(idx)
			code := calleeReturns99CreationCode()
			hash, err := evmtest.SendLegacyTxWithParamsResult(node.RPCURL(), evmtest.LegacyTxParams{
				PrivateKey: privateKey,
				Nonce:      nonce,
				To:         nil,
				Value:      big.NewInt(0),
				Gas:        500_000,
				GasPrice:   gasPrice,
				Data:       code,
			})
			results[idx] = opResult{kind: "deploy", nonce: nonce, hash: hash, err: err}
		}(i)
	}

	close(start) // Release all goroutines simultaneously.
	wg.Wait()

	// Verify results: count accepted txs and wait for receipts.
	var accepted int
	for _, r := range results {
		if r.err != nil {
			t.Logf("op %s nonce=%d rejected: %v", r.kind, r.nonce, r.err)
			continue
		}
		accepted++

		receipt := node.WaitForReceipt(t, r.hash, 60*time.Second)
		evmtest.AssertReceiptMatchesTxHash(t, receipt, r.hash)

		status := evmtest.MustStringField(t, receipt, "status")
		if !strings.EqualFold(status, "0x1") {
			t.Fatalf("op %s nonce=%d tx=%s failed with status %s", r.kind, r.nonce, r.hash, status)
		}
	}

	if accepted == 0 {
		t.Fatal("all concurrent operations were rejected — expected at least some to succeed")
	}

	// Verify nonce consistency: final nonce should equal baseNonce + accepted.
	finalNonce := node.MustGetPendingNonceWithRetry(t, fromAddr.Hex(), 20*time.Second)
	expectedNonce := baseNonce + uint64(accepted)
	if finalNonce != expectedNonce {
		t.Fatalf("nonce mismatch after concurrent ops: got %d want %d (base=%d accepted=%d)",
			finalNonce, expectedNonce, baseNonce, accepted)
	}

	t.Logf("concurrent mixed ops: %d/%d accepted, final nonce=%d", accepted, numOps, finalNonce)
}
