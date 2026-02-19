//go:build integration
// +build integration

package mempool_test

import (
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
)

// TestMempoolSuite runs default app-side mempool behavior checks against one
// shared node. Tests that require custom startup config stay standalone.
func TestMempoolSuite(t *testing.T) {
	node := evmtest.NewEVMNode(t, "lumera-mempool-suite", 600)
	node.StartAndWaitRPC()
	defer node.Stop()

	run := func(name string, fn func(t *testing.T, node *evmtest.Node)) {
		t.Run(name, func(t *testing.T) {
			latest := evmtest.MustGetBlockNumber(t, node.RPCURL())
			evmtest.WaitForBlockNumberAtLeast(t, node.RPCURL(), latest+1, 20*time.Second)
			fn(t, node)
		})
	}

	run("DeterministicOrderingUnderContention", func(t *testing.T, node *evmtest.Node) {
		testDeterministicOrderingUnderContention(t, node)
	})
	run("EVMFeePriorityOrderingSameBlock", func(t *testing.T, node *evmtest.Node) {
		testEVMFeePriorityOrderingSameBlock(t, node)
	})
	run("PendingTxSubscriptionEmitsHash", func(t *testing.T, node *evmtest.Node) {
		testPendingTxSubscriptionEmitsHash(t, node)
	})
	run("NonceGapPromotionAfterGapFilled", func(t *testing.T, node *evmtest.Node) {
		testNonceGapPromotionAfterGapFilled(t, node)
	})
}
