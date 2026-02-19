//go:build system_test

package system

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

func TestAuditSubmitReport_PostponedReporterSelfOnly(t *testing.T) {
	const (
		// Keep epochs short so the test stays fast.
		epochLengthBlocks = uint64(10)
	)
	const originHeight = int64(1)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastEpochs(t, epochLengthBlocks, 1, 1, 1, []uint32{4444}),
		func(genesis []byte) []byte {
			// Postpone after one missed epoch so we can drive the state transition quickly.
			state, err := sjson.SetRawBytes(genesis, "app_state.audit.params.consecutive_epochs_to_postpone", []byte("1"))
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

	assigned0 := auditQueryAssignedTargets(t, epochID1, true, n0.accAddr)
	host := auditHostReportJSON([]string{"PORT_STATE_OPEN"})
	obs0 := make([]string, 0, len(assigned0.TargetSupernodeAccounts))
	for _, target := range assigned0.TargetSupernodeAccounts {
		obs0 = append(obs0, storageChallengeObservationJSON(target, []string{"PORT_STATE_OPEN"}))
	}

	// Submit only node0 in epoch1 so node1 is postponed due to missing report.
	txResp0 := submitEpochReport(t, cli, n0.nodeName, epochID1, host, obs0)
	RequireTxSuccess(t, txResp0)

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
