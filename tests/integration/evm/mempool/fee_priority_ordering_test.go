//go:build integration
// +build integration

package mempool_test

import (
	"context"
	"encoding/json"
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	"math/big"
	"testing"
	"time"

	lcfg "github.com/LumeraProtocol/lumera/config"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	testjsonrpc "github.com/LumeraProtocol/lumera/testutil/jsonrpc"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

// TestEVMFeePriorityOrderingSameBlock verifies higher-fee tx ordering precedence
// within the same block for distinct senders.
func testEVMFeePriorityOrderingSameBlock(t *testing.T, node *evmtest.Node) {
	t.Helper()

	senderAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	senderPriv := evmtest.MustDerivePrivateKey(t, node.KeyInfo().Mnemonic)

	receiverPriv, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate second key: %v", err)
	}
	receiverAddr := crypto.PubkeyToAddress(receiverPriv.PublicKey)

	// Wait until gas price is affordable for two 21k txs from the fixed test balance.
	lowGasPrice := waitForAffordableGasPrice(t, node, big.NewInt(2_200_000_000), 30*time.Second)
	highGasPrice := new(big.Int).Add(lowGasPrice, big.NewInt(100_000_000))

	highTxCost := new(big.Int).Mul(new(big.Int).Set(highGasPrice), big.NewInt(21_000))
	highTxCost.Add(highTxCost, big.NewInt(1))
	receiverFunding := new(big.Int).Add(highTxCost, big.NewInt(1_000_000_000_000))

	accCodec := addresscodec.NewBech32Codec(lcfg.Bech32AccountAddressPrefix)
	receiverBech32, err := accCodec.BytesToString(receiverAddr.Bytes())
	if err != nil {
		t.Fatalf("encode receiver bech32 address: %v", err)
	}
	fundAccountViaBankSend(t, node, receiverBech32, receiverFunding)

	nonce1 := evmtest.MustGetPendingNonceWithRetry(t, node.RPCURL(), senderAddr.Hex(), 20*time.Second)
	nonce2 := evmtest.MustGetPendingNonceWithRetry(t, node.RPCURL(), receiverAddr.Hex(), 20*time.Second)

	lowHash := evmtest.SendLegacyTxWithParams(t, node.RPCURL(), evmtest.LegacyTxParams{
		PrivateKey: senderPriv,
		Nonce:      nonce1,
		To:         &senderAddr,
		Value:      big.NewInt(1),
		Gas:        21_000,
		GasPrice:   lowGasPrice,
	})
	highHash := evmtest.SendLegacyTxWithParams(t, node.RPCURL(), evmtest.LegacyTxParams{
		PrivateKey: receiverPriv,
		Nonce:      nonce2,
		To:         &receiverAddr,
		Value:      big.NewInt(1),
		Gas:        21_000,
		GasPrice:   highGasPrice,
	})

	lowReceipt := evmtest.WaitForReceipt(t, node.RPCURL(), lowHash, node.WaitCh(), node.OutputBuffer(), 40*time.Second)
	highReceipt := evmtest.WaitForReceipt(t, node.RPCURL(), highHash, node.WaitCh(), node.OutputBuffer(), 40*time.Second)

	lowBlock := evmtest.MustUint64HexField(t, lowReceipt, "blockNumber")
	highBlock := evmtest.MustUint64HexField(t, highReceipt, "blockNumber")
	if lowBlock != highBlock {
		t.Fatalf("expected same-block inclusion, got low_block=%d high_block=%d", lowBlock, highBlock)
	}

	lowIndex := evmtest.MustUint64HexField(t, lowReceipt, "transactionIndex")
	highIndex := evmtest.MustUint64HexField(t, highReceipt, "transactionIndex")
	if highIndex >= lowIndex {
		t.Fatalf("higher-fee tx should be ordered first in block %d, got high_index=%d low_index=%d", highBlock, highIndex, lowIndex)
	}
}

// waitForAffordableGasPrice polls eth_gasPrice until the value is below a
// target ceiling so the test account can fund all required txs.
func waitForAffordableGasPrice(t *testing.T, node *evmtest.Node, maxGasPrice *big.Int, timeout time.Duration) *big.Int {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		gasPrice, err := gasPriceFromRPC(node.RPCURL())
		if err == nil && gasPrice.Cmp(maxGasPrice) <= 0 {
			return gasPrice
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for affordable gas price <= %s", maxGasPrice.String())
	return nil
}

// gasPriceFromRPC fetches and decodes the current eth_gasPrice value.
func gasPriceFromRPC(rpcURL string) (*big.Int, error) {
	var gasPriceHex string
	if err := testjsonrpc.Call(context.Background(), rpcURL, "eth_gasPrice", []any{}, &gasPriceHex); err != nil {
		return nil, err
	}
	return hexutil.DecodeBig(gasPriceHex)
}

// fundAccountViaBankSend sends native funds to a bech32 recipient so it can
// cover EVM tx fees in ordering tests.
func fundAccountViaBankSend(t *testing.T, node *evmtest.Node, recipient string, amount *big.Int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	output, err := evmtest.RunCommand(
		ctx,
		node.RepoRoot(),
		node.BinPath(),
		"tx", "bank", "send", "validator", recipient, amount.String()+lcfg.ChainDenom,
		"--home", node.HomeDir(),
		"--keyring-backend", "test",
		"--chain-id", node.ChainID(),
		"--node", node.CometRPCURL(),
		"--broadcast-mode", "async",
		"--gas", "200000",
		"--fees", "1000"+lcfg.ChainDenom,
		"--yes",
		"--output", "json",
		"--log_no_color",
	)
	if err != nil {
		t.Fatalf("broadcast bank send to %s: %v\n%s", recipient, err, output)
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("decode bank send response: %v\n%s", err, output)
	}

	if codeRaw, ok := resp["code"]; ok {
		if code, ok := codeRaw.(float64); ok && code != 0 {
			t.Fatalf("bank send checktx rejected with code %.0f: %#v", code, resp)
		}
	}

	txHash, ok := resp["txhash"].(string)
	if !ok || txHash == "" {
		t.Fatalf("missing txhash in bank send response: %#v", resp)
	}
	evmtest.WaitForCosmosTxHeight(t, node, txHash, 40*time.Second)
}
