//go:build integration
// +build integration

package precompiles_test

import (
	"math/big"
	"strings"
	"testing"
	"time"

	lcfg "github.com/LumeraProtocol/lumera/config"
	actionprecompile "github.com/LumeraProtocol/lumera/precompiles/action"
	supernodeprecompile "github.com/LumeraProtocol/lumera/precompiles/supernode"
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

// ---------------------------------------------------------------------------
// Supernode precompile tx-path tests
// ---------------------------------------------------------------------------

// testSupernodeRegisterTxPath verifies that the genesis validator account can
// register a supernode via the precompile's registerSupernode method using
// eth_sendRawTransaction and that the supernode appears in listSuperNodes.
func testSupernodeRegisterTxPath(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	callerBech32 := node.KeyInfo().Address

	// Derive the validator operator address from the same account bytes.
	accAddr, err := sdk.AccAddressFromBech32(callerBech32)
	if err != nil {
		t.Fatalf("parse bech32 account address: %v", err)
	}
	validatorAddr, err := sdk.Bech32ifyAddressBytes(lcfg.Bech32ValidatorAddressPrefix, accAddr.Bytes())
	if err != nil {
		t.Fatalf("derive validator address: %v", err)
	}

	// Pack registerSupernode(validatorAddress, ipAddress, supernodeAccount, p2pPort)
	input, err := supernodeprecompile.ABI.Pack(
		supernodeprecompile.RegisterSupernodeMethod,
		validatorAddr,
		"127.0.0.1",
		callerBech32,
		"4001",
	)
	if err != nil {
		t.Fatalf("pack registerSupernode input: %v", err)
	}

	txHash := sendPrecompileLegacyTx(t, node, supernodeprecompile.SupernodePrecompileAddress, input, 800_000)
	receipt := node.WaitForReceipt(t, txHash, 45*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)

	status := evmtest.MustStringField(t, receipt, "status")
	if !strings.EqualFold(status, "0x1") {
		t.Fatalf("expected successful registerSupernode tx, got status %q (%#v)", status, receipt)
	}

	// Verify the supernode exists via listSuperNodes query.
	listInput, err := supernodeprecompile.ABI.Pack(
		supernodeprecompile.ListSuperNodesMethod,
		uint64(0),  // offset
		uint64(10), // limit
	)
	if err != nil {
		t.Fatalf("pack listSuperNodes input: %v", err)
	}
	listResult := mustEthCallPrecompile(t, node, supernodeprecompile.SupernodePrecompileAddress, listInput)
	listOut, err := supernodeprecompile.ABI.Unpack(supernodeprecompile.ListSuperNodesMethod, listResult)
	if err != nil {
		t.Fatalf("unpack listSuperNodes output: %v", err)
	}
	if len(listOut) < 2 {
		t.Fatalf("expected 2 return values from listSuperNodes, got %d", len(listOut))
	}
	total, ok := listOut[1].(uint64)
	if !ok {
		t.Fatalf("unexpected total type: %#v", listOut[1])
	}
	if total == 0 {
		t.Fatalf("expected at least 1 supernode after registration, got 0")
	}

	// Verify receipt contains logs (SupernodeRegistered event).
	logs, ok := receipt["logs"].([]any)
	if !ok || len(logs) == 0 {
		t.Logf("WARNING: no logs in receipt — event emission may not be working")
	}
}

// testSupernodeReportMetricsTxPath verifies that the registered supernode
// account can report metrics via the precompile. Depends on the supernode
// having been registered by a prior test in the suite.
func testSupernodeReportMetricsTxPath(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	callerBech32 := node.KeyInfo().Address
	accAddr, err := sdk.AccAddressFromBech32(callerBech32)
	if err != nil {
		t.Fatalf("parse bech32 account address: %v", err)
	}
	validatorAddr, err := sdk.Bech32ifyAddressBytes(lcfg.Bech32ValidatorAddressPrefix, accAddr.Bytes())
	if err != nil {
		t.Fatalf("derive validator address: %v", err)
	}

	// Build the MetricsReport struct for the ABI.
	type MetricsReport struct {
		VersionMajor     uint32 `abi:"versionMajor"`
		VersionMinor     uint32 `abi:"versionMinor"`
		VersionPatch     uint32 `abi:"versionPatch"`
		CpuCoresTotal    uint32 `abi:"cpuCoresTotal"`
		CpuUsagePercent  uint64 `abi:"cpuUsagePercent"`
		MemTotalGb       uint64 `abi:"memTotalGb"`
		MemUsagePercent  uint64 `abi:"memUsagePercent"`
		MemFreeGb        uint64 `abi:"memFreeGb"`
		DiskTotalGb      uint64 `abi:"diskTotalGb"`
		DiskUsagePercent uint64 `abi:"diskUsagePercent"`
		DiskFreeGb       uint64 `abi:"diskFreeGb"`
		UptimeSeconds    uint64 `abi:"uptimeSeconds"`
		PeersCount       uint32 `abi:"peersCount"`
	}

	metrics := MetricsReport{
		VersionMajor:     1,
		VersionMinor:     0,
		VersionPatch:     0,
		CpuCoresTotal:    8,
		CpuUsagePercent:  25,
		MemTotalGb:       32,
		MemUsagePercent:  40,
		MemFreeGb:        19,
		DiskTotalGb:      500,
		DiskUsagePercent: 30,
		DiskFreeGb:       350,
		UptimeSeconds:    86400,
		PeersCount:       5,
	}

	// reportMetrics(validatorAddress, supernodeAccount, metrics)
	// Note: supernodeAccount arg is now ignored by the precompile (derived from caller).
	input, err := supernodeprecompile.ABI.Pack(
		supernodeprecompile.ReportMetricsMethod,
		validatorAddr,
		callerBech32, // ignored but required by ABI
		metrics,
	)
	if err != nil {
		t.Fatalf("pack reportMetrics input: %v", err)
	}

	txHash := sendPrecompileLegacyTx(t, node, supernodeprecompile.SupernodePrecompileAddress, input, 800_000)
	receipt := node.WaitForReceipt(t, txHash, 45*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)

	status := evmtest.MustStringField(t, receipt, "status")
	if !strings.EqualFold(status, "0x1") {
		t.Fatalf("expected successful reportMetrics tx, got status %q (%#v)", status, receipt)
	}
}

// testSupernodeReportMetricsTxPathFailsForWrongCaller verifies that an
// account that is NOT the registered supernode account cannot report metrics.
// This validates the auth fix (Finding #1): contract.Caller() must match the
// on-chain supernode account.
func testSupernodeReportMetricsTxPathFailsForWrongCaller(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	// Use a freshly generated key that is NOT the registered supernode account.
	wrongKey, wrongAddr := testaccounts.MustGenerateEthKey(t)

	// Fund the wrong account so it can send a tx.
	fundTx(t, node, wrongAddr, big.NewInt(1_000_000_000_000))

	callerBech32 := node.KeyInfo().Address
	accAddr, err := sdk.AccAddressFromBech32(callerBech32)
	if err != nil {
		t.Fatalf("parse bech32 account address: %v", err)
	}
	validatorAddr, err := sdk.Bech32ifyAddressBytes(lcfg.Bech32ValidatorAddressPrefix, accAddr.Bytes())
	if err != nil {
		t.Fatalf("derive validator address: %v", err)
	}

	type MetricsReport struct {
		VersionMajor     uint32 `abi:"versionMajor"`
		VersionMinor     uint32 `abi:"versionMinor"`
		VersionPatch     uint32 `abi:"versionPatch"`
		CpuCoresTotal    uint32 `abi:"cpuCoresTotal"`
		CpuUsagePercent  uint64 `abi:"cpuUsagePercent"`
		MemTotalGb       uint64 `abi:"memTotalGb"`
		MemUsagePercent  uint64 `abi:"memUsagePercent"`
		MemFreeGb        uint64 `abi:"memFreeGb"`
		DiskTotalGb      uint64 `abi:"diskTotalGb"`
		DiskUsagePercent uint64 `abi:"diskUsagePercent"`
		DiskFreeGb       uint64 `abi:"diskFreeGb"`
		UptimeSeconds    uint64 `abi:"uptimeSeconds"`
		PeersCount       uint32 `abi:"peersCount"`
	}

	metrics := MetricsReport{
		VersionMajor: 1, CpuCoresTotal: 4, MemTotalGb: 16,
		DiskTotalGb: 200, DiskFreeGb: 100, UptimeSeconds: 3600, PeersCount: 2,
	}

	input, err := supernodeprecompile.ABI.Pack(
		supernodeprecompile.ReportMetricsMethod,
		validatorAddr,
		callerBech32, // tries to impersonate the real supernode account
		metrics,
	)
	if err != nil {
		t.Fatalf("pack reportMetrics input: %v", err)
	}

	// Send from the wrong account — should fail because contract.Caller()
	// doesn't match the on-chain supernode account.
	nonce := node.MustGetPendingNonceWithRetry(t, wrongAddr.Hex(), 20*time.Second)
	gasPrice := node.MustGetGasPriceWithRetry(t, 20*time.Second)
	toAddr := common.HexToAddress(supernodeprecompile.SupernodePrecompileAddress)

	txHash := node.SendLegacyTxWithParams(t, evmtest.LegacyTxParams{
		PrivateKey: wrongKey,
		Nonce:      nonce,
		To:         &toAddr,
		Value:      big.NewInt(0),
		Gas:        800_000,
		GasPrice:   gasPrice,
		Data:       input,
	})

	receipt := node.WaitForReceipt(t, txHash, 45*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)

	status := evmtest.MustStringField(t, receipt, "status")
	if !strings.EqualFold(status, "0x0") {
		t.Fatalf("expected FAILED reportMetrics from wrong caller, got status %q", status)
	}
}

// ---------------------------------------------------------------------------
// Action precompile tx-path tests
// ---------------------------------------------------------------------------

// testActionRequestCascadeTxPathFailsWithBadSignature verifies that
// requestCascade rejects a request with an invalid signature format.
// The keeper expects "Base64(rq_ids).creator_signature" but we send garbage.
func testActionRequestCascadeTxPathFailsWithBadSignature(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	input, err := actionprecompile.ABI.Pack(
		actionprecompile.RequestCascadeMethod,
		"deadbeef1234567890abcdef",               // dataHash
		"test-file.dat",                           // fileName
		uint64(3),                                 // rqIdsIc
		"not-a-valid-dot-delimited-signature",     // signatures (bad format)
		big.NewInt(100_000),                       // price
		int64(time.Now().Add(1*time.Hour).Unix()), // expirationTime
		uint64(100),                               // fileSizeKbs
	)
	if err != nil {
		t.Fatalf("pack requestCascade input: %v", err)
	}

	txHash := sendPrecompileLegacyTx(t, node, actionprecompile.ActionPrecompileAddress, input, 800_000)
	receipt := node.WaitForReceipt(t, txHash, 45*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)

	status := evmtest.MustStringField(t, receipt, "status")
	if !strings.EqualFold(status, "0x0") {
		t.Fatalf("expected FAILED requestCascade with bad signature, got status %q", status)
	}
}

// testActionApproveActionTxPathFailsForNonExistent verifies that
// approveAction correctly reverts when called for a non-existent action ID.
func testActionApproveActionTxPathFailsForNonExistent(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	input, err := actionprecompile.ABI.Pack(
		actionprecompile.ApproveActionMethod,
		"non-existent-action-id-12345",
	)
	if err != nil {
		t.Fatalf("pack approveAction input: %v", err)
	}

	txHash := sendPrecompileLegacyTx(t, node, actionprecompile.ActionPrecompileAddress, input, 500_000)
	receipt := node.WaitForReceipt(t, txHash, 45*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)

	status := evmtest.MustStringField(t, receipt, "status")
	if !strings.EqualFold(status, "0x0") {
		t.Fatalf("expected FAILED approveAction for non-existent action, got status %q", status)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// fundTx sends ulume from the genesis account to a target address so it can
// pay gas for subsequent transactions.
func fundTx(t *testing.T, node *evmtest.Node, to common.Address, amount *big.Int) {
	t.Helper()

	keyInfo := node.KeyInfo()
	fromAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, keyInfo)
	privateKey := evmtest.MustDerivePrivateKey(t, keyInfo.Mnemonic)
	nonce := node.MustGetPendingNonceWithRetry(t, fromAddr.Hex(), 20*time.Second)
	gasPrice := node.MustGetGasPriceWithRetry(t, 20*time.Second)

	txHash := node.SendLegacyTxWithParams(t, evmtest.LegacyTxParams{
		PrivateKey: privateKey,
		Nonce:      nonce,
		To:         &to,
		Value:      amount,
		Gas:        21_000,
		GasPrice:   gasPrice,
		Data:       nil,
	})
	receipt := node.WaitForReceipt(t, txHash, 30*time.Second)
	if status := evmtest.MustStringField(t, receipt, "status"); !strings.EqualFold(status, "0x1") {
		t.Fatalf("fund tx failed: status=%s", status)
	}
}
