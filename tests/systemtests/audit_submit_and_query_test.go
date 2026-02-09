//go:build system_test

package system

// This test validates the "happy path" end-to-end:
// - supernode registration
// - epoch report submission via CLI
// - querying stored report via gRPC
//
// Assertions:
// - the reporter's report is stored under (epoch_id, supernode_account)

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuditSubmitReportAndQuery(t *testing.T) {
	const (
		// Keep epochs long enough in real time to avoid end-blocker enforcement during the test.
		epochLengthBlocks = uint64(20)
	)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastEpochs(t, epochLengthBlocks, 1, 1, 1, []uint32{4444}),
	)
	sut.StartChain(t)

	cli := NewLumeradCLI(t, sut, true)
	n0 := getNodeIdentity(t, cli, "node0")
	n1 := getNodeIdentity(t, cli, "node1")

	registerSupernode(t, cli, n0, "192.168.1.1")
	registerSupernode(t, cli, n1, "192.168.1.2")

	// These nodes register during epoch 0, after the epoch-0 snapshot was created at height 1.
	// If they submit *no* report in epoch 0, EndBlock will postpone them for "missing report".
	// Submit host-only reports in the current epoch to keep them ACTIVE for the next epoch.
	ws0 := auditQueryAssignedTargets(t, 0, false, n0.accAddr)
	host := auditHostReportJSON([]string{"PORT_STATE_OPEN"})
	RequireTxSuccess(t, submitEpochReport(t, cli, n0.nodeName, ws0.EpochId, host, nil))
	RequireTxSuccess(t, submitEpochReport(t, cli, n1.nodeName, ws0.EpochId, host, nil))

	// Now wait for the next epoch boundary so the snapshot includes the registered supernodes.
	awaitAtLeastHeight(t, ws0.EpochStartHeight+int64(epochLengthBlocks))
	ws1 := auditQueryAssignedTargets(t, 0, false, n0.accAddr)
	require.Greater(t, len(ws1.TargetSupernodeAccounts), 0, "expected n0 to be assigned targets in epoch %d", ws1.EpochId)

	// Construct a minimal report. Since n0 is ACTIVE, it is a prober and must submit peer observations
	// for all assigned targets for the epoch.
	peerObs := make([]string, 0, len(ws1.TargetSupernodeAccounts))
	for _, target := range ws1.TargetSupernodeAccounts {
		peerObs = append(peerObs, storageChallengeObservationJSON(target, []string{"PORT_STATE_OPEN"}))
	}

	txResp := submitEpochReport(t, cli, n0.nodeName, ws1.EpochId, host, peerObs)
	RequireTxSuccess(t, txResp)

	// Query via gRPC instead of CLI JSON because EpochReport contains float fields and
	// CLI JSON marshalling is currently broken ("unknown type float64") in this environment.
	report := auditQueryReport(t, ws1.EpochId, n0.accAddr)
	require.Equal(t, n0.accAddr, report.SupernodeAccount)
	require.Equal(t, ws1.EpochId, report.EpochId)
}
