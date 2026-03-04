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

// testMixedCosmosAndEVMTransactionsCanShareBlock validates that Cosmos and EVM
// transactions can co-exist in the same committed block.
//
// Workflow:
// 1. Fund a dedicated EVM sender.
// 2. Broadcast Cosmos tx + EVM tx in short succession.
// 3. Retry until both hashes are observed at the same height.
func testMixedCosmosAndEVMTransactionsCanShareBlock(t *testing.T, node *evmtest.Node) {
	t.Helper()
	// Use a dedicated EVM sender to avoid nonce coupling with validator Cosmos txs.
	validatorAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	validatorPriv := evmtest.MustDerivePrivateKey(t, node.KeyInfo().Mnemonic)
	evmSenderPriv, evmSenderAddr := testaccounts.MustGenerateEthKey(t)

	fundNonce := node.MustGetPendingNonceWithRetry(t, validatorAddr.Hex(), 20*time.Second)
	fundGasPrice := node.MustGetGasPriceWithRetry(t, 20*time.Second)
	fundHash := node.SendLegacyTxWithParams(t, evmtest.LegacyTxParams{
		PrivateKey: validatorPriv,
		Nonce:      fundNonce,
		To:         &evmSenderAddr,
		Value:      big.NewInt(200_000_000_000_000),
		Gas:        21_000,
		GasPrice:   fundGasPrice,
	})
	node.WaitForReceipt(t, fundHash, 40*time.Second)

	// Try a few rounds to reliably catch both tx types in the same block.
	for attempt := 0; attempt < 8; attempt++ {
		cosmosHash := evmtest.SendOneCosmosBankTx(t, node)
		evmNonce := node.MustGetPendingNonceWithRetry(t, evmSenderAddr.Hex(), 20*time.Second)
		gasPrice := node.MustGetGasPriceWithRetry(t, 20*time.Second)
		ethHash := node.SendLegacyTxWithParams(t, evmtest.LegacyTxParams{
			PrivateKey: evmSenderPriv,
			Nonce:      evmNonce,
			To:         &evmSenderAddr,
			Value:      big.NewInt(1),
			Gas:        21_000,
			GasPrice:   gasPrice,
		})

		receipt := node.WaitForReceipt(t, ethHash, 40*time.Second)
		ethHeight := evmtest.MustUint64HexField(t, receipt, "blockNumber")
		cosmosHeight := evmtest.WaitForCosmosTxHeight(t, node, cosmosHash, 40*time.Second)

		if cosmosHeight != ethHeight {
			continue
		}

		blockTxs := evmtest.MustGetCometBlockTxs(t, node, ethHeight)
		if len(blockTxs) < 2 {
			t.Fatalf("expected mixed block to contain at least 2 txs, got %d", len(blockTxs))
		}

		hashes := evmtest.CometTxHashesFromBase64(t, blockTxs)
		foundCosmos := false
		for _, h := range hashes {
			if strings.EqualFold(h, cosmosHash) {
				foundCosmos = true
				break
			}
		}
		if !foundCosmos {
			t.Fatalf("cosmos tx hash %s not found in block %d hashes %v", cosmosHash, ethHeight, hashes)
		}

		return
	}

	t.Fatalf("failed to observe mixed Cosmos+EVM tx inclusion in the same block after retries")
}
