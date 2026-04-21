//go:build system_test

package system

// System tests for LEP-6 PR5 (Activation) — Storage-Truth Enforcement and Ticket-Driven Self-Healing.
//
// Test coverage:
//   - Recheck evidence submission updates node suspicion and ticket deterioration scores.
//   - SOFT mode postpones a node when its suspicion score meets the postpone threshold.
//   - SHADOW mode emits events but does NOT postpone even when score meets threshold.
//   - Heal ops are scheduled at epoch end when ticket deterioration meets the heal threshold.
//   - Full heal-op lifecycle: schedule → ClaimHealComplete → SubmitHealVerification → VERIFIED.
//   - Verified heal resets ticket deterioration score to max(8, floor(D_old * 0.25)).
//
// Design notes:
//   - SubmitStorageRecheckEvidence with RECHECK_CONFIRMED_FAIL adds +20 to both node suspicion
//     and ticket deterioration (with NORMAL reporter trust band, 100% multiplier).
//   - Enforcement thresholds are set very low in genesis so a single recheck submission
//     triggers observable state transitions without multiple epochs.
//   - consecutive_epochs_to_postpone=100 disables missing-report postponement so it never
//     interferes with the storage-truth enforcement assertions.
//   - All gRPC queries go to node0 at localhost:9090.

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	audittypes "github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// ── genesis mutators ─────────────────────────────────────────────────────────

// setStorageTruthTestParams returns a genesis mutator that overrides storage-truth params
// to enable enforcement at low thresholds so single-recheck submissions are observable.
//
//   - mode: proto enum name (e.g. "STORAGE_TRUTH_ENFORCEMENT_MODE_SOFT")
//   - postponeThreshold: suspicion score at which the node is postponed (SOFT/FULL only)
//   - watchThreshold: suspicion score at which Watch band begins
//   - healThreshold: ticket deterioration score at which heal ops are scheduled
//   - decayPerEpoch: score decay applied each epoch (0 disables decay)
//   - maxHealOps: maximum self-heal ops scheduled per epoch
func setStorageTruthTestParams(
	t *testing.T,
	mode string,
	postponeThreshold, watchThreshold, healThreshold, decayPerEpoch int64,
	maxHealOps uint32,
) GenesisMutator {
	return func(genesis []byte) []byte {
		t.Helper()
		state := genesis
		var err error

		// Enum: proto3 JSON string.
		state, err = sjson.SetRawBytes(state,
			"app_state.audit.params.storage_truth_enforcement_mode",
			[]byte(fmt.Sprintf("%q", mode)))
		require.NoError(t, err)

		// int64 thresholds: proto3 JSON represents int64 as strings.
		state, err = sjson.SetRawBytes(state,
			"app_state.audit.params.storage_truth_node_suspicion_threshold_postpone",
			[]byte(fmt.Sprintf("%q", strconv.FormatInt(postponeThreshold, 10))))
		require.NoError(t, err)

		// Set probation midway between watch and postpone.
		probation := (watchThreshold + postponeThreshold) / 2
		state, err = sjson.SetRawBytes(state,
			"app_state.audit.params.storage_truth_node_suspicion_threshold_probation",
			[]byte(fmt.Sprintf("%q", strconv.FormatInt(probation, 10))))
		require.NoError(t, err)

		state, err = sjson.SetRawBytes(state,
			"app_state.audit.params.storage_truth_node_suspicion_threshold_watch",
			[]byte(fmt.Sprintf("%q", strconv.FormatInt(watchThreshold, 10))))
		require.NoError(t, err)

		state, err = sjson.SetRawBytes(state,
			"app_state.audit.params.storage_truth_ticket_deterioration_heal_threshold",
			[]byte(fmt.Sprintf("%q", strconv.FormatInt(healThreshold, 10))))
		require.NoError(t, err)

		state, err = sjson.SetRawBytes(state,
			"app_state.audit.params.storage_truth_node_suspicion_decay_per_epoch",
			[]byte(fmt.Sprintf("%q", strconv.FormatInt(decayPerEpoch, 10))))
		require.NoError(t, err)

		// uint32: proto3 JSON number.
		state, err = sjson.SetRawBytes(state,
			"app_state.audit.params.storage_truth_max_self_heal_ops_per_epoch",
			[]byte(strconv.FormatUint(uint64(maxHealOps), 10)))
		require.NoError(t, err)

		// divisor=1 ensures every active node gets an assignment so tests can always
		// find a prober for any target (needed to seed transcript records for recheck).
		state, err = sjson.SetRawBytes(state,
			"app_state.audit.params.storage_truth_challenge_target_divisor",
			[]byte("1"))
		require.NoError(t, err)

		// strong_postpone must be >= postpone to satisfy params.Validate() in InitGenesis.
		strongPostpone := postponeThreshold + 200
		state, err = sjson.SetRawBytes(state,
			"app_state.audit.params.storage_truth_node_suspicion_threshold_strong_postpone",
			[]byte(fmt.Sprintf("%q", strconv.FormatInt(strongPostpone, 10))))
		require.NoError(t, err)

		state = seedStorageTruthSyntheticTicketCounts(t, state)

		return state
	}
}

// ── gRPC query helpers ────────────────────────────────────────────────────────

func auditQueryNodeSuspicionStateST(t *testing.T, supernodeAccount string) (audittypes.NodeSuspicionState, bool) {
	t.Helper()
	conn, err := grpc.Dial("localhost:9090", grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	qc := audittypes.NewQueryClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := qc.NodeSuspicionState(ctx, &audittypes.QueryNodeSuspicionStateRequest{
		SupernodeAccount: supernodeAccount,
	})
	if err != nil {
		return audittypes.NodeSuspicionState{}, false
	}
	return resp.State, true
}

func auditQueryTicketDeteriorationStateST(t *testing.T, ticketID string) (audittypes.TicketDeteriorationState, bool) {
	t.Helper()
	conn, err := grpc.Dial("localhost:9090", grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	qc := audittypes.NewQueryClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := qc.TicketDeteriorationState(ctx, &audittypes.QueryTicketDeteriorationStateRequest{
		TicketId: ticketID,
	})
	if err != nil {
		return audittypes.TicketDeteriorationState{}, false
	}
	return resp.State, true
}

func auditQueryHealOpsByTicketST(t *testing.T, ticketID string) []audittypes.HealOp {
	t.Helper()
	conn, err := grpc.Dial("localhost:9090", grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	qc := audittypes.NewQueryClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := qc.HealOpsByTicket(ctx, &audittypes.QueryHealOpsByTicketRequest{
		TicketId: ticketID,
	})
	if err != nil {
		return nil
	}
	return resp.HealOps
}

// ── CLI transaction helpers ───────────────────────────────────────────────────

func submitStorageRecheckEvidence(
	t *testing.T,
	cli *LumeradCli,
	fromNode string,
	epochID uint64,
	challengedAccount string,
	ticketID string,
	challengedHash string,
	recheckHash string,
	resultClass string,
) string {
	t.Helper()
	return cli.CustomCommand(
		"tx", "audit", "submit-storage-recheck-evidence",
		strconv.FormatUint(epochID, 10),
		challengedAccount,
		ticketID,
		"--challenged-result-transcript-hash", challengedHash,
		"--recheck-transcript-hash", recheckHash,
		"--recheck-result-class", resultClass,
		"--from", fromNode,
	)
}

func submitClaimHealCompleteST(
	t *testing.T,
	cli *LumeradCli,
	fromNode string,
	healOpID uint64,
	ticketID string,
	manifestHash string,
) string {
	t.Helper()
	return cli.CustomCommand(
		"tx", "audit", "claim-heal-complete",
		strconv.FormatUint(healOpID, 10),
		ticketID,
		manifestHash,
		"--from", fromNode,
	)
}

func submitHealVerificationST(
	t *testing.T,
	cli *LumeradCli,
	fromNode string,
	healOpID uint64,
	verified bool,
	verificationHash string,
) string {
	t.Helper()
	return cli.CustomCommand(
		"tx", "audit", "submit-heal-verification",
		strconv.FormatUint(healOpID, 10),
		strconv.FormatBool(verified),
		verificationHash,
		"--from", fromNode,
	)
}

// ── tests ─────────────────────────────────────────────────────────────────────

// TestStorageTruth_RecheckEvidence_UpdatesScore verifies that submitting a
// SubmitStorageRecheckEvidence message with RECHECK_CONFIRMED_FAIL updates both:
//   - the node suspicion score for the challenged supernode, and
//   - the ticket deterioration score for the given ticket ID.
//
// This test validates the recheck evidence path end-to-end without relying on
// epoch-end enforcement (so it completes in a single epoch).
func TestStorageTruth_RecheckEvidence_UpdatesScore(t *testing.T) {
	const (
		epochLengthBlocks = uint64(15)
		originHeight      = int64(1)
		ticketID          = "sys-test-ticket-recheck-1"
	)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastEpochs(t, epochLengthBlocks, 1, 1, 1, []uint32{4444}),
		setStorageTruthTestParams(t,
			"STORAGE_TRUTH_ENFORCEMENT_MODE_SOFT",
			100, // postpone threshold (high — don't trigger postponement in this test)
			20,  // watch threshold
			10,  // heal threshold
			0,   // decay (disable for predictable scores)
			5,   // max heal ops per epoch
		),
		func(genesis []byte) []byte {
			// Disable missing-report postponement to avoid interference.
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

	// Wait for epoch 1 to start so the anchor exists and registered nodes are included.
	currentHeight := sut.AwaitNextBlock(t)
	epochID1, epoch1Start := nextEpochAfterHeight(originHeight, epochLengthBlocks, currentHeight)
	awaitAtLeastHeight(t, epoch1Start)
	_, _, target := findAssignedProberAndTarget(t, epochID1, nodes)

	// Verify no score exists for the challenged node yet.
	_, found := auditQueryNodeSuspicionStateST(t, target.accAddr)
	require.False(t, found, "target should have no suspicion state before any recheck evidence")

	// Seed the transcript record: find which candidate is assigned target, have them submit an epoch
	// report with an INVALID_TRANSCRIPT result so the transcript KV store is populated.
	// Returns the rechecker (a candidate ≠ prober, ≠ n1) for the subsequent recheck call.
	rechecker := seedProofTranscripts(t, cli, epochID1, nodes, target.accAddr,
		[]transcriptSeed{{ticketID: ticketID, transcriptHash: "old-transcript-hash-1"}}, false)

	// Submit recheck evidence from rechecker against the assigned target with RECHECK_CONFIRMED_FAIL.
	// RECHECK_CONFIRMED_FAIL adds +15 to node suspicion and +8 to ticket deterioration
	// (with NORMAL trust band, 100% multiplier).
	recheckResp := submitStorageRecheckEvidence(t, cli,
		rechecker.nodeName,
		epochID1,
		target.accAddr,
		ticketID,
		"old-transcript-hash-1",
		"recheck-transcript-hash-1",
		"recheck-confirmed-fail", // STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL = 7
	)
	RequireTxSuccess(t, recheckResp)
	sut.AwaitNextBlock(t)

	// Node suspicion score for the challenged target should now be 15.
	nodeState, found := auditQueryNodeSuspicionStateST(t, target.accAddr)
	require.True(t, found, "target should have suspicion state after recheck evidence")
	require.Equal(t, int64(15), nodeState.SuspicionScore, "target suspicion should be +15 from RECHECK_CONFIRMED_FAIL")

	// Ticket deterioration score should also be 8.
	ticketState, found := auditQueryTicketDeteriorationStateST(t, ticketID)
	require.True(t, found, "ticket should have deterioration state after recheck evidence")
	require.Equal(t, int64(8), ticketState.DeteriorationScore, "ticket deterioration should be +8")
}

// TestStorageTruth_SoftMode_PostponesNodeOnHighSuspicion verifies that in SOFT enforcement
// mode a supernode is postponed at epoch end when its suspicion score meets or exceeds
// the configured postpone threshold.
//
// Setup:
//   - postpone_threshold = 10 (single recheck evidence result of +20 exceeds threshold)
//   - enforcement_mode  = SOFT
//
// Expected: the challenged node is POSTPONED after epoch end; the challenger stays ACTIVE.
func TestStorageTruth_SoftMode_PostponesNodeOnHighSuspicion(t *testing.T) {
	const (
		epochLengthBlocks = uint64(12)
		originHeight      = int64(1)
		ticketID          = "sys-test-ticket-soft-postpone"
	)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastEpochs(t, epochLengthBlocks, 1, 1, 1, []uint32{}),
		setStorageTruthTestParams(t,
			"STORAGE_TRUTH_ENFORCEMENT_MODE_SOFT",
			10, // postpone threshold — score of 15 exceeds this
			5,  // watch threshold
			10, // heal threshold
			0,  // no decay
			5,
		),
		func(genesis []byte) []byte {
			// High consecutive threshold prevents missing-report postponement.
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

	// Wait for epoch 1 start so epoch anchor is created with all nodes as ACTIVE.
	currentHeight := sut.AwaitNextBlock(t)
	epochID1, epoch1Start := nextEpochAfterHeight(originHeight, epochLengthBlocks, currentHeight)
	epoch1End := epoch1Start + int64(epochLengthBlocks)
	awaitAtLeastHeight(t, epoch1Start)
	_, _, target := findAssignedProberAndTarget(t, epochID1, nodes)

	// Seed the transcript record so the subsequent recheck evidence call is accepted.
	rechecker := seedProofTranscripts(t, cli, epochID1, nodes, target.accAddr,
		[]transcriptSeed{{ticketID: ticketID, transcriptHash: "challenged-hash"}}, false)

	// rechecker submits recheck evidence against target; target suspicion score becomes 15.
	// This exceeds the postpone_threshold of 10.
	recheckResp := submitStorageRecheckEvidence(t, cli,
		rechecker.nodeName,
		epochID1,
		target.accAddr,
		ticketID,
		"challenged-hash",
		"recheck-hash",
		"recheck-confirmed-fail", // RECHECK_CONFIRMED_FAIL
	)
	RequireTxSuccess(t, recheckResp)

	// Wait for epoch end → enforcement runs → n1 should be postponed.
	awaitAtLeastHeight(t, epoch1End)
	sut.AwaitNextBlock(t)

	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, target.valAddr),
		"target should be POSTPONED after suspicion score exceeds SOFT-mode postpone threshold 10")
	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, rechecker.valAddr),
		"rechecker should remain ACTIVE")
}

// TestStorageTruth_ShadowMode_NoPostponement verifies that in SHADOW enforcement mode
// events are emitted but supernodes are NOT postponed even when the suspicion score
// exceeds the postpone threshold.
//
// This is identical to the SOFT-mode test except enforcement_mode = SHADOW.
func TestStorageTruth_ShadowMode_NoPostponement(t *testing.T) {
	const (
		epochLengthBlocks = uint64(12)
		originHeight      = int64(1)
		ticketID          = "sys-test-ticket-shadow-nopostpone"
	)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastEpochs(t, epochLengthBlocks, 1, 1, 1, []uint32{}),
		setStorageTruthTestParams(t,
			"STORAGE_TRUTH_ENFORCEMENT_MODE_SHADOW",
			10, // postpone threshold — score of 15 exceeds this, but SHADOW mode ignores it
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
	nodes := []testNodeIdentity{n0, n1, n2}

	currentHeight := sut.AwaitNextBlock(t)
	epochID1, epoch1Start := nextEpochAfterHeight(originHeight, epochLengthBlocks, currentHeight)
	epoch1End := epoch1Start + int64(epochLengthBlocks)
	awaitAtLeastHeight(t, epoch1Start)
	_, _, target := findAssignedProberAndTarget(t, epochID1, nodes)

	rechecker := seedProofTranscripts(t, cli, epochID1, nodes, target.accAddr,
		[]transcriptSeed{{ticketID: ticketID, transcriptHash: "challenged-hash-shadow"}}, false)

	// Push target suspicion to 15 (above postpone_threshold=10).
	recheckResp := submitStorageRecheckEvidence(t, cli,
		rechecker.nodeName,
		epochID1,
		target.accAddr,
		ticketID,
		"challenged-hash-shadow",
		"recheck-hash-shadow",
		"recheck-confirmed-fail", // RECHECK_CONFIRMED_FAIL
	)
	RequireTxSuccess(t, recheckResp)

	// Wait for epoch end — enforcement runs in SHADOW mode.
	awaitAtLeastHeight(t, epoch1End)
	sut.AwaitNextBlock(t)

	// In SHADOW mode the node must NOT be postponed despite score exceeding threshold.
	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, target.valAddr),
		"target should remain ACTIVE in SHADOW mode even when suspicion exceeds postpone threshold")
	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, rechecker.valAddr),
		"rechecker should remain ACTIVE")
}

// TestStorageTruth_HealOp_ScheduledAndVerified covers the complete self-heal lifecycle:
//
//  1. Index proof failure exceeds the heal threshold at epoch end → heal op scheduled.
//  2. Healer submits ClaimHealComplete.
//  3. Each assigned verifier submits SubmitHealVerification(verified=true).
//  4. Once all verifiers have voted, the heal op status becomes VERIFIED.
//  5. Ticket deterioration score is reset to max(8, floor(D_old * 0.25)).
//
// Three supernodes are registered so the scheduler can assign a healer and up to 2 verifiers.
// The node suspicion postpone threshold is set very high (1000) so no node gets postponed
// during the test, keeping all three nodes ACTIVE throughout.
func TestStorageTruth_HealOp_ScheduledAndVerified(t *testing.T) {
	const (
		epochLengthBlocks = uint64(12)
		originHeight      = int64(1)
		ticketID          = "sys-test-ticket-heal-lifecycle-1"
	)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastEpochs(t, epochLengthBlocks, 1, 1, 1, []uint32{}),
		setStorageTruthTestParams(t,
			"STORAGE_TRUTH_ENFORCEMENT_MODE_SOFT",
			1000, // postpone threshold — very high, no node gets postponed
			500,  // watch threshold
			10,   // heal threshold — index hash mismatch gives +12, above 10
			0,    // no decay
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

	// Build a map from supernode account address → CLI node name for dynamic dispatch.
	nodeForAccount := map[string]testNodeIdentity{
		n0.accAddr: n0,
		n1.accAddr: n1,
		n2.accAddr: n2,
	}

	// Wait for epoch 1 start so the anchor includes the three registered supernodes.
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
			"orig-transcript-hash",
			"STORAGE_PROOF_BUCKET_TYPE_RECENT",
			"STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH",
		),
	}
	for _, target := range proberResp.TargetSupernodeAccounts {
		args = append(args, "--storage-challenge-observations", storageChallengeObservationJSON(target, portStates))
	}
	proofResp := cli.CustomCommand(args...)
	RequireTxSuccess(t, proofResp)

	// Verify ticket deterioration is 12 before epoch end.
	sut.AwaitNextBlock(t)
	ticketBefore, found := auditQueryTicketDeteriorationStateST(t, ticketID)
	require.True(t, found, "ticket state should exist after index proof failure")
	require.Equal(t, int64(12), ticketBefore.DeteriorationScore)

	// Wait for epoch 1 end — heal ops are scheduled for tickets above the heal threshold.
	awaitAtLeastHeight(t, epoch1End)
	sut.AwaitNextBlock(t)

	// There should be exactly one heal op scheduled for our ticket.
	healOps := auditQueryHealOpsByTicketST(t, ticketID)
	require.Len(t, healOps, 1, "exactly one heal op should be scheduled for the ticket")

	healOp := healOps[0]
	require.Equal(t, ticketID, healOp.TicketId)
	require.Equal(t, audittypes.HealOpStatus_HEAL_OP_STATUS_SCHEDULED, healOp.Status,
		"heal op should be in SCHEDULED status after epoch end")
	t.Logf("heal op ID=%d healer=%s verifiers=%v", healOp.HealOpId, healOp.HealerSupernodeAccount, healOp.VerifierSupernodeAccounts)

	// Identify which CLI node is the healer.
	healerID, ok := nodeForAccount[healOp.HealerSupernodeAccount]
	require.True(t, ok, "healer account %q must be one of the three registered supernodes", healOp.HealerSupernodeAccount)

	// Healer claims heal complete.
	claimResp := submitClaimHealCompleteST(t, cli,
		healerID.nodeName,
		healOp.HealOpId,
		ticketID,
		"heal-manifest-hash-1",
	)
	RequireTxSuccess(t, claimResp)
	sut.AwaitNextBlock(t)

	if len(healOp.VerifierSupernodeAccounts) == 0 {
		// Single-node network: ClaimHealComplete finalizes immediately.
		finalOps := auditQueryHealOpsByTicketST(t, ticketID)
		require.Len(t, finalOps, 1)
		require.Equal(t, audittypes.HealOpStatus_HEAL_OP_STATUS_VERIFIED, finalOps[0].Status,
			"single-node heal op should finalize to VERIFIED immediately on ClaimHealComplete")
	} else {
		// Multi-node network: verifiers must each submit their verification.
		for i, verifierAccount := range healOp.VerifierSupernodeAccounts {
			verifierID, ok := nodeForAccount[verifierAccount]
			require.True(t, ok, "verifier account %q must be one of the three registered supernodes", verifierAccount)

			verifyResp := submitHealVerificationST(t, cli,
				verifierID.nodeName,
				healOp.HealOpId,
				true,
				fmt.Sprintf("verify-hash-%d", i),
			)
			RequireTxSuccess(t, verifyResp)
			sut.AwaitNextBlock(t)
		}
	}

	// Verify heal op finalized as VERIFIED.
	finalOps := auditQueryHealOpsByTicketST(t, ticketID)
	require.Len(t, finalOps, 1)
	require.Equal(t, audittypes.HealOpStatus_HEAL_OP_STATUS_VERIFIED, finalOps[0].Status,
		"heal op should be VERIFIED after all verifiers confirmed")

	// Ticket deterioration score should be reset: D = max(8, floor(12 * 0.25)) = max(8, 3) = 8.
	ticketAfter, found := auditQueryTicketDeteriorationStateST(t, ticketID)
	require.True(t, found, "ticket state should still exist after heal verification")
	require.Equal(t, int64(8), ticketAfter.DeteriorationScore,
		"ticket deterioration should be reset to max(8, floor(12*0.25)) = 8 after verified heal")
}
