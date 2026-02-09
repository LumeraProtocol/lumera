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

func awaitAtLeastHeight(t *testing.T, height int64) {
	t.Helper()
	if sut.currentHeight >= height {
		return
	}
	sut.AwaitBlockHeight(t, height)
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
