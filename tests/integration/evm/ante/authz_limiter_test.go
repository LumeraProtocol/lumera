//go:build integration
// +build integration

package ante_test

import (
	"strings"
	"testing"
	"time"

	lcfg "github.com/LumeraProtocol/lumera/config"
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testtext "github.com/LumeraProtocol/lumera/testutil/text"
)

// TestAuthzGenericGrantRejectsBlockedMsgTypes verifies authz grant filtering in
// the Cosmos ante path.
//
// Matrix:
// - Generic grant for MsgEthereumTx should be rejected.
// - Generic grant for MsgCreateVestingAccount should be rejected.
func testAuthzGenericGrantRejectsBlockedMsgTypes(t *testing.T, node *evmtest.Node) {
	t.Helper()
	evmtest.WaitForBlockNumberAtLeast(t, node.RPCURL(), 1, 20*time.Second)

	grantee := mustAddKeyAddress(t, node, "grantee-reject")

	testCases := []string{
		"/cosmos.evm.vm.v1.MsgEthereumTx",
		"/cosmos.vesting.v1beta1.MsgCreateVestingAccount",
	}

	for _, msgType := range testCases {
		// Each case executes a full CLI tx path to exercise real ante wiring.
		resp, out, err := authzGrantResult(
			t,
			node,
			"validator",
			grantee,
			msgType,
			"2000000"+lcfg.ChainDenom,
		)
		if resp == nil {
			t.Fatalf("expected CLI json response for %s, got decode/run failure: %v\n%s", msgType, err, out)
		}

		code := txResponseCode(resp)
		if code == 0 {
			t.Fatalf("expected blocked authz grant for %s, got success response: %#v", msgType, resp)
		}

		rawLog := strings.ToLower(txResponseRawLog(resp))
		if rawLog == "" {
			rawLog = strings.ToLower(out)
		}
		if !testtext.ContainsAny(rawLog, "unauthorized", "disabled msg type", strings.ToLower(msgType)) {
			t.Fatalf("expected authz limiter error for %s, got output: %s", msgType, out)
		}
	}
}

// TestAuthzGenericGrantAllowsNonBlockedMsgType is the positive control for the
// authz limiter: a regular MsgSend authorization must still pass.
func testAuthzGenericGrantAllowsNonBlockedMsgType(t *testing.T, node *evmtest.Node) {
	t.Helper()
	evmtest.WaitForBlockNumberAtLeast(t, node.RPCURL(), 1, 20*time.Second)

	grantee := mustAddKeyAddress(t, node, "grantee-allow")
	resp := mustBroadcastAuthzGenericGrant(
		t,
		node,
		"validator",
		grantee,
		"/cosmos.bank.v1beta1.MsgSend",
		"2000000"+lcfg.ChainDenom,
	)
	code := txResponseCode(resp)
	if code != 0 {
		t.Fatalf("expected allowed authz grant, got code=%d resp=%#v", code, resp)
	}

	txHash := mustTxHash(t, resp)
	evmtest.WaitForCosmosTxHeight(t, node, txHash, 40*time.Second)
}

// authzGrantResult executes `tx authz grant ... generic` and returns parsed
// JSON response, raw command output, and process error.
func authzGrantResult(
	t *testing.T,
	node *evmtest.Node,
	from, grantee, msgType, fees string,
) (map[string]any, string, error) {
	t.Helper()

	return broadcastTxCommandResult(t, node,
		"tx", "authz", "grant", grantee, "generic",
		"--msg-type", msgType,
		"--from", from,
		"--home", node.HomeDir(),
		"--keyring-backend", "test",
		"--chain-id", node.ChainID(),
		"--node", node.CometRPCURL(),
		"--broadcast-mode", "sync",
		"--gas", "250000",
		"--fees", fees,
		"--yes",
		"--output", "json",
		"--log_no_color",
	)
}
