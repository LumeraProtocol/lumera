//go:build integration
// +build integration

package evmtest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	lcfg "github.com/LumeraProtocol/lumera/config"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
)

// nodePorts defines the set of ports used by a test EVM node instance. Ports are dynamically allocated to avoid conflicts across parallel test runs.
type nodePorts struct {
	JSONRPC   int // JSON-RPC for EVM (default enabled on 8545)
	JSONWSRPC int // WebSocket JSON-RPC for EVM (default enabled on 8546)
	CometRPC  int // CometBFT RPC (default 26657)
	GRPC      int // gRPC (default 9090)
	GRPCWeb   int // gRPC-Web (default 9091)
	API       int // Cosmos REST API (default 1317)
	ABCI      int // ABCI (default 26658)
	P2P       int // P2P (default 26656)
}

type evmNode struct {
	t        *testing.T               // Parent test instance for fail-fast helpers.
	repoRoot string                   // Repository root used to run build/CLI commands.
	binPath  string                   // Path to the ephemeral lumerad binary.
	homeDir  string                   // Isolated node home directory.
	chainID  string                   // Chain ID used for this node fixture.
	keyInfo  testaccounts.TestKeyInfo // Validator key generated during setup.

	haltHeight     int      // Halt height passed to `lumerad start`.
	extraStartArgs []string // Caller-appended args, preserved across port re-reservation.

	rpcURL      string   // HTTP JSON-RPC endpoint.
	wsURL       string   // WebSocket JSON-RPC endpoint.
	apiURL      string   // Cosmos REST API endpoint.
	cometRPCURL string   // Comet RPC endpoint for Cosmos CLI commands.
	startArgs   []string // Cached `lumerad start` arguments (base args + extraStartArgs).

	cancel context.CancelFunc // Process cancellation hook.
	cmd    *exec.Cmd          // Running node process handle.
	waitCh <-chan error       // Async process wait channel.
	output *bytes.Buffer      // Combined stdout/stderr capture.
}

var (
	sharedLumeraBuildOnce sync.Once
	sharedLumeraBuildPath string
	sharedLumeraBuildErr  error
)

// newEVMNode creates an isolated node fixture (fresh binary, home, genesis and ports).
func newEVMNode(t *testing.T, chainID string, haltHeight int) *evmNode {
	t.Helper()

	repoRoot := mustFindRepoRoot(t)
	binPath := mustBuildLumeraBinary(t, repoRoot)
	homeDir := filepath.Join(t.TempDir(), "home")
	keyInfo := setupGenesisWithGentx(t, repoRoot, binPath, homeDir, chainID)
	ports := reserveNodePorts(t)

	return &evmNode{
		t:           t,
		repoRoot:    repoRoot,
		binPath:     binPath,
		homeDir:     homeDir,
		chainID:     chainID,
		keyInfo:     keyInfo,
		haltHeight:  haltHeight,
		rpcURL:      fmt.Sprintf("http://127.0.0.1:%d", ports.JSONRPC),
		wsURL:       fmt.Sprintf("ws://127.0.0.1:%d", ports.JSONWSRPC),
		apiURL:      fmt.Sprintf("http://127.0.0.1:%d", ports.API),
		cometRPCURL: fmt.Sprintf("tcp://127.0.0.1:%d", ports.CometRPC),
		startArgs:   buildStartArgs(homeDir, ports, haltHeight),
	}
}

// refreshPorts re-reserves a fresh set of node ports and rebuilds startArgs,
// preserving any caller-appended args. Used to retry startup after a transient
// "address already in use" race (a previously-reserved port being grabbed by
// another process, or lingering in TIME_WAIT from a prior node's teardown).
func (n *evmNode) refreshPorts() {
	n.t.Helper()
	ports := reserveNodePorts(n.t)
	n.rpcURL = fmt.Sprintf("http://127.0.0.1:%d", ports.JSONRPC)
	n.wsURL = fmt.Sprintf("ws://127.0.0.1:%d", ports.JSONWSRPC)
	n.apiURL = fmt.Sprintf("http://127.0.0.1:%d", ports.API)
	n.cometRPCURL = fmt.Sprintf("tcp://127.0.0.1:%d", ports.CometRPC)
	n.startArgs = append(buildStartArgs(n.homeDir, ports, n.haltHeight), n.extraStartArgs...)
}

// Start launches `lumerad start` with precomputed args and captures logs.
func (n *evmNode) Start() {
	n.t.Helper()
	if n.cmd != nil {
		n.t.Fatal("node is already running")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd, waitCh, output := startProcess(n.t, ctx, n.repoRoot, n.binPath, n.startArgs...)
	n.cancel = cancel
	n.cmd = cmd
	n.waitCh = waitCh
	n.output = output
}

// StartAndWaitRPC starts the node and blocks until JSON-RPC responds. If the
// node exits during startup because a reserved port was already in use (a race
// against another process or a prior node still releasing its sockets), it
// re-reserves fresh ports and retries a few times before failing.
func (n *evmNode) StartAndWaitRPC() {
	n.t.Helper()

	const maxAttempts = 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		n.Start()
		err := waitForJSONRPCResult(n.rpcURL, n.waitCh, n.output)
		if err == nil {
			return
		}

		out := n.output.String()
		if attempt < maxAttempts && isAddrInUse(out) {
			n.t.Logf("node startup attempt %d/%d hit a port conflict, retrying with fresh ports: %v",
				attempt, maxAttempts, err)
			n.Stop()
			n.refreshPorts()
			continue
		}

		n.t.Fatalf("json-rpc server did not become ready: %v\n%s", err, out)
	}
}

// Stop gracefully terminates the running node process.
func (n *evmNode) Stop() {
	n.t.Helper()
	if n.cancel == nil {
		return
	}
	stopProcess(n.t, n.cancel, n.cmd, n.waitCh)
	n.cancel = nil
	n.cmd = nil
	n.waitCh = nil
	n.output = nil
}

// RestartAndWaitRPC performs stop+start and waits for RPC readiness.
func (n *evmNode) RestartAndWaitRPC() {
	n.t.Helper()
	n.Stop()
	n.StartAndWaitRPC()
}

// OutputString returns aggregated stdout/stderr from the latest node run.
func (n *evmNode) OutputString() string {
	n.t.Helper()
	if n.output == nil {
		return ""
	}
	return n.output.String()
}

func (n *evmNode) RPCURL() string { return n.rpcURL }

func (n *evmNode) WSURL() string { return n.wsURL }

func (n *evmNode) APIURL() string { return n.apiURL }

func (n *evmNode) APIListenAddress() string { return strings.Replace(n.apiURL, "http://", "tcp://", 1) }

func (n *evmNode) CometRPCURL() string { return n.cometRPCURL }

func (n *evmNode) HomeDir() string { return n.homeDir }

func (n *evmNode) ChainID() string { return n.chainID }

func (n *evmNode) KeyInfo() testaccounts.TestKeyInfo { return n.keyInfo }

func (n *evmNode) WaitCh() <-chan error { return n.waitCh }

func (n *evmNode) OutputBuffer() *bytes.Buffer { return n.output }

func (n *evmNode) RepoRoot() string { return n.repoRoot }

func (n *evmNode) BinPath() string { return n.binPath }

func (n *evmNode) StartArgs() []string {
	return append([]string(nil), n.startArgs...)
}

func (n *evmNode) AppendStartArgs(args ...string) {
	n.extraStartArgs = append(n.extraStartArgs, args...)
	n.startArgs = append(n.startArgs, args...)
}

// --- RPC convenience methods ------------------------------------------------
// Each method delegates to the corresponding private function so the caller
// does not have to unpack rpcURL / waitCh / output manually.

func (n *evmNode) WaitForReceipt(t *testing.T, txHash string, timeout time.Duration) map[string]any {
	t.Helper()
	return waitForReceipt(t, n.rpcURL, txHash, n.waitCh, n.output, timeout)
}

func (n *evmNode) WaitForTransactionByHash(t *testing.T, txHash string, timeout time.Duration) map[string]any {
	t.Helper()
	return waitForTransactionByHash(t, n.rpcURL, txHash, n.waitCh, n.output, timeout)
}

func (n *evmNode) WaitForBlockNumberAtLeast(t *testing.T, minBlock uint64, timeout time.Duration) {
	t.Helper()
	waitForBlockNumberAtLeast(t, n.rpcURL, minBlock, timeout)
}

func (n *evmNode) MustGetBlockNumber(t *testing.T) uint64 {
	t.Helper()
	return mustGetBlockNumber(t, n.rpcURL)
}

func (n *evmNode) MustGetGasPriceWithRetry(t *testing.T, timeout time.Duration) *big.Int {
	t.Helper()
	return mustGetGasPriceWithRetry(t, n.rpcURL, timeout)
}

func (n *evmNode) MustGetPendingNonceWithRetry(t *testing.T, fromHex string, timeout time.Duration) uint64 {
	t.Helper()
	return mustGetPendingNonceWithRetry(t, n.rpcURL, fromHex, timeout)
}

func (n *evmNode) SendOneLegacyTx(t *testing.T) string {
	t.Helper()
	return sendOneLegacyTx(t, n.rpcURL, n.keyInfo)
}

func (n *evmNode) SendLogEmitterCreationTx(t *testing.T, topicHex string) string {
	t.Helper()
	return sendLogEmitterCreationTx(t, n.rpcURL, n.keyInfo, topicHex)
}

func (n *evmNode) SendLegacyTxWithParams(t *testing.T, p legacyTxParams) string {
	t.Helper()
	return sendLegacyTxWithParams(t, n.rpcURL, p)
}

func (n *evmNode) SendDynamicFeeTxWithParams(t *testing.T, p dynamicFeeTxParams) string {
	t.Helper()
	return sendDynamicFeeTxWithParams(t, n.rpcURL, p)
}

func (n *evmNode) MustGetBlock(t *testing.T, method string, params []any) map[string]any {
	t.Helper()
	return mustGetBlock(t, n.rpcURL, method, params)
}

func (n *evmNode) MustGetLogs(t *testing.T, filter map[string]any) []map[string]any {
	t.Helper()
	return mustGetLogs(t, n.rpcURL, filter)
}

func (n *evmNode) MustJSONRPC(t *testing.T, method string, params []any, out any) {
	t.Helper()
	mustJSONRPC(t, n.rpcURL, method, params, out)
}

// reserveNodePorts allocates a full set of free local ports for one node.
func reserveNodePorts(t *testing.T) nodePorts {
	t.Helper()
	return nodePorts{
		JSONRPC:   freePort(t),
		JSONWSRPC: freePort(t),
		CometRPC:  freePort(t),
		GRPC:      freePort(t),
		GRPCWeb:   freePort(t),
		API:       freePort(t),
		ABCI:      freePort(t),
		P2P:       freePort(t),
	}
}

// buildStartArgs builds deterministic CLI args for an integration node run.
func buildStartArgs(homeDir string, ports nodePorts, haltHeight int) []string {
	return []string{
		"start",
		"--home", homeDir,
		"--minimum-gas-prices", "0" + lcfg.ChainDenom,
		"--halt-height", strconv.Itoa(haltHeight),
		"--rpc.laddr", fmt.Sprintf("tcp://127.0.0.1:%d", ports.CometRPC),
		"--grpc.enable=false",
		"--grpc-web.enable=false",
		"--grpc.address", fmt.Sprintf("127.0.0.1:%d", ports.GRPC),
		"--grpc-web.address", fmt.Sprintf("127.0.0.1:%d", ports.GRPCWeb),
		"--json-rpc.address", fmt.Sprintf("127.0.0.1:%d", ports.JSONRPC),
		"--json-rpc.ws-address", fmt.Sprintf("127.0.0.1:%d", ports.JSONWSRPC),
		"--address", fmt.Sprintf("tcp://127.0.0.1:%d", ports.ABCI),
		"--p2p.laddr", fmt.Sprintf("tcp://127.0.0.1:%d", ports.P2P),
		"--rpc.pprof_laddr", "",
		"--log_no_color",
	}
}

// mustBuildLumeraBinary compiles the local `lumerad` binary for test execution.
func mustBuildLumeraBinary(t *testing.T, repoRoot string) string {
	t.Helper()

	sharedLumeraBuildOnce.Do(func() {
		buildDir, err := os.MkdirTemp("", "lumera-evmtest-bin-*")
		if err != nil {
			sharedLumeraBuildErr = fmt.Errorf("create temp dir for shared lumerad build: %w", err)
			return
		}

		binPath := filepath.Join(buildDir, "lumerad")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		out, err := run(ctx, repoRoot, "go", "build", "-o", binPath, "./cmd/lumera")
		if err != nil {
			sharedLumeraBuildErr = fmt.Errorf("build shared lumerad binary: %w\n%s", err, out)
			return
		}

		sharedLumeraBuildPath = binPath
	})

	if sharedLumeraBuildErr != nil {
		t.Fatalf("%v", sharedLumeraBuildErr)
	}
	if strings.TrimSpace(sharedLumeraBuildPath) == "" {
		t.Fatal("shared lumerad binary path is empty after successful build")
	}

	return sharedLumeraBuildPath
}

// setupGenesisWithGentx initializes home, validator key, genesis account and gentx.
func setupGenesisWithGentx(t *testing.T, repoRoot, binPath, homeDir, chainID string) testaccounts.TestKeyInfo {
	t.Helper()

	const setupCmdTimeout = 5 * time.Minute
	keyName := "validator"

	mustRun(t, repoRoot, setupCmdTimeout, binPath,
		"init", "smoke-node",
		"--chain-id", chainID,
		"--home", homeDir,
		"--log_no_color",
	)

	appTomlPath := filepath.Join(homeDir, "config", "app.toml")
	appToml, err := os.ReadFile(appTomlPath)
	if err != nil {
		t.Fatalf("read app.toml: %v", err)
	}
	appTomlStr := string(appToml)
	if !strings.Contains(appTomlStr, "[json-rpc]") ||
		!strings.Contains(appTomlStr, "enable = true") ||
		!strings.Contains(appTomlStr, "enable-indexer = true") {
		t.Fatalf("json-rpc defaults not written to app.toml:\n%s", appTomlStr)
	}
	if !strings.Contains(appTomlStr, "[mempool]") ||
		!strings.Contains(appTomlStr, "max-txs = 10000") {
		t.Fatalf("app-side mempool defaults not written to app.toml:\n%s", appTomlStr)
	}

	keysAddOut := mustRun(t, repoRoot, setupCmdTimeout, binPath,
		"keys", "add", keyName,
		"--home", homeDir,
		"--keyring-backend", "test",
		"--output", "json",
		"--log_no_color",
	)

	var keyInfo testaccounts.TestKeyInfo
	if err := json.Unmarshal([]byte(keysAddOut), &keyInfo); err != nil {
		t.Fatalf("failed to decode keys add output: %v\n%s", err, keysAddOut)
	}
	testaccounts.MustNormalizeAndValidateTestKeyInfo(t, &keyInfo)

	mustRun(t, repoRoot, setupCmdTimeout, binPath,
		"genesis", "add-genesis-account", keyInfo.Address, "1000000000000000"+lcfg.ChainDenom,
		"--home", homeDir,
		"--log_no_color",
	)

	mustRun(t, repoRoot, setupCmdTimeout, binPath,
		"genesis", "gentx", keyName, "900000000000"+lcfg.ChainDenom,
		"--chain-id", chainID,
		"--home", homeDir,
		"--keyring-backend", "test",
		"--fees", "100"+lcfg.ChainDenom,
		"--log_no_color",
	)

	mustRun(t, repoRoot, setupCmdTimeout, binPath,
		"genesis", "collect-gentxs",
		"--home", homeDir,
		"--log_no_color",
	)

	return keyInfo
}

// setIndexerEnabledInAppToml toggles the EVM JSON-RPC indexer in app.toml.
func setIndexerEnabledInAppToml(t *testing.T, homeDir string, enabled bool) {
	t.Helper()

	appTomlPath := filepath.Join(homeDir, "config", "app.toml")
	appToml, err := os.ReadFile(appTomlPath)
	if err != nil {
		t.Fatalf("read app.toml: %v", err)
	}

	appTomlStr := string(appToml)
	target := fmt.Sprintf("enable-indexer = %t", enabled)
	updated := strings.Replace(appTomlStr, "enable-indexer = true", target, 1)
	updated = strings.Replace(updated, "enable-indexer = false", target, 1)
	if updated == appTomlStr {
		t.Fatalf("failed to update enable-indexer in app.toml:\n%s", appTomlStr)
	}

	if err := os.WriteFile(appTomlPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("write app.toml: %v", err)
	}
}

// setEVMMempoolPriceBumpInAppToml sets [evm.mempool].price-bump in app.toml.
func setEVMMempoolPriceBumpInAppToml(t *testing.T, homeDir string, priceBump uint64) {
	t.Helper()

	appTomlPath := filepath.Join(homeDir, "config", "app.toml")
	appToml, err := os.ReadFile(appTomlPath)
	if err != nil {
		t.Fatalf("read app.toml: %v", err)
	}

	appTomlStr := string(appToml)
	target := fmt.Sprintf("price-bump = %d", priceBump)
	re := regexp.MustCompile(`(?m)^price-bump = [0-9]+$`)
	updated := re.ReplaceAllString(appTomlStr, target)
	if updated == appTomlStr {
		t.Fatalf("failed to update price-bump in app.toml:\n%s", appTomlStr)
	}

	if err := os.WriteFile(appTomlPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("write app.toml: %v", err)
	}
}

// setMempoolMaxTxsInAppToml sets [mempool].max-txs in app.toml.
func setMempoolMaxTxsInAppToml(t *testing.T, homeDir string, maxTxs int) {
	t.Helper()

	appTomlPath := filepath.Join(homeDir, "config", "app.toml")
	appToml, err := os.ReadFile(appTomlPath)
	if err != nil {
		t.Fatalf("read app.toml: %v", err)
	}

	appTomlStr := string(appToml)
	target := fmt.Sprintf("max-txs = %d", maxTxs)
	re := regexp.MustCompile(`(?m)^max-txs = [0-9]+$`)
	updated := re.ReplaceAllString(appTomlStr, target)
	if updated == appTomlStr {
		t.Fatalf("failed to update max-txs in app.toml:\n%s", appTomlStr)
	}

	if err := os.WriteFile(appTomlPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("write app.toml: %v", err)
	}
}

// writeLegacyPreEVMAppToml replaces app.toml with the minimal shape an
// upgraded pre-EVM home can have: normal SDK sections, but no [evm],
// [evm.mempool], [json-rpc], [tls], or [lumera.*] sections.
func writeLegacyPreEVMAppToml(t *testing.T, homeDir string, maxTxs int) {
	t.Helper()

	appTomlPath := filepath.Join(homeDir, "config", "app.toml")
	legacyToml := fmt.Sprintf(`
[api]
enable = true
address = "tcp://127.0.0.1:1317"

[grpc]
enable = false

[mempool]
max-txs = %d
`, maxTxs)

	if err := os.WriteFile(appTomlPath, []byte(legacyToml), 0o644); err != nil {
		t.Fatalf("write legacy app.toml: %v", err)
	}
}

// setCometMempoolSize sets `size` under the `[mempool]` section in config.toml.
// This controls how many txs CometBFT accepts into its mempool before rejecting.
func setCometMempoolSize(t *testing.T, homeDir string, size int) {
	t.Helper()

	configTomlPath := filepath.Join(homeDir, "config", "config.toml")
	configToml, err := os.ReadFile(configTomlPath)
	if err != nil {
		t.Fatalf("read config.toml: %v", err)
	}

	configTomlStr := string(configToml)
	target := fmt.Sprintf("size = %d", size)
	// Match `size = <number>` only under [mempool] section.
	// CometBFT config.toml has `size` under [mempool] as the mempool capacity.
	re := regexp.MustCompile(`(?m)(^\[mempool\].*\n(?:.*\n)*?)^size = [0-9]+`)
	updated := re.ReplaceAllString(configTomlStr, "${1}"+target)
	if updated == configTomlStr {
		t.Fatalf("failed to update mempool size in config.toml")
	}

	if err := os.WriteFile(configTomlPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}
}

// enablePrometheusMetrics enables the Cosmos SDK telemetry sink and API server
// so that /metrics is available at the given apiAddress. The caller must
// allocate a free port and pass it as "tcp://127.0.0.1:<port>".
func enablePrometheusMetrics(t *testing.T, homeDir string, apiAddress string) {
	t.Helper()

	s := enableAPIInAppToml(t, homeDir, apiAddress)

	// Enable telemetry (section-aware to avoid matching other "enabled" keys).
	s = regexp.MustCompile(`(?m)(^\[telemetry\]\n(?:[^\[].*\n)*?)^enabled = (true|false)`).
		ReplaceAllString(s, "${1}enabled = true")
	s = regexp.MustCompile(`(?m)^prometheus-retention-time = [0-9]+`).
		ReplaceAllString(s, "prometheus-retention-time = 60")

	writeAppToml(t, homeDir, s)
}

func enableAPIInAppToml(t *testing.T, homeDir string, apiAddress string) string {
	t.Helper()

	appTomlPath := filepath.Join(homeDir, "config", "app.toml")
	appToml, err := os.ReadFile(appTomlPath)
	if err != nil {
		t.Fatalf("read app.toml: %v", err)
	}
	s := string(appToml)

	updated := regexp.MustCompile(`(?m)(^\[api\]\n(?:[^\[].*\n)*?)^enable = (true|false)`).
		ReplaceAllString(s, "${1}enable = true")
	updated = regexp.MustCompile(`(?m)(^\[api\]\n(?:[^\[].*\n)*?)^address = "[^"]+"`).
		ReplaceAllString(updated, fmt.Sprintf(`${1}address = "%s"`, apiAddress))

	if updated == s {
		t.Fatalf("failed to enable API in app.toml")
	}

	return updated
}

func writeAppToml(t *testing.T, homeDir string, content string) {
	t.Helper()

	appTomlPath := filepath.Join(homeDir, "config", "app.toml")
	if err := os.WriteFile(appTomlPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write app.toml: %v", err)
	}
}

// setCometTxIndexer sets `[tx_index].indexer` in Comet config.toml.
func setCometTxIndexer(t *testing.T, homeDir, indexer string) {
	t.Helper()

	configTomlPath := filepath.Join(homeDir, "config", "config.toml")
	configToml, err := os.ReadFile(configTomlPath)
	if err != nil {
		t.Fatalf("read config.toml: %v", err)
	}

	configTomlStr := string(configToml)
	target := fmt.Sprintf("indexer = %q", indexer)
	updated := strings.Replace(configTomlStr, `indexer = "kv"`, target, 1)
	updated = strings.Replace(updated, `indexer = "null"`, target, 1)
	updated = strings.Replace(updated, `indexer = "psql"`, target, 1)
	if updated == configTomlStr {
		t.Fatalf("failed to update [tx_index].indexer in config.toml:\n%s", configTomlStr)
	}

	if err := os.WriteFile(configTomlPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}
}
