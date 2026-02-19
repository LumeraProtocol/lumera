//go:build integration
// +build integration

package jsonrpc_test

import (
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	"math/big"
	"reflect"
	"testing"
	"time"

	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	"github.com/ethereum/go-ethereum/crypto"
)

// testMixedBlockOrderingPersistsAcrossRestart verifies stable block tx ordering
// for mixed Cosmos+EVM blocks across node restart.
func testMixedBlockOrderingPersistsAcrossRestart(t *testing.T, node *evmtest.Node) {
	t.Helper()

	// Use a dedicated EVM sender to avoid nonce coupling with validator Cosmos txs.
	validatorAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	validatorPriv := evmtest.MustDerivePrivateKey(t, node.KeyInfo().Mnemonic)
	evmSenderPriv, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate evm sender key: %v", err)
	}
	evmSenderAddr := crypto.PubkeyToAddress(evmSenderPriv.PublicKey)

	fundNonce := evmtest.MustGetPendingNonceWithRetry(t, node.RPCURL(), validatorAddr.Hex(), 20*time.Second)
	fundGasPrice := evmtest.MustGetGasPriceWithRetry(t, node.RPCURL(), 20*time.Second)
	fundHash := evmtest.SendLegacyTxWithParams(t, node.RPCURL(), evmtest.LegacyTxParams{
		PrivateKey: validatorPriv,
		Nonce:      fundNonce,
		To:         &evmSenderAddr,
		Value:      big.NewInt(200_000_000_000_000),
		Gas:        21_000,
		GasPrice:   fundGasPrice,
	})
	evmtest.WaitForReceipt(t, node.RPCURL(), fundHash, node.WaitCh(), node.OutputBuffer(), 40*time.Second)

	var (
		targetHeight uint64
		beforeTxs    []string
	)

	// Build a block that contains both Cosmos and EVM txs.
	for attempt := 0; attempt < 8; attempt++ {
		cosmosHash := evmtest.SendOneCosmosBankTx(t, node)
		evmNonce := evmtest.MustGetPendingNonceWithRetry(t, node.RPCURL(), evmSenderAddr.Hex(), 20*time.Second)
		gasPrice := evmtest.MustGetGasPriceWithRetry(t, node.RPCURL(), 20*time.Second)
		ethHash := evmtest.SendLegacyTxWithParams(t, node.RPCURL(), evmtest.LegacyTxParams{
			PrivateKey: evmSenderPriv,
			Nonce:      evmNonce,
			To:         &evmSenderAddr,
			Value:      big.NewInt(1),
			Gas:        21_000,
			GasPrice:   gasPrice,
		})

		receipt := evmtest.WaitForReceipt(t, node.RPCURL(), ethHash, node.WaitCh(), node.OutputBuffer(), 40*time.Second)
		ethHeight := evmtest.MustUint64HexField(t, receipt, "blockNumber")
		cosmosHeight := evmtest.WaitForCosmosTxHeight(t, node, cosmosHash, 40*time.Second)

		if cosmosHeight != ethHeight {
			continue
		}

		beforeTxs = evmtest.MustGetCometBlockTxs(t, node, ethHeight)
		if len(beforeTxs) < 2 {
			t.Fatalf("expected mixed block to contain at least 2 txs, got %d", len(beforeTxs))
		}

		targetHeight = ethHeight
		break
	}

	if targetHeight == 0 {
		t.Fatalf("failed to create a mixed Cosmos+EVM block after retries")
	}

	node.RestartAndWaitRPC()

	afterTxs := evmtest.MustGetCometBlockTxs(t, node, targetHeight)
	if !reflect.DeepEqual(beforeTxs, afterTxs) {
		t.Fatalf("block tx ordering changed across restart at height %d\nbefore=%v\nafter=%v", targetHeight, beforeTxs, afterTxs)
	}
}
