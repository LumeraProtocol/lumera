//go:build integration
// +build integration

package ante_test

import (
	"strings"
	"testing"
	"time"

	lcfg "github.com/LumeraProtocol/lumera/config"
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testtext "github.com/LumeraProtocol/lumera/pkg/text"
)

// TestCosmosTxFeeEnforcement validates Cosmos-path fee checks with EVM ante enabled.
//
// Workflow:
// 1. Start a single-node EVM test chain and wait for first block.
// 2. Broadcast an intentionally underpriced bank tx and assert fee rejection.
// 3. Broadcast a sufficiently funded bank tx and assert inclusion on chain.
func testCosmosTxFeeEnforcement(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	// Dynamic fee ante checks must reject an underpriced Cosmos tx.
	lowFeeResp := mustBroadcastBankSend(t, node, node.KeyInfo().Address, "1"+lcfg.ChainDenom, "0"+lcfg.ChainDenom)
	lowFeeCode := txResponseCode(lowFeeResp)
	if lowFeeCode == 0 {
		t.Fatalf("expected insufficient-fee rejection, got success response: %#v", lowFeeResp)
	}
	lowFeeLog := strings.ToLower(txResponseRawLog(lowFeeResp))
	if !testtext.ContainsAny(lowFeeLog, "insufficient fee", "gas prices too low", "provided fee < minimum global fee") {
		t.Fatalf("expected insufficient fee error in raw log, got: %s", txResponseRawLog(lowFeeResp))
	}

	// A sufficiently priced Cosmos tx should pass CheckTx and be included.
	okResp := mustBroadcastBankSend(t, node, node.KeyInfo().Address, "1"+lcfg.ChainDenom, "2000000"+lcfg.ChainDenom)
	okCode := txResponseCode(okResp)
	if okCode != 0 {
		t.Fatalf("expected successful bank tx, got code=%d resp=%#v", okCode, okResp)
	}
	txHash := mustTxHash(t, okResp)
	evmtest.WaitForCosmosTxHeight(t, node, txHash, 40*time.Second)
}
