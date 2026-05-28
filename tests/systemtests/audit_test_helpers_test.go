//go:build system_test

package system

// This file contains helper functions used by the audit module systemtests.
//
// Why helpers exist here:
// - The audit module behavior depends heavily on block height (epoch boundaries).
// - The systemtest harness runs a real multi-node testnet; we need stable ways to:
//   - pick a safe epoch to test against (avoid racing enforcement),
//   - derive deterministic peer targets (same logic as the keeper),
//   - submit reports via CLI,
//   - query results reliably (gRPC where CLI JSON marshalling is known to break).

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	client "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"lukechampine.com/blake3"

	lcfg "github.com/LumeraProtocol/lumera/config"
	audittypes "github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// setAuditParamsForFastEpochs overrides audit module params in genesis so tests complete quickly.
func setAuditParamsForFastEpochs(t *testing.T, epochLengthBlocks uint64, peerQuorumReports, minTargets, maxTargets uint32, requiredOpenPorts []uint32) GenesisMutator {
	return func(genesis []byte) []byte {
		t.Helper()

		state := genesis
		var err error

		state, err = sjson.SetRawBytes(state, "app_state.audit.params.epoch_length_blocks", []byte(fmt.Sprintf("%q", strconv.FormatUint(epochLengthBlocks, 10))))
		require.NoError(t, err)
		// In system tests, start epoch 0 at height 1 (the first block height on a fresh chain).
		state, err = sjson.SetRawBytes(state, "app_state.audit.params.epoch_zero_height", []byte(fmt.Sprintf("%q", "1")))
		require.NoError(t, err)

		state, err = sjson.SetRawBytes(state, "app_state.audit.params.peer_quorum_reports", []byte(strconv.FormatUint(uint64(peerQuorumReports), 10)))
		require.NoError(t, err)
		state, err = sjson.SetRawBytes(state, "app_state.audit.params.min_probe_targets_per_epoch", []byte(strconv.FormatUint(uint64(minTargets), 10)))
		require.NoError(t, err)
		state, err = sjson.SetRawBytes(state, "app_state.audit.params.max_probe_targets_per_epoch", []byte(strconv.FormatUint(uint64(maxTargets), 10)))
		require.NoError(t, err)

		portsJSON, err := json.Marshal(requiredOpenPorts)
		require.NoError(t, err)
		state, err = sjson.SetRawBytes(state, "app_state.audit.params.required_open_ports", portsJSON)
		require.NoError(t, err)

		return state
	}
}

// setAuditParamsForFastEpochsWithMinCpuFree is setAuditParamsForFastEpochs
// plus an explicit MinCpuFreePercent override. Used by tests that need to
// exercise the self-compliance gate against a non-storage host minimum
// (e.g. the empty-active-set bootstrap exception's self-compliance guard).
//
// Note: disk pressure is intentionally NOT used for postpone/self-compliance
// gating — per LEP-6 it is owned exclusively by the STORAGE_FULL transition
// path in audit SetReport. Tests that want to keep an SN out of the recovery
// path must drive non-compliance via a non-storage metric (cpu or mem).
func setAuditParamsForFastEpochsWithMinCpuFree(t *testing.T, epochLengthBlocks uint64, peerQuorumReports, minTargets, maxTargets uint32, requiredOpenPorts []uint32, minCpuFreePercent uint32) GenesisMutator {
	base := setAuditParamsForFastEpochs(t, epochLengthBlocks, peerQuorumReports, minTargets, maxTargets, requiredOpenPorts)
	return func(genesis []byte) []byte {
		t.Helper()
		state := base(genesis)
		var err error
		state, err = sjson.SetRawBytes(state, "app_state.audit.params.min_cpu_free_percent", []byte(strconv.FormatUint(uint64(minCpuFreePercent), 10)))
		require.NoError(t, err)
		return state
	}
}

// setSupernodeParamsForAuditTests keeps supernode registration permissive for test environments.
//
// These tests register supernodes and then submit audit reports "on their behalf" using node keys.
// We keep minimum stake and min version permissive so registration is not the bottleneck.
func setSupernodeParamsForAuditTests(t *testing.T) GenesisMutator {
	return func(genesis []byte) []byte {
		t.Helper()

		state, err := sjson.SetRawBytes(genesis, "app_state.supernode.params.min_supernode_version", []byte(`"0.0.0"`))
		require.NoError(t, err)

		coinJSON := fmt.Sprintf(`{"denom":"%s","amount":"1"}`, lcfg.ChainDenom)
		state, err = sjson.SetRawBytes(state, "app_state.supernode.params.minimum_stake_for_sn", []byte(coinJSON))
		require.NoError(t, err)

		return state
	}
}

func awaitAtLeastHeight(t *testing.T, height int64, timeout ...time.Duration) {
	t.Helper()
	if sut.currentHeight >= height {
		return
	}
	sut.AwaitBlockHeight(t, height, timeout...)
}

// pickEpochForStartAtOrAfter returns the first epoch whose start height is >= minStartHeight.
// This is a "ceiling" epoch picker.
func pickEpochForStartAtOrAfter(originHeight int64, epochBlocks uint64, minStartHeight int64) (epochID uint64, startHeight int64) {
	if epochBlocks == 0 {
		return 0, originHeight
	}
	if minStartHeight < originHeight {
		minStartHeight = originHeight
	}

	blocks := int64(epochBlocks)
	delta := minStartHeight - originHeight
	epochID = uint64((delta + blocks - 1) / blocks) // ceil(delta/blocks)
	startHeight = originHeight + int64(epochID)*blocks
	return epochID, startHeight
}

// nextEpochAfterHeight returns the next epoch after the provided height.
//
// We use this in tests to:
// - register supernodes first,
// - then wait for the *next* epoch boundary to ensure snapshot inclusion and acceptance.
func nextEpochAfterHeight(originHeight int64, epochBlocks uint64, height int64) (epochID uint64, startHeight int64) {
	if epochBlocks == 0 {
		return 0, originHeight
	}
	if height < originHeight {
		return 0, originHeight
	}
	blocks := int64(epochBlocks)
	currentID := uint64((height - originHeight) / blocks)
	epochID = currentID + 1
	startHeight = originHeight + int64(epochID)*blocks
	return epochID, startHeight
}

type testNodeIdentity struct {
	nodeName string
	accAddr  string
	valAddr  string
}

// getNodeIdentity reads the node's account and validator operator address from the systemtest keyring.
func getNodeIdentity(t *testing.T, cli *LumeradCli, nodeName string) testNodeIdentity {
	t.Helper()
	accAddr := cli.GetKeyAddr(nodeName)
	valAddr := strings.TrimSpace(cli.Keys("keys", "show", nodeName, "--bech", "val", "-a"))
	require.NotEmpty(t, accAddr)
	require.NotEmpty(t, valAddr)
	return testNodeIdentity{nodeName: nodeName, accAddr: accAddr, valAddr: valAddr}
}

// registerSupernode registers a supernode using the node's own key as both:
// - the tx signer (via --from),
// - the supernode_account (so that later MsgSubmitEpochReport signatures match).
func registerSupernode(t *testing.T, cli *LumeradCli, id testNodeIdentity, ip string) {
	t.Helper()
	resp := cli.CustomCommand(
		"tx", "supernode", "register-supernode",
		id.valAddr,
		ip,
		id.accAddr,
		"--from", id.nodeName,
	)
	RequireTxSuccess(t, resp)
	sut.AwaitNextBlock(t)
}

// headerHashAtHeight fetches the block header hash at an exact height.
// The audit module uses ctx.HeaderHash() as the snapshot seed; the assignment logic needs this seed.
func headerHashAtHeight(t *testing.T, rpcAddr string, height int64) []byte {
	t.Helper()
	httpClient, err := client.New(rpcAddr, "/websocket")
	require.NoError(t, err)
	require.NoError(t, httpClient.Start())
	t.Cleanup(func() { _ = httpClient.Stop() })

	res, err := httpClient.Block(context.Background(), &height)
	require.NoError(t, err)
	hash := res.Block.Header.Hash()
	require.True(t, len(hash) >= 8, "expected header hash >= 8 bytes")
	return []byte(hash)
}

func epochSeedAtHeight(t *testing.T, rpcAddr string, height int64, epochID uint64) []byte {
	t.Helper()

	raw := headerHashAtHeight(t, rpcAddr, height)
	epochBz := make([]byte, 8)
	binary.BigEndian.PutUint64(epochBz, epochID)

	var msg bytes.Buffer
	msg.WriteString("lumera:epoch-seed")
	msg.Write(raw)
	msg.Write(epochBz)

	sum := blake3.Sum256(msg.Bytes())
	out := make([]byte, len(sum))
	copy(out, sum[:])
	return out
}

// computeKEpoch replicates x/audit/v1/keeper.computeKWindow to keep tests deterministic and black-box.
// It computes how many peer targets each sender must probe this epoch.
func computeKEpoch(peerQuorumReports, minTargets, maxTargets uint32, sendersCount, receiversCount int) uint32 {
	if sendersCount <= 0 || receiversCount <= 1 {
		return 0
	}

	a := uint64(sendersCount)
	n := uint64(receiversCount)
	q := uint64(peerQuorumReports)
	kNeeded := (q*n + a - 1) / a

	kMin := uint64(minTargets)
	kMax := uint64(maxTargets)
	if kNeeded < kMin {
		kNeeded = kMin
	}
	if kNeeded > kMax {
		kNeeded = kMax
	}
	if kNeeded > n-1 {
		kNeeded = n - 1
	}

	return uint32(kNeeded)
}

// assignedTargets replicates x/audit/v1/keeper.assignedTargets.
//
// Notes:
// - The assignment is order-sensitive; the module enforces that peer observations match targets by index.
// - We use this to build exactly-valid test reports.
func assignedTargets(seed []byte, senders, receivers []string, kWindow uint32, senderSupernodeAccount string) ([]string, bool) {
	k := int(kWindow)
	if k == 0 || len(receivers) == 0 {
		return []string{}, true
	}

	senderIndex := -1
	for i, s := range senders {
		if s == senderSupernodeAccount {
			senderIndex = i
			break
		}
	}
	if senderIndex < 0 {
		return nil, false
	}
	if len(seed) < 8 {
		return nil, false
	}

	n := len(receivers)
	offsetU64 := binary.BigEndian.Uint64(seed[:8])
	offset := int(offsetU64 % uint64(n))

	seen := make(map[int]struct{}, k)
	out := make([]string, 0, k)

	for j := 0; j < k; j++ {
		slot := senderIndex*k + j
		candidate := (offset + slot) % n

		tries := 0
		for tries < n {
			if receivers[candidate] != senderSupernodeAccount {
				if _, ok := seen[candidate]; !ok {
					break
				}
			}
			candidate = (candidate + 1) % n
			tries++
		}
		if tries >= n {
			break
		}

		seen[candidate] = struct{}{}
		out = append(out, receivers[candidate])
	}

	return out, true
}

// auditHostReportJSON builds the JSON payload for the positional host-report argument.
// HostReport contains float fields (cpu/mem/disk), so we keep values simple.
func auditHostReportJSON(inboundPortStates []string) string {
	bz, _ := json.Marshal(map[string]any{
		"cpu_usage_percent":    1.0,
		"mem_usage_percent":    1.0,
		"disk_usage_percent":   1.0,
		"inbound_port_states":  inboundPortStates,
		"failed_actions_count": 0,
	})
	return string(bz)
}

// auditHostReportWithDiskUsageJSON is like auditHostReportJSON but lets the
// caller pin disk_usage_percent. Used by tests that exercise the
// STORAGE_FULL transition via the SetReport disk-pressure path.
func auditHostReportWithDiskUsageJSON(inboundPortStates []string, diskUsagePercent float64) string {
	bz, _ := json.Marshal(map[string]any{
		"cpu_usage_percent":    1.0,
		"mem_usage_percent":    1.0,
		"disk_usage_percent":   diskUsagePercent,
		"inbound_port_states":  inboundPortStates,
		"failed_actions_count": 0,
	})
	return string(bz)
}

// auditHostReportWithCpuUsageJSON is like auditHostReportJSON but lets the
// caller pin cpu_usage_percent. Used by tests that exercise the
// self-compliance gate against a non-storage host minimum.
func auditHostReportWithCpuUsageJSON(inboundPortStates []string, cpuUsagePercent float64) string {
	bz, _ := json.Marshal(map[string]any{
		"cpu_usage_percent":    cpuUsagePercent,
		"mem_usage_percent":    1.0,
		"disk_usage_percent":   1.0,
		"inbound_port_states":  inboundPortStates,
		"failed_actions_count": 0,
	})
	return string(bz)
}

// storageChallengeObservationJSON builds the JSON payload for --storage-challenge-observations flag.
func storageChallengeObservationJSON(targetSupernodeAccount string, portStates []string) string {
	bz, _ := json.Marshal(map[string]any{
		"target_supernode_account": targetSupernodeAccount,
		"port_states":              portStates,
	})
	return string(bz)
}

// submitEpochReport submits a report using the AutoCLI command:
//
//	tx audit submit-epoch-report [epoch-id] [host-report-json] --storage-challenge-observations <json>...
//
// We keep it as a CLI call to validate the end-to-end integration path (signer handling, encoding).
func submitEpochReport(t *testing.T, cli *LumeradCli, fromNode string, epochID uint64, hostReportJSON string, storageChallengeObservationJSONs []string) string {
	t.Helper()

	args := []string{"tx", "audit", "submit-epoch-report", strconv.FormatUint(epochID, 10), hostReportJSON, "--from", fromNode}
	for _, obs := range storageChallengeObservationJSONs {
		args = append(args, "--storage-challenge-observations", obs)
	}

	return cli.CustomCommand(args...)
}

// querySupernodeLatestState reads the latest supernode state string (e.g. "SUPERNODE_STATE_POSTPONED") via CLI JSON.
func querySupernodeLatestState(t *testing.T, cli *LumeradCli, validatorAddress string) string {
	t.Helper()
	resp := cli.CustomQuery("q", "supernode", "get-supernode", validatorAddress)
	states := gjson.Get(resp, "supernode.states")
	require.True(t, states.Exists(), "missing states: %s", resp)
	arr := states.Array()
	require.NotEmpty(t, arr, "missing states: %s", resp)
	return arr[len(arr)-1].Get("state").String()
}

// gjsonUint64 is a small helper because some CLI outputs represent uint64 as strings.
func gjsonUint64(v gjson.Result) uint64 {
	if !v.Exists() {
		return 0
	}
	if v.Type == gjson.Number {
		return uint64(v.Uint())
	}
	if v.Type == gjson.String {
		out, err := strconv.ParseUint(v.String(), 10, 64)
		if err != nil {
			return 0
		}
		return out
	}
	return 0
}

func sortedStrings(in ...string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

// newAuditQueryClient creates a gRPC query client against node0's gRPC endpoint.
//
//   - `EpochReport` contains float fields; CLI JSON marshalling for those fields is currently broken
//     in this environment and fails with "unknown type float64".
func newAuditQueryClient(t *testing.T) (audittypes.QueryClient, func()) {
	t.Helper()
	conn, err := grpc.Dial("localhost:9090", grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	closeFn := func() { _ = conn.Close() }
	t.Cleanup(closeFn)
	return audittypes.NewQueryClient(conn), closeFn
}

// auditQueryReport queries a stored report via gRPC.
func auditQueryReport(t *testing.T, epochID uint64, reporterSupernodeAccount string) audittypes.EpochReport {
	t.Helper()
	qc, _ := newAuditQueryClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := qc.EpochReport(ctx, &audittypes.QueryEpochReportRequest{
		EpochId:          epochID,
		SupernodeAccount: reporterSupernodeAccount,
	})
	require.NoError(t, err)
	return resp.Report
}

func auditQueryReporterReliabilityState(t *testing.T, reporterSupernodeAccount string) audittypes.ReporterReliabilityState {
	t.Helper()
	qc, _ := newAuditQueryClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := qc.ReporterReliabilityState(ctx, &audittypes.QueryReporterReliabilityStateRequest{
		ReporterSupernodeAccount: reporterSupernodeAccount,
	})
	require.NoError(t, err)
	return resp.State
}

func auditQueryAssignedTargets(t *testing.T, epochID uint64, filterByEpochID bool, proberSupernodeAccount string) audittypes.QueryAssignedTargetsResponse {
	t.Helper()
	qc, _ := newAuditQueryClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := qc.AssignedTargets(ctx, &audittypes.QueryAssignedTargetsRequest{
		EpochId:          epochID,
		FilterByEpochId:  filterByEpochID,
		SupernodeAccount: proberSupernodeAccount,
	})
	require.NoError(t, err)
	return *resp
}

func awaitCurrentEpochAnchorWithActiveSupernodes(t *testing.T, minEpochID uint64, expectedAccounts ...string) audittypes.EpochAnchor {
	t.Helper()
	qc, _ := newAuditQueryClient(t)
	deadline := time.Now().Add(2 * time.Minute)
	var last audittypes.EpochAnchor
	var lastErr error

	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		resp, err := qc.CurrentEpochAnchor(ctx, &audittypes.QueryCurrentEpochAnchorRequest{})
		cancel()
		if err == nil {
			last = resp.Anchor
			if last.EpochId >= minEpochID && containsAllStrings(last.ActiveSupernodeAccounts, expectedAccounts...) && containsAllStrings(last.TargetSupernodeAccounts, expectedAccounts...) {
				return last
			}
		} else {
			lastErr = err
		}
		sut.AwaitNextBlock(t)
	}

	require.FailNowf(t,
		"epoch anchor did not include expected supernodes",
		"min_epoch_id=%d expected=%v last_epoch_id=%d last_active=%v last_targets=%v last_err=%v",
		minEpochID,
		expectedAccounts,
		last.EpochId,
		last.ActiveSupernodeAccounts,
		last.TargetSupernodeAccounts,
		lastErr,
	)
	return audittypes.EpochAnchor{}
}

func containsAllStrings(values []string, needles ...string) bool {
	for _, needle := range needles {
		if !containsString(values, needle) {
			return false
		}
	}
	return true
}

// setStorageTruthEnforcementModeUnspecified sets enforcement_mode=UNSPECIFIED in genesis.
// Use this for tests that rely on the k-based peer-assignment formula rather than the
// storage-truth one-third coverage formula that activates under any non-UNSPECIFIED mode.
func setStorageTruthEnforcementModeUnspecified(t *testing.T) GenesisMutator {
	return func(genesis []byte) []byte {
		t.Helper()
		state, err := sjson.SetRawBytes(genesis,
			"app_state.audit.params.storage_truth_enforcement_mode",
			[]byte(`"STORAGE_TRUTH_ENFORCEMENT_MODE_UNSPECIFIED"`))
		require.NoError(t, err)
		return state
	}
}

func seedStorageTruthSyntheticTicketCounts(t *testing.T, genesis []byte) []byte {
	t.Helper()

	ticketIDs := []string{
		"sys-test-ticket-recheck-1",
		"sys-test-ticket-soft-postpone",
		"sys-test-ticket-shadow-nopostpone",
		"sys-test-ticket-heal-lifecycle-1",
		"edge-ticket-full-mode-recent",
		"edge-ticket-full-mode-old",
		"edge-ticket-unspecified",
		"edge-ticket-failed-heal",
		"edge-ticket-replay",
	}
	for i := 0; i < 3; i++ {
		ticketIDs = append(ticketIDs, fmt.Sprintf("edge-ticket-decay-%d", i))
	}
	for i := 0; i < 4; i++ {
		ticketIDs = append(ticketIDs, fmt.Sprintf("multi-ticket-%d", i))
	}

	states := make([]map[string]any, 0, len(ticketIDs))
	for _, ticketID := range ticketIDs {
		states = append(states, map[string]any{
			"ticket_id":             ticketID,
			"index_artifact_count":  8,
			"symbol_artifact_count": 8,
		})
	}
	bz, err := json.Marshal(states)
	require.NoError(t, err)

	state, err := sjson.SetRawBytes(genesis, "app_state.audit.ticket_artifact_count_states", bz)
	require.NoError(t, err)
	return state
}

// buildStorageProofResultJSON builds a single StorageProofResult JSON object for the
// --storage-proof-results CLI flag.
//
// Uses INVALID_TRANSCRIPT result class: score-neutral (nodeSuspicion=0, ticketDeterioration=0)
// but recheck-eligible, so it seeds the on-chain transcript KV store without corrupting
// any node-suspicion or ticket-deterioration score assertions in the test.
func buildStorageProofResultJSONWithClass(challengerAcct, targetAcct, ticketID, transcriptHash, bucketType, resultClass string) string {
	bz, _ := json.Marshal(map[string]any{
		"target_supernode_account":     targetAcct,
		"challenger_supernode_account": challengerAcct,
		"ticket_id":                    ticketID,
		"transcript_hash":              transcriptHash,
		"bucket_type":                  bucketType,
		"result_class":                 resultClass,
		"artifact_class":               "STORAGE_PROOF_ARTIFACT_CLASS_INDEX",
		"artifact_key":                 "seed-artifact-key",
		"artifact_ordinal":             0,
		"artifact_count":               8,
		"derivation_input_hash":        "seed-derivation-hash",
		"challenger_signature":         "seed-challenger-signature",
	})
	return string(bz)
}

func buildStorageProofResultJSON(challengerAcct, targetAcct, ticketID, transcriptHash, bucketType string) string {
	return buildStorageProofResultJSONWithClass(
		challengerAcct,
		targetAcct,
		ticketID,
		transcriptHash,
		bucketType,
		"STORAGE_PROOF_RESULT_CLASS_INVALID_TRANSCRIPT",
	)
}

// submitEpochReportWithProofResults submits an epoch report that includes storage proof results
// via the AutoCLI --storage-proof-results flag. Uses an empty host report (no port measurements).
func submitEpochReportWithProofResults(t *testing.T, cli *LumeradCli, fromNode string, epochID uint64, proofResultJSONs []string) string {
	t.Helper()
	args := []string{
		"tx", "audit", "submit-epoch-report",
		strconv.FormatUint(epochID, 10),
		auditHostReportJSON([]string{}),
		"--from", fromNode,
	}
	for _, pr := range proofResultJSONs {
		args = append(args, "--storage-proof-results", pr)
	}
	return cli.CustomCommand(args...)
}

type transcriptSeed struct {
	ticketID       string
	transcriptHash string
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func findAssignedProberForTarget(
	t *testing.T,
	epochID uint64,
	candidates []testNodeIdentity,
	targetAcct string,
) (audittypes.QueryAssignedTargetsResponse, testNodeIdentity) {
	t.Helper()

	var fallbackResp audittypes.QueryAssignedTargetsResponse
	var fallbackProber testNodeIdentity
	for _, candidate := range candidates {
		resp := auditQueryAssignedTargets(t, epochID, true, candidate.accAddr)
		if !containsString(resp.TargetSupernodeAccounts, targetAcct) {
			continue
		}
		if candidate.accAddr != targetAcct {
			return resp, candidate
		}
		fallbackResp = resp
		fallbackProber = candidate
	}
	if fallbackProber.accAddr != "" {
		return fallbackResp, fallbackProber
	}

	require.FailNowf(t, "no assigned prober", "no candidate assigned to target %q in epoch %d", targetAcct, epochID)
	return audittypes.QueryAssignedTargetsResponse{}, testNodeIdentity{}
}

func findAssignedProberAndTarget(
	t *testing.T,
	epochID uint64,
	candidates []testNodeIdentity,
) (audittypes.QueryAssignedTargetsResponse, testNodeIdentity, testNodeIdentity) {
	t.Helper()

	byAccount := make(map[string]testNodeIdentity, len(candidates))
	for _, candidate := range candidates {
		byAccount[candidate.accAddr] = candidate
	}

	for _, candidate := range candidates {
		resp := auditQueryAssignedTargets(t, epochID, true, candidate.accAddr)
		for _, targetAcct := range resp.TargetSupernodeAccounts {
			target, ok := byAccount[targetAcct]
			if ok && target.accAddr != candidate.accAddr {
				return resp, candidate, target
			}
		}
	}

	require.FailNowf(t, "no assigned prober/target pair", "no candidate had an assigned registered target in epoch %d", epochID)
	return audittypes.QueryAssignedTargetsResponse{}, testNodeIdentity{}, testNodeIdentity{}
}

// seedProofTranscripts seeds on-chain transcript records so that subsequent
// SubmitStorageRecheckEvidence calls can reference a valid challenged_result_transcript_hash.
//
// It queries assignments to find which node in candidates is assigned targetAcct,
// submits an epoch report with INVALID_TRANSCRIPT results from that prober, then
// returns the rechecker node (any candidate ≠ prober).
//
// For fullMode=true (FULL enforcement), exactly one seed is expected and both RECENT and OLD
// results are included to satisfy compound-coverage validation. For fullMode=false, one
// RECENT result is generated per seed.
func seedProofTranscripts(
	t *testing.T,
	cli *LumeradCli,
	epochID uint64,
	candidates []testNodeIdentity,
	targetAcct string,
	seeds []transcriptSeed,
	fullMode bool,
) testNodeIdentity {
	return seedProofTranscriptsWithClass(t, cli, epochID, candidates, targetAcct, seeds, fullMode,
		"STORAGE_PROOF_RESULT_CLASS_INVALID_TRANSCRIPT")
}

// seedProofTranscriptsWithClass is identical to seedProofTranscripts but emits seed
// proof results carrying an explicit result class. Callers that need the seed to
// count as a Class A failure under postpone predicates pass HASH_MISMATCH; callers
// that only need a recheck-eligible record can stick with the INVALID_TRANSCRIPT
// default exposed via seedProofTranscripts. Class A semantics are now failure-class
// driven (see storageTruthIsClassAFault), so an INDEX-artifact alone no longer
// promotes a non-class-A result to Class A.
func seedProofTranscriptsWithClass(
	t *testing.T,
	cli *LumeradCli,
	epochID uint64,
	candidates []testNodeIdentity,
	targetAcct string,
	seeds []transcriptSeed,
	fullMode bool,
	resultClass string,
) testNodeIdentity {
	t.Helper()

	var prober, rechecker testNodeIdentity
	proberIdx := -1
	var proberResp audittypes.QueryAssignedTargetsResponse
	for i, c := range candidates {
		resp := auditQueryAssignedTargets(t, epochID, true, c.accAddr)
		for _, a := range resp.TargetSupernodeAccounts {
			if a == targetAcct {
				prober = c
				proberIdx = i
				proberResp = resp
				break
			}
		}
		if proberIdx >= 0 {
			break
		}
	}
	require.GreaterOrEqual(t, proberIdx, 0,
		"no candidate assigned to %q in epoch %d — check challenge_target_divisor=1 in genesis", targetAcct, epochID)
	for i, c := range candidates {
		if i != proberIdx && c.accAddr != targetAcct {
			rechecker = c
			break
		}
	}
	require.NotEmpty(t, rechecker.accAddr, "no rechecker available — candidates must include a node distinct from prober and target")

	// Build port states sized to required_open_ports (chain rejects mismatched lengths).
	portStates := make([]string, len(proberResp.RequiredOpenPorts))
	for j := range portStates {
		portStates[j] = "PORT_STATE_OPEN"
	}

	// Probers must include peer observations for ALL assigned targets.
	var observations []string
	for _, tgt := range proberResp.TargetSupernodeAccounts {
		observations = append(observations, storageChallengeObservationJSON(tgt, portStates))
	}

	var proofResults []string
	for _, s := range seeds {
		proofResults = append(proofResults, buildStorageProofResultJSONWithClass(
			prober.accAddr, targetAcct, s.ticketID, s.transcriptHash,
			"STORAGE_PROOF_BUCKET_TYPE_RECENT",
			resultClass,
		))
		if fullMode {
			// FULL mode requires both RECENT and OLD results for every assigned target.
			proofResults = append(proofResults, buildStorageProofResultJSONWithClass(
				prober.accAddr, targetAcct, s.ticketID, s.transcriptHash+"-old-seed",
				"STORAGE_PROOF_BUCKET_TYPE_OLD",
				resultClass,
			))
		}
	}

	// Submit full epoch report: host report + peer observations + proof results.
	args := []string{
		"tx", "audit", "submit-epoch-report",
		strconv.FormatUint(epochID, 10),
		auditHostReportJSON(portStates),
		"--from", prober.nodeName,
		"--gas", "500000",
	}
	for _, obs := range observations {
		args = append(args, "--storage-challenge-observations", obs)
	}
	for _, pr := range proofResults {
		args = append(args, "--storage-proof-results", pr)
	}
	seedResp := cli.CustomCommand(args...)
	RequireTxSuccess(t, seedResp)
	sut.AwaitNextBlock(t)

	return rechecker
}
