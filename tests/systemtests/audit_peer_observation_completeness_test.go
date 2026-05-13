//go:build system_test

package system

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

// This test validates that incomplete peer observations are accepted, persisted, and
// penalize the reporter once without immediate supernode postponement.
func TestAuditSubmitReport_IncompletePeerObservationsAcceptedWithReporterPenalty(t *testing.T) {
	const (
		// Keep epochs long enough in real time to avoid end-blocker enforcement during the test.
		epochLengthBlocks = uint64(20)
	)
	const originHeight = int64(1)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastEpochs(t, epochLengthBlocks, 1, 1, 1, []uint32{4444}),
		func(genesis []byte) []byte {
			// Avoid missing-report postponement before the epoch under test.
			state, err := sjson.SetRawBytes(genesis, "app_state.audit.params.consecutive_epochs_to_postpone", []byte(strconv.FormatUint(2, 10)))
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
	epochID, epochStartHeight := nextEpochAfterHeight(originHeight, epochLengthBlocks, currentHeight)
	awaitAtLeastHeight(t, epochStartHeight)

	host := auditHostReportJSON([]string{"PORT_STATE_OPEN"})
	_, prober, _ := findAssignedProberAndTarget(t, epochID, []testNodeIdentity{n0, n1})
	txResp := submitEpochReport(t, cli, prober.nodeName, epochID, host, nil)
	RequireTxSuccess(t, txResp)

	report := auditQueryReport(t, epochID, prober.accAddr)
	require.Len(t, report.StorageChallengeObservations, 0)

	reliability := auditQueryReporterReliabilityState(t, prober.accAddr)
	require.Equal(t, int64(8), reliability.ReliabilityScore)
	require.Equal(t, epochID, reliability.LastUpdatedEpoch)
	require.Equal(t, "REPORTER_TRUST_BAND_NORMAL", reliability.TrustBand.String())
	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, prober.valAddr))
}
