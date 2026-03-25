//go:build system_test

package system

import (
	"context"
	"fmt"
	"testing"

	client "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func submitCascadeClientFailureEvidence(t *testing.T, cli LumeradCli, fromNode, subjectAddr, actionID, metadataJSON string) string {
	t.Helper()
	tx := cli.CustomCommand(
		"tx", "audit", "submit-evidence",
		subjectAddr,
		"cascade-client-failure",
		actionID,
		metadataJSON,
		"--from", fromNode,
	)
	RequireTxSuccess(t, tx)
	return tx
}

func latestHeightAndAppHashAtHeight(t *testing.T, rpcAddr string, height int64) (int64, string) {
	t.Helper()
	httpClient, err := client.New(rpcAddr, "/websocket")
	require.NoError(t, err)
	require.NoError(t, httpClient.Start())
	defer func() { _ = httpClient.Stop() }()

	status, err := httpClient.Status(context.Background())
	require.NoError(t, err)
	latest := status.SyncInfo.LatestBlockHeight

	res, err := httpClient.Block(context.Background(), &height)
	require.NoError(t, err)
	return latest, fmt.Sprintf("%X", res.Block.Header.AppHash)
}

func assertChainProgressAndSingleAppHash(t *testing.T, blocks int) {
	t.Helper()
	nodes := sut.AllNodes(t)
	require.NotEmpty(t, nodes)

	lastMinHeight := int64(0)
	for i := 0; i < blocks; i++ {
		minHeight := int64(1<<62 - 1)
		for _, n := range nodes {
			rpc := fmt.Sprintf("tcp://localhost:%d", n.RPCPort)
			h, _ := latestHeightAndAppHashAtHeight(t, rpc, 1)
			if h < minHeight {
				minHeight = h
			}
		}
		require.Greater(t, minHeight, int64(0))

		var expectedHash string
		for _, n := range nodes {
			rpc := fmt.Sprintf("tcp://localhost:%d", n.RPCPort)
			_, hash := latestHeightAndAppHashAtHeight(t, rpc, minHeight)
			if expectedHash == "" {
				expectedHash = hash
				continue
			}
			require.Equal(t, expectedHash, hash, "app hash mismatch at height %d", minHeight)
		}

		if i > 0 {
			require.GreaterOrEqual(t, minHeight, lastMinHeight)
		}
		lastMinHeight = minHeight
		sut.AwaitNextBlock(t)
	}
	require.Greater(t, lastMinHeight, int64(1), "chain did not progress")
}

func queryEvidenceMetadataBase64ByAction(t *testing.T, cli LumeradCli, actionID string) string {
	t.Helper()
	out := cli.CustomQuery("q", "audit", "evidence-by-action", actionID)
	meta := gjson.Get(out, "evidence.0.metadata")
	if !meta.Exists() {
		meta = gjson.Get(out, "evidences.0.metadata")
	}
	require.True(t, meta.Exists(), "missing metadata in response: %s", out)
	return meta.String()
}

func bootFreshChain(t *testing.T) {
	t.Helper()
	sut.ResetChain(t)
	sut.StartChain(t)
	t.Cleanup(func() { sut.StopChain() })
}

func TestAuditEvidenceDeterminism_A_ChainProgressSingleAppHash(t *testing.T) {
	bootFreshChain(t)
	cli := NewLumeradCLI(t, sut, true)
	n0 := getNodeIdentity(t, cli, "node0")
	n1 := getNodeIdentity(t, cli, "node1")

	metadata := `{"reporter_component":2,"target_supernode_accounts":["lumera1mfldjaqc7ec5rlh4k58yttv3cd978gzl070zk6"],"details":{"action_id":"123637","error":"download failed: insufficient symbols","iteration":"1","operation":"download","supernode_account":"lumera1mfldjaqc7ec5rlh4k58yttv3cd978gzl070zk6","supernode_endpoint":"18.190.53.108:4444","task_id":"9700ec8a"}}`
	submitCascadeClientFailureEvidence(t, *cli, n0.nodeName, n1.accAddr, "sys-a-1", metadata)
	assertChainProgressAndSingleAppHash(t, 8)
}

func TestAuditEvidenceDeterminism_B_JSONPermutationStableMetadata(t *testing.T) {
	bootFreshChain(t)
	cli := NewLumeradCLI(t, sut, true)
	n0 := getNodeIdentity(t, cli, "node0")
	n1 := getNodeIdentity(t, cli, "node1")

	meta1 := `{"reporter_component":2,"target_supernode_accounts":["lumera1mfldjaqc7ec5rlh4k58yttv3cd978gzl070zk6"],"details":{"action_id":"123637","error":"download failed: insufficient symbols","iteration":"1","operation":"download","supernode_account":"lumera1mfldjaqc7ec5rlh4k58yttv3cd978gzl070zk6","supernode_endpoint":"18.190.53.108:4444","task_id":"9700ec8a"}}`
	meta2 := `{"reporter_component":2,"target_supernode_accounts":["lumera1mfldjaqc7ec5rlh4k58yttv3cd978gzl070zk6"],"details":{"task_id":"9700ec8a","supernode_endpoint":"18.190.53.108:4444","supernode_account":"lumera1mfldjaqc7ec5rlh4k58yttv3cd978gzl070zk6","operation":"download","iteration":"1","error":"download failed: insufficient symbols","action_id":"123637"}}`

	submitCascadeClientFailureEvidence(t, *cli, n0.nodeName, n1.accAddr, "sys-b-1", meta1)
	submitCascadeClientFailureEvidence(t, *cli, n0.nodeName, n1.accAddr, "sys-b-2", meta2)
	sut.AwaitNextBlock(t)

	m1 := queryEvidenceMetadataBase64ByAction(t, *cli, "sys-b-1")
	m2 := queryEvidenceMetadataBase64ByAction(t, *cli, "sys-b-2")
	require.Equal(t, m1, m2)
	assertChainProgressAndSingleAppHash(t, 6)
}

func TestAuditEvidenceDeterminism_C_ReplayAfterHeightTransition(t *testing.T) {
	bootFreshChain(t)
	cli := NewLumeradCLI(t, sut, true)
	n0 := getNodeIdentity(t, cli, "node0")
	n1 := getNodeIdentity(t, cli, "node1")
	metadata := `{"reporter_component":2,"target_supernode_accounts":["lumera1mfldjaqc7ec5rlh4k58yttv3cd978gzl070zk6"],"details":{"action_id":"123637","error":"download failed: insufficient symbols","iteration":"1","operation":"download","supernode_account":"lumera1mfldjaqc7ec5rlh4k58yttv3cd978gzl070zk6","supernode_endpoint":"18.190.53.108:4444","task_id":"9700ec8a"}}`

	submitCascadeClientFailureEvidence(t, *cli, n0.nodeName, n1.accAddr, "sys-c-1", metadata)
	assertChainProgressAndSingleAppHash(t, 4)

	// High-level replay across further block transitions.
	targetHeight := sut.AwaitNextBlock(t) + 8
	sut.AwaitBlockHeight(t, targetHeight)

	submitCascadeClientFailureEvidence(t, *cli, n0.nodeName, n1.accAddr, "sys-c-2", metadata)
	assertChainProgressAndSingleAppHash(t, 8)
}

func TestAuditEvidenceDeterminism_D_RestartKeepsDeterministicState(t *testing.T) {
	bootFreshChain(t)
	cli := NewLumeradCLI(t, sut, true)
	n0 := getNodeIdentity(t, cli, "node0")
	n1 := getNodeIdentity(t, cli, "node1")
	metadata := `{"reporter_component":2,"target_supernode_accounts":["lumera1mfldjaqc7ec5rlh4k58yttv3cd978gzl070zk6"],"details":{"action_id":"123637","error":"download failed: insufficient symbols","iteration":"1","operation":"download","supernode_account":"lumera1mfldjaqc7ec5rlh4k58yttv3cd978gzl070zk6","supernode_endpoint":"18.190.53.108:4444","task_id":"9700ec8a"}}`

	submitCascadeClientFailureEvidence(t, *cli, n0.nodeName, n1.accAddr, "sys-d-1", metadata)
	h := sut.AwaitNextBlock(t)
	nodes := sut.AllNodes(t)
	require.NotEmpty(t, nodes)
	_, beforeHash := latestHeightAndAppHashAtHeight(t, fmt.Sprintf("tcp://localhost:%d", nodes[0].RPCPort), h)

	// restart full validator set without reset and verify deterministic state remains consistent.
	sut.StopChain()
	sut.StartChain(t)
	sut.AwaitNodeUp(t, "tcp://localhost:26657")
	sut.AwaitBlockHeight(t, h+3)

	for _, n := range sut.AllNodes(t) {
		_, got := latestHeightAndAppHashAtHeight(t, fmt.Sprintf("tcp://localhost:%d", n.RPCPort), h)
		require.Equal(t, beforeHash, got)
	}
	assertChainProgressAndSingleAppHash(t, 6)
}

func TestAuditEvidenceDeterminism_E_ReservedEvidenceTypeRejectedChainContinues(t *testing.T) {
	bootFreshChain(t)
	cli := NewLumeradCLI(t, sut, true)
	n0 := getNodeIdentity(t, cli, "node0")
	n1 := getNodeIdentity(t, cli, "node1")

	// ACTION_EXPIRED is reserved for action module; direct msg submission must fail.
	resp := cli.CustomCommand(
		"tx", "audit", "submit-evidence",
		n1.accAddr,
		"action-expired",
		"sys-e-1",
		`{"top_10_validator_addresses":[]}`,
		"--from", n0.nodeName,
	)
	RequireTxFailure(t, resp, "reserved for the action module")
	assertChainProgressAndSingleAppHash(t, 6)
}
