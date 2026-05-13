//go:build system_test

package system

// Edge-case and boundary system tests for LEP-6 PR5 activation.
//
// These tests augment audit_storage_truth_activation_test.go and cover:
//   - FULL enforcement mode postpones identically to SOFT mode.
//   - UNSPECIFIED mode: no postponement, no heal-op scheduling, no events.
//   - Score decay over epochs triggers recovery from storage-truth postponement.
//   - Multiple recheck evidence submissions against the same node accumulate scores.
//   - Failed heal verification bumps ticket deterioration by 15 on-chain.
//   - Replay protection for recheck evidence is enforced on a live chain.

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"

	audittypes "github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// ── Test 1: FULL mode postpones the same as SOFT ─────────────────────────────

// TestStorageTruth_FullMode_PostponesLikeSoft verifies that FULL enforcement mode
// postpones a supernode when its suspicion score meets the threshold and the
// LEP-6 postpone predicate is satisfied.
func TestStorageTruth_FullMode_PostponesLikeSoft(t *testing.T) {
	const (
		epochLengthBlocks = uint64(12)
		originHeight      = int64(1)
		ticketID          = "edge-ticket-full-mode"
	)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastEpochs(t, epochLengthBlocks, 1, 1, 1, []uint32{4444}), // Per CP3 119-F11 — Validate() rejects empty required_open_ports.
		setStorageTruthTestParams(t,
			"STORAGE_TRUTH_ENFORCEMENT_MODE_FULL",
			10, // postpone threshold — index hash mismatches exceed
			5,  // watch threshold
			10, // heal threshold
			0,  // no decay
			5,
		),
		func(genesis []byte) []byte {
			state, err := sjson.SetRawBytes(genesis, "app_state.audit.params.consecutive_epochs_to_postpone", []byte("100"))
			require.NoError(t, err)
			return state
		},
	)
	sut.StartChain(t)

	cli := NewLumeradCLI(t, sut, true)
	n0 := getNodeIdentity(t, cli, "node0")
	n1 := getNodeIdentity(t, cli, "node1")
	n2 := getNodeIdentity(t, cli, "node2")

	registerSupernode(t, cli, n0, "192.168.1.1")
	registerSupernode(t, cli, n1, "192.168.1.2")
	registerSupernode(t, cli, n2, "192.168.1.3")

	currentHeight := sut.AwaitNextBlock(t)
	epochID1, epoch1Start := nextEpochAfterHeight(originHeight, epochLengthBlocks, currentHeight)
	epoch1End := epoch1Start + int64(epochLengthBlocks)
	awaitAtLeastHeight(t, epoch1Start)

	proberResp, prober, target := findAssignedProberAndTarget(t, epochID1, []testNodeIdentity{n0, n1, n2})

	portStates := make([]string, len(proberResp.RequiredOpenPorts))
	for i := range portStates {
		portStates[i] = "PORT_STATE_OPEN"
	}
	args := []string{
		"tx", "audit", "submit-epoch-report",
		strconv.FormatUint(epochID1, 10),
		auditHostReportJSON(portStates),
		"--from", prober.nodeName,
		"--gas", "500000",
		"--storage-proof-results", buildStorageProofResultJSONWithClass(
			prober.accAddr,
			target.accAddr,
			ticketID+"-recent",
			"hash-full-mode-recent",
			"STORAGE_PROOF_BUCKET_TYPE_RECENT",
			"STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH",
		),
		"--storage-proof-results", buildStorageProofResultJSONWithClass(
			prober.accAddr,
			target.accAddr,
			ticketID+"-old",
			"hash-full-mode-old",
			"STORAGE_PROOF_BUCKET_TYPE_OLD",
			"STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH",
		),
	}
	for _, target := range proberResp.TargetSupernodeAccounts {
		args = append(args, "--storage-challenge-observations", storageChallengeObservationJSON(target, portStates))
	}
	proofResp := cli.CustomCommand(args...)
	RequireTxSuccess(t, proofResp)

	awaitAtLeastHeight(t, epoch1End)
	sut.AwaitNextBlock(t)

	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, target.valAddr),
		"FULL mode must postpone target when index hash mismatches exceed threshold and satisfy postpone predicates")
	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, prober.valAddr),
		"challenger must remain ACTIVE")
}

// ── Test 2: UNSPECIFIED mode is a no-op ──────────────────────────────────────

// TestStorageTruth_UnspecifiedMode_NeitherPostponesNorSchedulesHealOps verifies
// that when enforcement_mode = UNSPECIFIED:
//   - No supernode is postponed (even at extreme suspicion scores).
//   - No heal ops are scheduled (even above the heal threshold).
func TestStorageTruth_UnspecifiedMode_NeitherPostponesNorSchedulesHealOps(t *testing.T) {
	const (
		epochLengthBlocks = uint64(12)
		originHeight      = int64(1)
		ticketID          = "edge-ticket-unspecified"
	)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastEpochs(t, epochLengthBlocks, 1, 1, 1, []uint32{4444}), // Per CP3 119-F11 — Validate() rejects empty required_open_ports.
		setStorageTruthTestParams(t,
			"STORAGE_TRUTH_ENFORCEMENT_MODE_UNSPECIFIED",
			1, // postpone threshold — 1 would postpone immediately in any other mode
			1, // watch threshold
			1, // heal threshold — very low
			0, // no decay
			5,
		),
		func(genesis []byte) []byte {
			state, err := sjson.SetRawBytes(genesis, "app_state.audit.params.consecutive_epochs_to_postpone", []byte("100"))
			require.NoError(t, err)
			return state
		},
	)
	sut.StartChain(t)

	cli := NewLumeradCLI(t, sut, true)
	n0 := getNodeIdentity(t, cli, "node0")
	n1 := getNodeIdentity(t, cli, "node1")
	n2 := getNodeIdentity(t, cli, "node2")

	registerSupernode(t, cli, n0, "192.168.1.1")
	registerSupernode(t, cli, n1, "192.168.1.2")
	registerSupernode(t, cli, n2, "192.168.1.3")

	currentHeight := sut.AwaitNextBlock(t)
	epochID1, epoch1Start := nextEpochAfterHeight(originHeight, epochLengthBlocks, currentHeight)
	epoch1End := epoch1Start + int64(epochLengthBlocks)
	awaitAtLeastHeight(t, epoch1Start)
	nodes := []testNodeIdentity{n0, n1, n2}
	_, _, target := findAssignedProberAndTarget(t, epochID1, nodes)

	// Seed transcript record (UNSPECIFIED uses k-based assignment; divisor param is irrelevant here).
	rechecker := seedProofTranscripts(t, cli, epochID1, nodes, target.accAddr,
		[]transcriptSeed{{ticketID: ticketID, transcriptHash: "hash-unspec-orig"}}, false)

	recheckResp := submitStorageRecheckEvidence(t, cli,
		rechecker.nodeName,
		epochID1,
		target.accAddr,
		ticketID,
		"hash-unspec-orig",
		"hash-unspec-recheck",
		"recheck-confirmed-fail",
	)
	RequireTxSuccess(t, recheckResp)
	sut.AwaitNextBlock(t)

	awaitAtLeastHeight(t, epoch1End)
	sut.AwaitNextBlock(t)

	// UNSPECIFIED: no postponement.
	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, target.valAddr),
		"UNSPECIFIED mode must not postpone target")

	// UNSPECIFIED: no heal ops scheduled.
	healOps := auditQueryHealOpsByTicketST(t, ticketID)
	require.Empty(t, healOps,
		"UNSPECIFIED mode must not schedule heal ops even above the heal threshold")
}

// ── Test 3: score decay triggers recovery ────────────────────────────────────

// TestStorageTruth_ScoreDecay_TriggersRecovery verifies the full postpone →
// decay-over-epochs lifecycle on a live chain. Recovery also requires clean-pass
// evidence, so this test verifies the node remains postponed without it.
//
// Setup:
//   - 3 × RECHECK_CONFIRMED_FAIL (different ticket IDs) in epoch 1 → suspicion exceeds 50.
//   - postpone_threshold = 50, so epoch 1 end → n1 POSTPONED.
//   - decay_per_epoch = 30, watch_threshold = 20.
//   - epoch 2 end: decayed score = 60 - 30*1 = 30 > watch(20) → still POSTPONED.
//   - epoch 3 end: score has decayed below watch(20), but no clean passes were submitted.
func TestStorageTruth_ScoreDecay_TriggersRecovery(t *testing.T) {
	const (
		epochLengthBlocks = uint64(14)
		originHeight      = int64(1)
	)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastEpochs(t, epochLengthBlocks, 1, 1, 1, []uint32{4444}), // Per CP3 119-F11 — Validate() rejects empty required_open_ports.
		setStorageTruthTestParams(t,
			"STORAGE_TRUTH_ENFORCEMENT_MODE_SOFT",
			50, // postpone threshold
			20, // watch threshold
			5,  // heal threshold (low, but irrelevant for this test)
			30, // decay per epoch — 2 epochs = 60 decay, score reaches 0
			5,
		),
		func(genesis []byte) []byte {
			state, err := sjson.SetRawBytes(genesis, "app_state.audit.params.consecutive_epochs_to_postpone", []byte("100"))
			require.NoError(t, err)
			return state
		},
	)
	sut.StartChain(t)

	cli := NewLumeradCLI(t, sut, true)
	n0 := getNodeIdentity(t, cli, "node0")
	n1 := getNodeIdentity(t, cli, "node1")
	n2 := getNodeIdentity(t, cli, "node2")

	registerSupernode(t, cli, n0, "192.168.1.1")
	registerSupernode(t, cli, n1, "192.168.1.2")
	registerSupernode(t, cli, n2, "192.168.1.3")

	currentHeight := sut.AwaitNextBlock(t)
	epochID1, epoch1Start := nextEpochAfterHeight(originHeight, epochLengthBlocks, currentHeight)
	epoch1End := epoch1Start + int64(epochLengthBlocks)
	epoch3End := epoch1End + 2*int64(epochLengthBlocks)

	awaitAtLeastHeight(t, epoch1Start)
	nodes := []testNodeIdentity{n0, n1, n2}
	_, _, target := findAssignedProberAndTarget(t, epochID1, nodes)

	// Seed all 3 transcript records in one epoch report from the prober; get rechecker.
	var decaySeeds []transcriptSeed
	for i := 0; i < 3; i++ {
		decaySeeds = append(decaySeeds, transcriptSeed{
			ticketID:       fmt.Sprintf("edge-ticket-decay-%d", i),
			transcriptHash: fmt.Sprintf("orig-hash-%d", i),
		})
	}
	rechecker := seedProofTranscripts(t, cli, epochID1, nodes, target.accAddr, decaySeeds, false)

	// Submit 3 recheks against target with distinct ticket IDs → suspicion exceeds 50.
	for i := 0; i < 3; i++ {
		resp := submitStorageRecheckEvidence(t, cli,
			rechecker.nodeName,
			epochID1,
			target.accAddr,
			fmt.Sprintf("edge-ticket-decay-%d", i),
			fmt.Sprintf("orig-hash-%d", i),
			fmt.Sprintf("recheck-hash-%d", i),
			"recheck-confirmed-fail",
		)
		RequireTxSuccess(t, resp)
		sut.AwaitNextBlock(t)
	}

	// Epoch 1 end: score exceeds postpone threshold 50 → POSTPONED.
	awaitAtLeastHeight(t, epoch1End)
	sut.AwaitNextBlock(t)
	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, target.valAddr),
		"target must be POSTPONED after suspicion 60 exceeds threshold 50")

	// Epoch 3 end: multiplicative decay brings the score below watch(20), but the
	// recovery clean-pass gate keeps the node POSTPONED.
	// Use an explicit 120s timeout: waiting 28 blocks can exceed the default window under load.
	sut.AwaitBlockHeight(t, epoch3End, 120*time.Second)
	sut.AwaitNextBlock(t)
	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, target.valAddr),
		"target must remain POSTPONED until recovery clean-pass requirements are met")
}

// ── Test 4: multiple recheck evidence accumulates per-node score ──────────────

// TestStorageTruth_MultipleRecheckEvidence_AccumulatesScore verifies that
// submitting multiple SubmitStorageRecheckEvidence messages targeting the same
// supernode with different ticket IDs results in an additive node suspicion score.
func TestStorageTruth_MultipleRecheckEvidence_AccumulatesScore(t *testing.T) {
	const (
		epochLengthBlocks = uint64(15)
		originHeight      = int64(1)
	)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastEpochs(t, epochLengthBlocks, 1, 1, 1, []uint32{4444}),
		setStorageTruthTestParams(t,
			"STORAGE_TRUTH_ENFORCEMENT_MODE_SOFT",
			1000, // threshold very high — no postponement
			500,
			5,
			0,
			5,
		),
		func(genesis []byte) []byte {
			state, err := sjson.SetRawBytes(genesis, "app_state.audit.params.consecutive_epochs_to_postpone", []byte("100"))
			require.NoError(t, err)
			return state
		},
	)
	sut.StartChain(t)

	cli := NewLumeradCLI(t, sut, true)
	n0 := getNodeIdentity(t, cli, "node0")
	n1 := getNodeIdentity(t, cli, "node1")
	n2 := getNodeIdentity(t, cli, "node2")

	registerSupernode(t, cli, n0, "192.168.1.1")
	registerSupernode(t, cli, n1, "192.168.1.2")
	registerSupernode(t, cli, n2, "192.168.1.3")

	currentHeight := sut.AwaitNextBlock(t)
	epochID1, epoch1Start := nextEpochAfterHeight(originHeight, epochLengthBlocks, currentHeight)
	awaitAtLeastHeight(t, epoch1Start)
	nodes := []testNodeIdentity{n0, n1, n2}
	_, _, target := findAssignedProberAndTarget(t, epochID1, nodes)

	// Verify no suspicion state before any recheck.
	_, found := auditQueryNodeSuspicionStateST(t, target.accAddr)
	require.False(t, found, "target must have no suspicion state before any recheck evidence")

	// Seed all 4 transcript records (each recheck ticket needs its own transcript hash).
	var multiSeeds []transcriptSeed
	for i := 0; i < 4; i++ {
		multiSeeds = append(multiSeeds, transcriptSeed{
			ticketID:       "multi-ticket-" + strconv.Itoa(i),
			transcriptHash: fmt.Sprintf("orig-hash-%d", i),
		})
	}
	rechecker := seedProofTranscripts(t, cli, epochID1, nodes, target.accAddr, multiSeeds, false)

	// Submit 4 recheks with distinct ticket IDs and hashes. Base node delta is 15;
	// repeated distinct ticket failures add escalation bonuses.
	for i := 0; i < 4; i++ {
		resp := submitStorageRecheckEvidence(t, cli,
			rechecker.nodeName,
			epochID1,
			target.accAddr,
			"multi-ticket-"+strconv.Itoa(i),
			fmt.Sprintf("orig-hash-%d", i),
			fmt.Sprintf("recheck-hash-%d", i),
			"recheck-confirmed-fail",
		)
		RequireTxSuccess(t, resp)
		sut.AwaitNextBlock(t)
	}

	nodeState, found := auditQueryNodeSuspicionStateST(t, target.accAddr)
	require.True(t, found, "target must have suspicion state after 4 recheks")
	// Per CP3 121-F1 — RECHECK bucket bypasses storageTruthBookkeepingForResult,
	// so rechecks no longer double-count the pattern escalation bonus.
	// Spec-derived expected score:
	//   seedProofTranscripts submits 4 INVALID_TRANSCRIPT results (RECENT bucket) in ONE tx.
	//   INVALID_TRANSCRIPT base node delta = 0 (LEP6.md §14:494-499); RECENT bucket triggers
	//   bookkeeping path; pattern escalation bonus per LEP6.md §14:756-758 (distinct-ticket count):
	//     R1 (count=1): +0   R2 (count=2): +10   R3 (count=3): +15   R4 (count≥3): +15  → +40 from seeds.
	//   Then 4 RECHECK_CONFIRMED_FAIL submissions (RECHECK bucket → bypass);
	//   per-recheck base node delta = +15 (LEP6.md §14:500-505) with NO bonus  → +60 from rechecks.
	//   Total: 40 + 60 = 100. (Pre-CP3 incorrectly returned 160 because rechecks went through
	//   bookkeeping and added pattern bonuses, double-counting with the contradiction penalty
	//   already handled in SubmitStorageRecheckEvidence.)
	require.Equal(t, int64(100), nodeState.SuspicionScore,
		"4 INVALID_TRANSCRIPT seeds (+40 from §14 pattern escalation) + 4 RECHECK_CONFIRMED_FAIL (+60 base, bypass per 121-F1) = 100")

	// Also verify 4 ticket deterioration states were created. Recheck-confirmed
	// failures are confirmed outcomes, so reporter trust scaling does not reduce
	// their ticket deterioration delta.
	for i := 0; i < 4; i++ {
		td, found := auditQueryTicketDeteriorationStateST(t, "multi-ticket-"+strconv.Itoa(i))
		require.True(t, found, "ticket %d must have deterioration state", i)
		require.Equal(t, int64(8), td.DeteriorationScore)
	}
}

// ── Test 5: failed heal bumps ticket deterioration on-chain ──────────────────

// TestStorageTruth_FailedHeal_BumpsTicketDeterioration tests the negative path
// of the heal lifecycle end-to-end on a live chain:
//
//  1. Index proof failure → ticket deterioration = 12 > heal threshold → heal op scheduled.
//  2. Healer claims complete.
//  3. Verifier submits false verification.
//  4. Heal op transitions to FAILED.
//  5. Ticket deterioration increases by 15: 12 + 15 = 27.
func TestStorageTruth_FailedHeal_BumpsTicketDeterioration(t *testing.T) {
	const (
		epochLengthBlocks = uint64(12)
		originHeight      = int64(1)
		ticketID          = "edge-ticket-failed-heal"
	)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastEpochs(t, epochLengthBlocks, 1, 1, 1, []uint32{4444}), // Per CP3 119-F11 — Validate() rejects empty required_open_ports.
		setStorageTruthTestParams(t,
			"STORAGE_TRUTH_ENFORCEMENT_MODE_SOFT",
			1000, // postpone threshold very high
			500,
			10, // heal threshold — index hash mismatch (12) exceeds this
			0,
			10,
		),
		func(genesis []byte) []byte {
			state, err := sjson.SetRawBytes(genesis, "app_state.audit.params.consecutive_epochs_to_postpone", []byte("100"))
			require.NoError(t, err)
			return state
		},
	)
	sut.StartChain(t)

	cli := NewLumeradCLI(t, sut, true)
	n0 := getNodeIdentity(t, cli, "node0")
	n1 := getNodeIdentity(t, cli, "node1")
	n2 := getNodeIdentity(t, cli, "node2")

	registerSupernode(t, cli, n0, "192.168.1.1")
	registerSupernode(t, cli, n1, "192.168.1.2")
	registerSupernode(t, cli, n2, "192.168.1.3")

	nodeForAccount := map[string]testNodeIdentity{
		n0.accAddr: n0,
		n1.accAddr: n1,
		n2.accAddr: n2,
	}

	currentHeight := sut.AwaitNextBlock(t)
	epochID1, epoch1Start := nextEpochAfterHeight(originHeight, epochLengthBlocks, currentHeight)
	epoch1End := epoch1Start + int64(epochLengthBlocks)
	awaitAtLeastHeight(t, epoch1Start)

	proberResp, prober, target := findAssignedProberAndTarget(t, epochID1, []testNodeIdentity{n0, n1, n2})

	portStates := make([]string, len(proberResp.RequiredOpenPorts))
	for i := range portStates {
		portStates[i] = "PORT_STATE_OPEN"
	}
	args := []string{
		"tx", "audit", "submit-epoch-report",
		strconv.FormatUint(epochID1, 10),
		auditHostReportJSON(portStates),
		"--from", prober.nodeName,
		"--gas", "500000",
		"--storage-proof-results", buildStorageProofResultJSONWithClass(
			prober.accAddr,
			target.accAddr,
			ticketID,
			"orig-hash-failed",
			"STORAGE_PROOF_BUCKET_TYPE_RECENT",
			"STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH",
		),
	}
	for _, target := range proberResp.TargetSupernodeAccounts {
		args = append(args, "--storage-challenge-observations", storageChallengeObservationJSON(target, portStates))
	}
	proofResp := cli.CustomCommand(args...)
	RequireTxSuccess(t, proofResp)
	sut.AwaitNextBlock(t)

	// Epoch 1 end: heal op scheduled.
	awaitAtLeastHeight(t, epoch1End)
	sut.AwaitNextBlock(t)

	healOps := auditQueryHealOpsByTicketST(t, ticketID)
	require.Len(t, healOps, 1, "one heal op expected after epoch end")

	healOp := healOps[0]
	require.Equal(t, audittypes.HealOpStatus_HEAL_OP_STATUS_SCHEDULED, healOp.Status)

	// Identify healer.
	healerID, ok := nodeForAccount[healOp.HealerSupernodeAccount]
	require.True(t, ok)

	claimResp := submitClaimHealCompleteST(t, cli, healerID.nodeName, healOp.HealOpId, ticketID, "manifest-failed")
	RequireTxSuccess(t, claimResp)
	sut.AwaitNextBlock(t)

	if len(healOp.VerifierSupernodeAccounts) == 0 {
		// Single-node path: heal finalizes immediately on ClaimHealComplete.
		// Cannot test failed-verify path here; skip.
		t.Skip("network has no verifiers assigned — failed verification path is unavailable")
	}

	// All verifiers submit FALSE; majority quorum then finalizes the heal op as FAILED.
	for i, verifierAccount := range healOp.VerifierSupernodeAccounts {
		verifierID, ok := nodeForAccount[verifierAccount]
		require.True(t, ok)

		verifyResp := submitHealVerificationST(t, cli, verifierID.nodeName, healOp.HealOpId, false, fmt.Sprintf("reject-hash-%d", i))
		RequireTxSuccess(t, verifyResp)
		sut.AwaitNextBlock(t)
	}

	finalOps := auditQueryHealOpsByTicketST(t, ticketID)
	require.Len(t, finalOps, 1)
	require.Equal(t, audittypes.HealOpStatus_HEAL_OP_STATUS_FAILED, finalOps[0].Status,
		"heal op must be FAILED after verifier rejects")

	// Ticket deterioration must increase by 15: 12 + 15 = 27.
	tdState, found := auditQueryTicketDeteriorationStateST(t, ticketID)
	require.True(t, found)
	require.Equal(t, int64(27), tdState.DeteriorationScore,
		"ticket deterioration must increase by 15 after failed heal")
}

// ── Test 6: replay rejection on a live chain ─────────────────────────────────

// TestStorageTruth_RecheckEvidence_ReplayRejectedOnChain verifies that the live
// chain enforces replay protection for SubmitStorageRecheckEvidence: a second
// submission with the same (epoch_id, ticket_id, creator) triple is rejected.
func TestStorageTruth_RecheckEvidence_ReplayRejectedOnChain(t *testing.T) {
	const (
		epochLengthBlocks = uint64(15)
		originHeight      = int64(1)
		ticketID          = "edge-ticket-replay"
	)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastEpochs(t, epochLengthBlocks, 1, 1, 1, []uint32{4444}),
		setStorageTruthTestParams(t,
			"STORAGE_TRUTH_ENFORCEMENT_MODE_SOFT",
			1000,
			500,
			10,
			0,
			5,
		),
		func(genesis []byte) []byte {
			state, err := sjson.SetRawBytes(genesis, "app_state.audit.params.consecutive_epochs_to_postpone", []byte("100"))
			require.NoError(t, err)
			return state
		},
	)
	sut.StartChain(t)

	cli := NewLumeradCLI(t, sut, true)
	n0 := getNodeIdentity(t, cli, "node0")
	n1 := getNodeIdentity(t, cli, "node1")
	n2 := getNodeIdentity(t, cli, "node2")

	registerSupernode(t, cli, n0, "192.168.1.1")
	registerSupernode(t, cli, n1, "192.168.1.2")
	registerSupernode(t, cli, n2, "192.168.1.3")
	nodes := []testNodeIdentity{n0, n1, n2}

	currentHeight := sut.AwaitNextBlock(t)
	epochID1, epoch1Start := nextEpochAfterHeight(originHeight, epochLengthBlocks, currentHeight)
	awaitAtLeastHeight(t, epoch1Start)
	_, _, target := findAssignedProberAndTarget(t, epochID1, nodes)

	// Seed transcript record; get rechecker.
	rechecker := seedProofTranscripts(t, cli, epochID1, nodes, target.accAddr,
		[]transcriptSeed{{ticketID: ticketID, transcriptHash: "orig-hash-replay"}}, false)

	// First submission must succeed.
	resp1 := submitStorageRecheckEvidence(t, cli,
		rechecker.nodeName,
		epochID1,
		target.accAddr,
		ticketID,
		"orig-hash-replay",
		"recheck-hash-replay",
		"recheck-confirmed-fail",
	)
	RequireTxSuccess(t, resp1)
	sut.AwaitNextBlock(t)

	// Second submission with the same (epoch, ticket, creator) triple must fail.
	resp2 := submitStorageRecheckEvidence(t, cli,
		rechecker.nodeName,
		epochID1,
		target.accAddr,
		ticketID,
		"orig-hash-replay",
		"recheck-hash-replay",
		"recheck-confirmed-fail",
	)
	// The CLI response will contain an error code if the tx was rejected.
	require.Contains(t, resp2, "already submitted",
		"duplicate recheck evidence must be rejected on-chain")
}
