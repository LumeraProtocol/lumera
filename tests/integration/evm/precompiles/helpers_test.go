//go:build integration
// +build integration

package precompiles_test

import (
	"math/big"
	"strings"
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// mustEthCallPrecompile executes an eth_call against a precompile address and
// returns decoded output bytes.
func mustEthCallPrecompile(t *testing.T, node *evmtest.Node, to string, input []byte) []byte {
	t.Helper()

	var resultHex string
	node.MustJSONRPC(t, "eth_call", []any{
		map[string]any{
			"to":   to,
			"data": hexutil.Encode(input),
		},
		"latest",
	}, &resultHex)

	if strings.TrimSpace(resultHex) == "" {
		t.Fatalf("eth_call returned empty response for precompile %s", to)
	}

	resultBz, err := hexutil.Decode(resultHex)
	if err != nil {
		t.Fatalf("decode eth_call result %q: %v", resultHex, err)
	}

	return resultBz
}

// sendPrecompileLegacyTx signs and broadcasts a legacy tx that calls a
// precompile contract and returns its tx hash.
func sendPrecompileLegacyTx(
	t *testing.T,
	node *evmtest.Node,
	to string,
	input []byte,
	gasLimit uint64,
) string {
	t.Helper()

	keyInfo := node.KeyInfo()
	fromAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, keyInfo)
	privateKey := evmtest.MustDerivePrivateKey(t, keyInfo.Mnemonic)
	nonce := node.MustGetPendingNonceWithRetry(t, fromAddr.Hex(), 20*time.Second)
	gasPrice := node.MustGetGasPriceWithRetry(t, 20*time.Second)
	toAddr := common.HexToAddress(to)

	return node.SendLegacyTxWithParams(t, evmtest.LegacyTxParams{
		PrivateKey: privateKey,
		Nonce:      nonce,
		To:         &toAddr,
		Value:      big.NewInt(0),
		Gas:        gasLimit,
		GasPrice:   gasPrice,
		Data:       input,
	})
}
