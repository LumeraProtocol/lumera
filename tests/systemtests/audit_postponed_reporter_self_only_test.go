//go:build system_test

package system

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

func TestAuditSubmitReport_PostponedReporterSelfOnly(t *testing.T) {
	const (
		// Keep epochs long enough in real time to avoid missing-report postponement before the first tested epoch.
		epochLengthBlocks = uint64(20)
	)
	const originHeight = int64(1)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastEpochs(t, epochLengthBlocks, 1, 1, 1, []uint32{4444}),
		func(genesis []byte) []byte {
			// Avoid missing-report postponement before the first tested epoch.
			state, err := sjson.SetRawBytes(genesis, "app_state.audit.params.consecutive_epochs_to_postpone", []byte("2"))
			require.NoError(t, err)

			// Make it easy to postpone node1 via host requirements (independent of consecutive epochs).
			state, err = sjson.SetRawBytes(state, "app_state.audit.params.min_cpu_free_percent", []byte("90"))
			require.NoError(t, err)
			return state
		},
	)
	sut.StartChain(t)

	cli := NewLumeradCLI(t, sut, true)
	n0 := getNodeIdentity(t, cli, "node0")
	n1 := getNodeIdentity(t, cli, "node1")

	registerSupernode(t, cli, n0, "192.168.1.1")
	registerSupernode(t, cli, n1, "192.168.1.2")

	currentHeight := sut.AwaitNextBlock(t)
	epochID1, epoch1Start := nextEpochAfterHeight(originHeight, epochLengthBlocks, currentHeight)
	epochID2 := epochID1 + 1
	epoch2Start := epoch1Start + int64(epochLengthBlocks)

	awaitAtLeastHeight(t, epoch1Start)

	seed1 := headerHashAtHeight(t, sut.rpcAddr, epoch1Start)
	senders := sortedStrings(n0.accAddr, n1.accAddr)
	receivers := sortedStrings(n0.accAddr, n1.accAddr)
	kEpoch := computeKEpoch(1, 1, 1, len(senders), len(receivers))
	require.Equal(t, uint32(1), kEpoch)

	targets0, ok := assignedTargets(seed1, senders, receivers, kEpoch, n0.accAddr)
	require.True(t, ok)
	require.Len(t, targets0, 1)

	host := auditHostReportJSON([]string{"PORT_STATE_OPEN"})

	node1BadSelfBz, err := json.Marshal(map[string]any{
		"cpu_usage_percent":    99.0,
		"mem_usage_percent":    1.0,
		"disk_usage_percent":   1.0,
		"inbound_port_states":  []string{"PORT_STATE_OPEN"},
		"failed_actions_count": 0,
	})
	require.NoError(t, err)
	node1BadSelf := string(node1BadSelfBz)

	// Both submit in epoch1 so missing-report enforcement doesn't interfere.
	// node1 violates host minimums and should become POSTPONED at epoch end.
	txResp0 := submitEpochReport(t, cli, n0.nodeName, epochID1, host, []string{
		storageChallengeObservationJSON(targets0[0], []string{"PORT_STATE_OPEN"}),
	})
	RequireTxSuccess(t, txResp0)

	txResp1 := submitEpochReport(t, cli, n1.nodeName, epochID1, node1BadSelf, []string{
		storageChallengeObservationJSON(n0.accAddr, []string{"PORT_STATE_OPEN"}),
	})
	RequireTxSuccess(t, txResp1)

	awaitAtLeastHeight(t, epoch2Start)
	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n1.valAddr))

	// POSTPONED reporter cannot submit storage challenge observations.
	txBad := submitEpochReport(t, cli, n1.nodeName, epochID2, host, []string{
		storageChallengeObservationJSON(n0.accAddr, []string{"PORT_STATE_OPEN"}),
	})
	RequireTxFailure(t, txBad, "reporter not eligible for storage challenge observations")

	// POSTPONED reporter can submit a host report only.
	txOK := submitEpochReport(t, cli, n1.nodeName, epochID2, host, nil)
	RequireTxSuccess(t, txOK)
}
