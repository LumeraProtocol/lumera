//go:build integration
// +build integration

package precisebank_test

import (
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
)

// TestPrecisebankSuite runs precisebank integration scenarios against a shared
// node fixture to reduce startup overhead.
func TestPrecisebankSuite(t *testing.T) {
	node := evmtest.NewEVMNode(t, "lumera-precisebank-suite", 800)
	node.StartAndWaitRPC()
	defer node.Stop()

	run := func(name string, fn func(t *testing.T, node *evmtest.Node)) {
		t.Run(name, func(t *testing.T) {
			latest := evmtest.MustGetBlockNumber(t, node.RPCURL())
			evmtest.WaitForBlockNumberAtLeast(t, node.RPCURL(), latest+1, 20*time.Second)
			fn(t, node)
		})
	}

	run("PreciseBankFractionalBalanceQueryMatrix", func(t *testing.T, node *evmtest.Node) {
		testPreciseBankFractionalBalanceQueryMatrix(t, node)
	})
	run("PreciseBankFractionalBalanceRejectsInvalidAddress", func(t *testing.T, node *evmtest.Node) {
		testPreciseBankFractionalBalanceRejectsInvalidAddress(t, node)
	})
	run("PreciseBankEVMTransferSendSplitMatrix", func(t *testing.T, node *evmtest.Node) {
		testPreciseBankEVMTransferSendSplitMatrix(t, node)
	})
	run("PreciseBankSecondarySenderBurnMintWorkflow", func(t *testing.T, node *evmtest.Node) {
		testPreciseBankSecondarySenderBurnMintWorkflow(t, node)
	})
}
