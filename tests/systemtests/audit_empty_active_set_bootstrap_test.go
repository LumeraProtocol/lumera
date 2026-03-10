//go:build system_test

package system

// This test validates the "empty active set deadlock" bootstrap scenario:
//
// When ALL supernodes are POSTPONED at epoch start, the epoch anchor has an
// empty active_supernode_accounts set. Without active probers, no peer
// observations are generated, and the audit module's recovery rule
// (compliant host report + peer all-ports-OPEN) can never be satisfied.
//
// The fix is to use legacy MsgReportSupernodeMetrics to recover SNs to
// ACTIVE mid-epoch. Combined with audit epoch reports, the SN survives
// the audit EndBlocker and appears in the next epoch's anchor, seeding
// the active set and bootstrapping the peer-observation cycle.
//
// Scenario:
//   1. Two supernodes register and start ACTIVE.
//   2. Neither submits epoch reports for epoch 0 → both POSTPONED at epoch 0 end.
//   3. Epoch 1: empty active set. Both submit host-only audit reports.
//      Verify: audit recovery alone cannot recover them (no peer observations).
//   4. Legacy MsgReportSupernodeMetrics recovers both mid-epoch 2.
//   5. Epoch 2 end: audit enforcement checks them as ACTIVE — they have reports,
//      host minimums disabled, no peer-port streak → they stay ACTIVE.
//   6. Epoch 3: both are in the anchor active set → peer observations flow → self-sustaining.

import (
	"testing"

	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	"github.com/stretchr/testify/require"
)

func TestAuditEmptyActiveSetBootstrap_LegacyMetricsBreaksDeadlock(t *testing.T) {
	const (
		epochLengthBlocks = uint64(10)
		originHeight      = int64(1)
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

	// Both are ACTIVE after registration.
	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, n0.valAddr))
	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, n1.valAddr))

	// ── Epoch 0: Do NOT submit any epoch reports. ──
	// This simulates the testnet scenario where SNs were running releases
	// without audit code when the chain upgraded to enable the audit module.
	currentHeight := sut.AwaitNextBlock(t)
	_, epoch0Start := nextEpochAfterHeight(originHeight, epochLengthBlocks, currentHeight)
	epoch1Start := epoch0Start + int64(epochLengthBlocks)
	epoch2Start := epoch1Start + int64(epochLengthBlocks)

	// Wait for epoch 0 to end → both get POSTPONED for missing reports.
	awaitAtLeastHeight(t, epoch1Start)

	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n0.valAddr),
		"node0 should be POSTPONED after missing epoch 0 report")
	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n1.valAddr),
		"node1 should be POSTPONED after missing epoch 0 report")

	// ── Epoch 1: Empty active set — the deadlock. ──
	epochID1 := uint64((epoch1Start - originHeight) / int64(epochLengthBlocks))

	// Both submit host-only audit epoch reports (as POSTPONED reporters, no observations).
	hostOK := auditHostReportJSON([]string{"PORT_STATE_OPEN"})
	tx0 := submitEpochReport(t, cli, n0.nodeName, epochID1, hostOK, nil)
	RequireTxSuccess(t, tx0)
	tx1 := submitEpochReport(t, cli, n1.nodeName, epochID1, hostOK, nil)
	RequireTxSuccess(t, tx1)

	// Wait for epoch 1 to end WITHOUT legacy metrics recovery.
	// Both should remain POSTPONED — audit recovery fails (no peer observations).
	awaitAtLeastHeight(t, epoch2Start)

	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n0.valAddr),
		"node0 should still be POSTPONED — audit recovery alone cannot break the deadlock")
	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n1.valAddr),
		"node1 should still be POSTPONED — audit recovery alone cannot break the deadlock")

	// ── Epoch 2: Break the deadlock with legacy MsgReportSupernodeMetrics. ──
	epochID2 := epochID1 + 1
	epoch3Start := epoch2Start + int64(epochLengthBlocks)

	// Submit legacy metrics → instant recovery to ACTIVE.
	compliantMetrics := sntypes.SupernodeMetrics{
		VersionMajor: 2,
		VersionMinor: 4,
		VersionPatch: 5,
		OpenPorts: []sntypes.PortStatus{
			{Port: 4444, State: sntypes.PortState_PORT_STATE_OPEN},
		},
	}

	hash0 := reportSupernodeMetrics(t, cli, n0.nodeName, n0.valAddr, n0.accAddr, compliantMetrics)
	txJSON0 := waitForTx(t, cli, hash0)
	resp0 := decodeTxResponse(t, txJSON0)
	require.Equal(t, uint32(0), resp0.Code, "legacy metrics tx for node0 should succeed: %s", resp0.RawLog)

	hash1 := reportSupernodeMetrics(t, cli, n1.nodeName, n1.valAddr, n1.accAddr, compliantMetrics)
	txJSON1 := waitForTx(t, cli, hash1)
	resp1 := decodeTxResponse(t, txJSON1)
	require.Equal(t, uint32(0), resp1.Code, "legacy metrics tx for node1 should succeed: %s", resp1.RawLog)

	// Both should now be ACTIVE (instant recovery via legacy path).
	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, n0.valAddr),
		"node0 should be ACTIVE after legacy metrics recovery")
	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, n1.valAddr),
		"node1 should be ACTIVE after legacy metrics recovery")

	// Also submit audit epoch reports so the audit EndBlocker doesn't re-postpone them.
	tx0e2 := submitEpochReport(t, cli, n0.nodeName, epochID2, hostOK, nil)
	RequireTxSuccess(t, tx0e2)
	tx1e2 := submitEpochReport(t, cli, n1.nodeName, epochID2, hostOK, nil)
	RequireTxSuccess(t, tx1e2)

	// Wait for epoch 2 to end.
	awaitAtLeastHeight(t, epoch3Start)

	// ── Verify: both survive the audit EndBlocker and remain ACTIVE. ──
	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, n0.valAddr),
		"node0 should remain ACTIVE after epoch 2 enforcement (legacy metrics + audit report)")
	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, n1.valAddr),
		"node1 should remain ACTIVE after epoch 2 enforcement (legacy metrics + audit report)")
}

// TestAuditEmptyActiveSetDeadlock_HostOnlyReportsCannotRecover verifies that
// when all supernodes are POSTPONED, submitting host-only epoch reports across
// multiple epochs is insufficient for recovery — proving the deadlock exists.
func TestAuditEmptyActiveSetDeadlock_HostOnlyReportsCannotRecover(t *testing.T) {
	const (
		epochLengthBlocks = uint64(10)
		originHeight      = int64(1)
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

	// Epoch 0: no reports → both POSTPONED.
	currentHeight := sut.AwaitNextBlock(t)
	_, epoch0Start := nextEpochAfterHeight(originHeight, epochLengthBlocks, currentHeight)
	epoch1Start := epoch0Start + int64(epochLengthBlocks)

	awaitAtLeastHeight(t, epoch1Start)

	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n0.valAddr))
	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n1.valAddr))

	// Submit host-only reports for 3 consecutive epochs. None should recover.
	hostOK := auditHostReportJSON([]string{"PORT_STATE_OPEN"})
	for i := 0; i < 3; i++ {
		epochStart := epoch1Start + int64(i)*int64(epochLengthBlocks)
		nextEpochStart := epochStart + int64(epochLengthBlocks)
		epochID := uint64((epochStart - originHeight) / int64(epochLengthBlocks))

		awaitAtLeastHeight(t, epochStart)

		tx0 := submitEpochReport(t, cli, n0.nodeName, epochID, hostOK, nil)
		RequireTxSuccess(t, tx0)
		tx1 := submitEpochReport(t, cli, n1.nodeName, epochID, hostOK, nil)
		RequireTxSuccess(t, tx1)

		awaitAtLeastHeight(t, nextEpochStart)

		require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n0.valAddr),
			"node0 should remain POSTPONED in epoch %d — no peer observations possible", epochID)
		require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n1.valAddr),
			"node1 should remain POSTPONED in epoch %d — no peer observations possible", epochID)
	}
}
