//go:build system_test

package system

// This test validates the "empty active set deadlock" bootstrap scenario:
//
// When ALL supernodes are POSTPONED at epoch start, the epoch anchor has an
// empty active_supernode_accounts set. Without active probers, no peer
// observations are generated, and the audit module's peer-port recovery rule
// (compliant host report + peer all-ports-OPEN) cannot be satisfied because
// no probers exist.
//
// To break this bootstrap chicken-and-egg, the audit module applies a
// bootstrap-recovery exception in shouldRecoverAtEpochEnd: when the epoch
// anchor's active set is empty, a compliant self host-report alone is
// sufficient for recovery. Self-compliance is still mandatory; a misbehaving
// SN cannot self-recover via this branch.
//
// With this exception, the chain self-heals from the deadlock once every
// POSTPONED SN submits a compliant host-only report — no operator
// intervention required.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func awaitAtLeastHeightWithSlack(t *testing.T, height int64) {
	t.Helper()
	if sut.currentHeight >= height {
		return
	}
	// This scenario intentionally waits across multiple epochs. On shared CI
	// runners, block production can be slower than the default per-block timeout
	// heuristic in AwaitBlockHeight; use explicit slack to avoid flakiness.
	sut.AwaitBlockHeight(t, height, 45*time.Second)
}

// TestAuditEmptyActiveSetBootstrap_HostOnlyReportsRecover verifies that the
// bootstrap-recovery exception breaks the empty-active-set deadlock: when
// all SNs are POSTPONED and the active set is empty, submitting compliant
// host-only audit reports is sufficient to recover them at epoch end.
//
// This inverts the pre-fix contract (which asserted permanent deadlock).
//
// Scenario:
//  1. Two supernodes register and start ACTIVE.
//  2. Neither submits epoch reports for epoch 0 → both POSTPONED at epoch 0 end.
//  3. Epoch 1: empty active set. Both submit host-only audit reports.
//  4. Epoch 1 end: bootstrap-recovery exception fires → both recover to ACTIVE.
//  5. Epoch 2: both are in the anchor active set → peer observations flow → self-sustaining.
func TestAuditEmptyActiveSetBootstrap_HostOnlyReportsRecover(t *testing.T) {
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

	// Do not assert immediate ACTIVE state here: on slower CI runners we can cross
	// an epoch boundary between registration and this assertion, and missing-report
	// enforcement may already have moved nodes to POSTPONED.

	// ── Epoch 0: Do NOT submit any epoch reports. ──
	// This simulates the testnet scenario where SNs were running releases
	// without audit code when the chain upgraded to enable the audit module.
	currentHeight := sut.AwaitNextBlock(t)
	_, epoch0Start := nextEpochAfterHeight(originHeight, epochLengthBlocks, currentHeight)
	epoch1Start := epoch0Start + int64(epochLengthBlocks)
	epoch2Start := epoch1Start + int64(epochLengthBlocks)

	// Wait for epoch 0 to end → both get POSTPONED for missing reports.
	awaitAtLeastHeightWithSlack(t, epoch1Start)

	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n0.valAddr),
		"node0 should be POSTPONED after missing epoch 0 report")
	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n1.valAddr),
		"node1 should be POSTPONED after missing epoch 0 report")

	// ── Epoch 1: empty active set — bootstrap-recovery exception applies. ──
	epochID1 := uint64((epoch1Start - originHeight) / int64(epochLengthBlocks))

	// Both submit compliant host-only audit epoch reports (as POSTPONED reporters,
	// no observations). With the bootstrap exception, this alone is sufficient
	// for recovery at epoch 1 end.
	hostOK := auditHostReportJSON([]string{"PORT_STATE_OPEN"})
	tx0 := submitEpochReport(t, cli, n0.nodeName, epochID1, hostOK, nil)
	RequireTxSuccess(t, tx0)
	tx1 := submitEpochReport(t, cli, n1.nodeName, epochID1, hostOK, nil)
	RequireTxSuccess(t, tx1)

	// Wait for epoch 1 to end.
	awaitAtLeastHeightWithSlack(t, epoch2Start)

	// Bootstrap-recovery exception: empty active set + compliant self host-report
	// → both SNs recover to ACTIVE.
	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, n0.valAddr),
		"node0 should recover to ACTIVE via the empty-active-set bootstrap exception")
	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, n1.valAddr),
		"node1 should recover to ACTIVE via the empty-active-set bootstrap exception")
}

// TestAuditEmptyActiveSetBootstrap_NonCompliantHostStaysPostponed verifies
// the bootstrap-recovery exception still gates on self-compliance. A
// POSTPONED supernode that submits a host report violating a non-storage
// min-free threshold (CPU here) MUST remain POSTPONED even when the active
// set is empty.
//
// This guards against the exception turning into a "free pass" for
// misbehaving SNs and complements the unit-level tests in
// x/audit/v1/keeper/enforcement_empty_active_set_test.go.
//
// Note: per LEP-6 §17 disk pressure is owned exclusively by the STORAGE_FULL
// transition path (audit SetReport) and is no longer a postpone reason, so
// this test exercises the non-storage CPU path. The disk-pressure bootstrap
// case is covered by TestAuditEmptyActiveSetBootstrap_DiskPressureGoesToStorageFull.
func TestAuditEmptyActiveSetBootstrap_NonCompliantHostStaysPostponed(t *testing.T) {
	const (
		epochLengthBlocks = uint64(10)
		originHeight      = int64(1)
	)

	// Set a non-zero MinCpuFreePercent so non-compliant CPU usage in the host
	// report blocks self-compliance.
	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastEpochsWithMinCpuFree(t, epochLengthBlocks, 1, 1, 1, []uint32{4444}, 20),
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
	epoch2Start := epoch1Start + int64(epochLengthBlocks)

	awaitAtLeastHeightWithSlack(t, epoch1Start)

	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n0.valAddr))
	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n1.valAddr))

	// Epoch 1: empty active set. Both submit host reports with CPU usage 95%
	// (5% free, below the 20% MinCpuFreePercent). Self-compliance fails.
	epochID1 := uint64((epoch1Start - originHeight) / int64(epochLengthBlocks))
	hostNonCompliant := auditHostReportWithCpuUsageJSON([]string{"PORT_STATE_OPEN"}, 95.0)
	RequireTxSuccess(t, submitEpochReport(t, cli, n0.nodeName, epochID1, hostNonCompliant, nil))
	RequireTxSuccess(t, submitEpochReport(t, cli, n1.nodeName, epochID1, hostNonCompliant, nil))

	awaitAtLeastHeightWithSlack(t, epoch2Start)

	// Self-compliance gate blocked the bootstrap exception → still POSTPONED.
	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n0.valAddr),
		"node0 should remain POSTPONED — self-compliance gate blocks the bootstrap exception")
	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n1.valAddr),
		"node1 should remain POSTPONED — self-compliance gate blocks the bootstrap exception")
}

// TestAuditEmptyActiveSetBootstrap_DiskPressureGoesToStorageFull verifies the
// LEP-6 §17 invariant that disk pressure is owned exclusively by the
// STORAGE_FULL transition path, not by audit_host_requirements POSTPONE.
//
// Scenario:
//  1. Two SNs register and miss epoch 0 reports → both POSTPONED for
//     audit_missing_reports.
//  2. Epoch 1: empty active set. Both submit host reports with disk usage
//     above the supernode module's MaxStorageUsagePercent (default 90).
//  3. Epoch 1 end: the bootstrap-recovery exception fires because
//     selfHostCompliant ignores disk (only cpu/mem gate the bootstrap
//     exception). The new recoverSupernodeFromPostponed helper observes
//     disk > MaxStorageUsagePercent in the same epoch's report and steers
//     recovery to STORAGE_FULL instead of ACTIVE.
//
// Invariant locked in: disk pressure never produces ACTIVE in this branch,
// never produces POSTPONED via audit_host_requirements, and produces
// STORAGE_FULL exactly when disk > MaxStorageUsagePercent.
func TestAuditEmptyActiveSetBootstrap_DiskPressureGoesToStorageFull(t *testing.T) {
	const (
		epochLengthBlocks = uint64(10)
		originHeight      = int64(1)
	)

	// No MinCpuFreePercent / MinMemFreePercent override → only disk pressure
	// is in play. The supernode module's default MaxStorageUsagePercent (90)
	// gates the STORAGE_FULL transition; we report 95% to cross it.
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

	// Epoch 0: no reports → both POSTPONED for audit_missing_reports.
	currentHeight := sut.AwaitNextBlock(t)
	_, epoch0Start := nextEpochAfterHeight(originHeight, epochLengthBlocks, currentHeight)
	epoch1Start := epoch0Start + int64(epochLengthBlocks)
	epoch2Start := epoch1Start + int64(epochLengthBlocks)

	awaitAtLeastHeightWithSlack(t, epoch1Start)

	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n0.valAddr))
	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n1.valAddr))

	// Epoch 1: empty active set. Both submit host reports with disk usage
	// 95% (> MaxStorageUsagePercent=90). Self-compliance passes (no
	// cpu/mem floor configured), so the bootstrap exception fires; the
	// recovery helper observes the high disk and steers to STORAGE_FULL.
	epochID1 := uint64((epoch1Start - originHeight) / int64(epochLengthBlocks))
	hostHighDisk := auditHostReportWithDiskUsageJSON([]string{"PORT_STATE_OPEN"}, 95.0)
	RequireTxSuccess(t, submitEpochReport(t, cli, n0.nodeName, epochID1, hostHighDisk, nil))
	RequireTxSuccess(t, submitEpochReport(t, cli, n1.nodeName, epochID1, hostHighDisk, nil))

	awaitAtLeastHeightWithSlack(t, epoch2Start)

	// LEP-6 §17 invariant: disk pressure routes POSTPONED → STORAGE_FULL,
	// never POSTPONED → ACTIVE, never stuck POSTPONED on audit_host_requirements.
	require.Equal(t, "SUPERNODE_STATE_STORAGE_FULL", querySupernodeLatestState(t, cli, n0.valAddr),
		"node0 should transition POSTPONED → STORAGE_FULL via the audit recovery helper (disk > MaxStorageUsagePercent)")
	require.Equal(t, "SUPERNODE_STATE_STORAGE_FULL", querySupernodeLatestState(t, cli, n1.valAddr),
		"node1 should transition POSTPONED → STORAGE_FULL via the audit recovery helper (disk > MaxStorageUsagePercent)")
}
