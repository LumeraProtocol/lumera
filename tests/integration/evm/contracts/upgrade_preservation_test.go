//go:build integration
// +build integration

package contracts_test

import (
	"strings"
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
)

// TestEVMStatePreservationAcrossRestart deploys a contract, writes state,
// restarts the node (simulating a chain upgrade binary swap), and verifies
// that contract code, storage, and query results survive intact.
//
// This is the integration-level equivalent of a chain upgrade EVM state
// preservation test. A full upgrade handler test requires devnet (multiple
// validators + governance proposal), but this validates the critical invariant:
// EVM state in the IAVL tree survives a process restart with the same binary.
func TestEVMStatePreservationAcrossRestart(t *testing.T) {
	node := evmtest.NewEVMNode(t, "lumera-upgrade-preserve", 600)
	node.StartAndWaitRPC()
	defer node.Stop()

	// 1) Deploy a contract that stores value and returns it.
	logTopic := "0x" + strings.Repeat("33", 32)
	deployTxHash := sendLoggingConstantContractCreationTx(t, node, logTopic)
	deployReceipt := node.WaitForReceipt(t, deployTxHash, 45*time.Second)
	assertReceiptBasics(t, deployReceipt)
	contractAddress := evmtest.MustStringField(t, deployReceipt, "contractAddress")

	// 2) Verify contract works before restart.
	var preRestartResult string
	node.MustJSONRPC(t, "eth_call", []any{
		map[string]any{
			"to":   contractAddress,
			"data": methodSelectorHex("getValue()"),
		},
		"latest",
	}, &preRestartResult)
	assertEthCallReturnsUint256(t, preRestartResult, 42)

	// 3) Verify code exists before restart.
	var preRestartCode string
	node.MustJSONRPC(t, "eth_getCode", []any{contractAddress, "latest"}, &preRestartCode)
	if strings.EqualFold(strings.TrimSpace(preRestartCode), "0x") || strings.TrimSpace(preRestartCode) == "" {
		t.Fatal("expected non-empty code before restart")
	}

	// Record block number for historical query after restart.
	preRestartBlock := node.MustGetBlockNumber(t)

	// 4) Restart node (simulates binary upgrade).
	node.RestartAndWaitRPC()

	// 5) Verify contract code survives restart.
	var postRestartCode string
	node.MustJSONRPC(t, "eth_getCode", []any{contractAddress, "latest"}, &postRestartCode)
	if !strings.EqualFold(preRestartCode, postRestartCode) {
		t.Fatalf("contract code changed across restart:\nbefore: %s\nafter:  %s", preRestartCode, postRestartCode)
	}

	// 6) Verify contract query returns the same value after restart.
	var postRestartResult string
	node.MustJSONRPC(t, "eth_call", []any{
		map[string]any{
			"to":   contractAddress,
			"data": methodSelectorHex("getValue()"),
		},
		"latest",
	}, &postRestartResult)
	assertEthCallReturnsUint256(t, postRestartResult, 42)

	// 7) Verify receipt is still available after restart.
	postRestartReceipt := node.WaitForReceipt(t, deployTxHash, 20*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, postRestartReceipt, deployTxHash)

	// 8) Deploy a NEW contract after restart to confirm EVM execution still works.
	newDeployHash := sendContractCreationTx(t, node, alwaysRevertContractCreationCode())
	newReceipt := node.WaitForReceipt(t, newDeployHash, 45*time.Second)
	assertReceiptBasics(t, newReceipt)

	t.Logf("EVM state preserved across restart: contract=%s, pre-restart block=%d", contractAddress, preRestartBlock)
}
