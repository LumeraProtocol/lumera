//go:build integration
// +build integration

package mempool_test

import (
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	"math/big"
	"sync"
	"testing"
	"time"

	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
)

// TestDeterministicOrderingUnderContention verifies nonce ordering remains
// deterministic when same-sender txs are submitted concurrently and out of order.
func testDeterministicOrderingUnderContention(t *testing.T, node *evmtest.Node) {
	t.Helper()

	fromAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	privateKey := evmtest.MustDerivePrivateKey(t, node.KeyInfo().Mnemonic)
	baseNonce := evmtest.MustGetPendingNonceWithRetry(t, node.RPCURL(), fromAddr.Hex(), 20*time.Second)
	gasPrice := evmtest.MustGetGasPriceWithRetry(t, node.RPCURL(), 20*time.Second)
	toAddr := fromAddr

	const totalTx = 6

	type sendResult struct {
		nonce uint64 // Nonce used to submit the tx.
		hash  string // Tx hash returned by broadcast.
		err   error  // Broadcast error, if any.
	}

	results := make(chan sendResult, totalTx)
	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < totalTx; i++ {
		nonce := baseNonce + uint64(totalTx-1-i) // reverse order to maximize contention
		wg.Add(1)
		go func(nonce uint64, value int64) {
			defer wg.Done()
			<-start
			hash, err := evmtest.SendLegacyTxWithParamsResult(node.RPCURL(), evmtest.LegacyTxParams{
				PrivateKey: privateKey,
				Nonce:      nonce,
				To:         &toAddr,
				Value:      big.NewInt(value),
				Gas:        21_000,
				GasPrice:   gasPrice,
			})
			results <- sendResult{nonce: nonce, hash: hash, err: err}
		}(nonce, int64(i+1))
	}

	close(start)
	wg.Wait()
	close(results)

	hashByNonce := make(map[uint64]string, totalTx)
	for result := range results {
		if result.err != nil {
			t.Fatalf("failed to submit tx nonce %d: %v", result.nonce, result.err)
		}
		hashByNonce[result.nonce] = result.hash
	}

	var (
		prevBlock uint64
		prevIndex uint64
		havePrev  bool
	)

	for i := 0; i < totalTx; i++ {
		nonce := baseNonce + uint64(i)
		txHash, ok := hashByNonce[nonce]
		if !ok {
			t.Fatalf("missing tx hash for nonce %d", nonce)
		}

		receipt := evmtest.WaitForReceipt(t, node.RPCURL(), txHash, node.WaitCh(), node.OutputBuffer(), 40*time.Second)
		evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)

		blockNum := evmtest.MustUint64HexField(t, receipt, "blockNumber")
		txIndex := evmtest.MustUint64HexField(t, receipt, "transactionIndex")

		if havePrev {
			if blockNum < prevBlock {
				t.Fatalf("nonce ordering regressed by block: nonce %d in block %d after previous block %d", nonce, blockNum, prevBlock)
			}
			if blockNum == prevBlock && txIndex <= prevIndex {
				t.Fatalf("nonce ordering regressed within block %d: nonce %d index %d after previous index %d", blockNum, nonce, txIndex, prevIndex)
			}
		}

		prevBlock = blockNum
		prevIndex = txIndex
		havePrev = true
	}
}
