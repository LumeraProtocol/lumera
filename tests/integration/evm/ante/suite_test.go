//go:build integration
// +build integration

package ante_test

import (
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
)

// TestAnteSuite runs Cosmos-path ante integration tests against a single node
// fixture to reduce repeated startup overhead.
func TestAnteSuite(t *testing.T) {
	node := evmtest.NewEVMNode(t, "lumera-ante-suite", 500)
	node.StartAndWaitRPC()
	defer node.Stop()

	run := func(name string, fn func(t *testing.T, node *evmtest.Node)) {
		t.Run(name, func(t *testing.T) {
			latest := node.MustGetBlockNumber(t)
			node.WaitForBlockNumberAtLeast(t, latest+1, 20*time.Second)
			fn(t, node)
		})
	}

	run("CosmosTxFeeEnforcement", func(t *testing.T, node *evmtest.Node) {
		testCosmosTxFeeEnforcement(t, node)
	})
	run("AuthzGenericGrantRejectsBlockedMsgTypes", func(t *testing.T, node *evmtest.Node) {
		testAuthzGenericGrantRejectsBlockedMsgTypes(t, node)
	})
	run("AuthzGenericGrantAllowsNonBlockedMsgType", func(t *testing.T, node *evmtest.Node) {
		testAuthzGenericGrantAllowsNonBlockedMsgType(t, node)
	})
}
