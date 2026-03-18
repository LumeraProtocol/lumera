//go:build integration
// +build integration

package contracts_test

import (
	"math/big"
	"strings"
	"testing"
	"time"

	actionprecompile "github.com/LumeraProtocol/lumera/precompiles/action"
	supernodeprecompile "github.com/LumeraProtocol/lumera/precompiles/supernode"
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/vm"
	evmprogram "github.com/ethereum/go-ethereum/core/vm/program"
)

// ---------------------------------------------------------------------------
// Bytecode generators
// ---------------------------------------------------------------------------

// staticCallProxyCreationCode returns init code for a minimal proxy contract
// that forwards any calldata to a hardcoded precompile address via STATICCALL
// and returns the precompile's output unchanged.
//
// This is the simplest "contract calls precompile" pattern — it proves that
// deployed Solidity contracts can access Lumera custom precompiles (action at
// 0x0901, supernode at 0x0902) through standard EVM cross-contract calls.
//
// Runtime bytecode logic:
//  1. CALLDATACOPY — copy incoming calldata to memory[0:]
//  2. STATICCALL  — forward calldata to the hardcoded precompile address
//  3. RETURNDATACOPY + RETURN — copy and return the precompile's response
func staticCallProxyCreationCode(precompileAddr int) []byte {
	runtime := evmprogram.New().
		// 1. Copy calldata to memory[0:calldatasize]
		Op(vm.CALLDATASIZE). // [cdSize]
		Push(0).             // [cdSize, 0]      offset in calldata
		Push(0).             // [cdSize, 0, 0]   dest offset in memory
		Op(vm.CALLDATACOPY). // mem[0:cdSize] = calldata; stack: []
		// 2. STATICCALL(gas, addr, argsOff, argsLen, retOff, retLen)
		Push(0).              // retLen = 0 (use RETURNDATASIZE after)
		Push(0).              // retOff = 0
		Op(vm.CALLDATASIZE).  // argsLen = calldatasize
		Push(0).              // argsOff = 0
		Push(precompileAddr). // target precompile address
		Op(vm.GAS).           // forward all remaining gas
		Op(vm.STATICCALL).    // → success flag
		Op(vm.POP).           // discard success flag
		// 3. Copy return data to memory and return it
		Op(vm.RETURNDATASIZE). // [retSize]
		Push(0).               // [retSize, 0]
		Push(0).               // [retSize, 0, 0]
		Op(vm.RETURNDATACOPY). // mem[0:retSize] = return data
		Op(vm.RETURNDATASIZE). // [retSize]
		Push(0).               // [retSize, 0]
		Op(vm.RETURN).         // return mem[0:retSize]
		Bytes()

	return evmprogram.New().
		ReturnViaCodeCopy(runtime).
		Bytes()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// deployStaticCallProxy deploys a STATICCALL proxy contract targeting the
// given precompile address and returns the deployed contract address hex.
func deployStaticCallProxy(t *testing.T, node *evmtest.Node, precompileAddr int) string {
	t.Helper()
	deployHash := sendContractCreationTx(t, node, staticCallProxyCreationCode(precompileAddr))
	receipt := node.WaitForReceipt(t, deployHash, 45*time.Second)
	assertReceiptBasics(t, receipt)
	addr := evmtest.MustStringField(t, receipt, "contractAddress")
	if strings.EqualFold(addr, "0x0000000000000000000000000000000000000000") {
		t.Fatal("proxy deployment returned zero address")
	}
	return addr
}

// ethCallProxy sends an eth_call to the proxy contract with the given
// ABI-encoded calldata and returns the raw result bytes.
func ethCallProxy(t *testing.T, node *evmtest.Node, proxyAddr string, calldata []byte) []byte {
	t.Helper()
	var resultHex string
	node.MustJSONRPC(t, "eth_call", []any{
		map[string]any{
			"to":   proxyAddr,
			"data": hexutil.Encode(calldata),
		},
		"latest",
	}, &resultHex)
	if strings.TrimSpace(resultHex) == "" || resultHex == "0x" {
		t.Fatalf("proxy eth_call returned empty result for %s", proxyAddr)
	}
	bz, err := hexutil.Decode(resultHex)
	if err != nil {
		t.Fatalf("decode proxy result %q: %v", resultHex, err)
	}
	return bz
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// testContractProxiesActionGetParams deploys a proxy contract targeting the
// action precompile (0x0901). The proxy forwards a getParams() call via
// STATICCALL, proving that deployed contracts can query Lumera precompiles.
func testContractProxiesActionGetParams(t *testing.T, node *evmtest.Node) {
	t.Helper()

	proxyAddr := deployStaticCallProxy(t, node, 0x0901)

	// ABI-encode getParams() calldata
	calldata, err := actionprecompile.ABI.Pack(actionprecompile.GetParamsMethod)
	if err != nil {
		t.Fatalf("pack getParams: %v", err)
	}

	// Call proxy → STATICCALL → action precompile
	resultBz := ethCallProxy(t, node, proxyAddr, calldata)

	// Decode and validate the 7-tuple response
	out, err := actionprecompile.ABI.Unpack(actionprecompile.GetParamsMethod, resultBz)
	if err != nil {
		t.Fatalf("unpack getParams: %v", err)
	}
	if len(out) != 7 {
		t.Fatalf("expected 7 return values, got %d", len(out))
	}

	// baseActionFee (uint256) should be positive
	baseFee, ok := out[0].(*big.Int)
	if !ok || baseFee == nil || baseFee.Sign() <= 0 {
		t.Fatalf("baseActionFee should be > 0, got %v", out[0])
	}

	t.Logf("action getParams via proxy: baseActionFee=%s", baseFee)
}

// testContractProxiesSupernodeGetParams deploys a proxy contract targeting the
// supernode precompile (0x0902). The proxy forwards a getParams() call via
// STATICCALL, verifying independent precompile accessibility from contracts.
func testContractProxiesSupernodeGetParams(t *testing.T, node *evmtest.Node) {
	t.Helper()

	proxyAddr := deployStaticCallProxy(t, node, 0x0902)

	calldata, err := supernodeprecompile.ABI.Pack(supernodeprecompile.GetParamsMethod)
	if err != nil {
		t.Fatalf("pack getParams: %v", err)
	}

	resultBz := ethCallProxy(t, node, proxyAddr, calldata)

	out, err := supernodeprecompile.ABI.Unpack(supernodeprecompile.GetParamsMethod, resultBz)
	if err != nil {
		t.Fatalf("unpack getParams: %v", err)
	}
	if len(out) != 7 {
		t.Fatalf("expected 7 return values, got %d", len(out))
	}

	// minimumStake (uint256) should be positive
	minStake, ok := out[0].(*big.Int)
	if !ok || minStake == nil || minStake.Sign() <= 0 {
		t.Fatalf("minimumStake should be > 0, got %v", out[0])
	}

	// minSupernodeVersion (string) should be non-empty
	version, ok := out[3].(string)
	if !ok || version == "" {
		t.Fatalf("minSupernodeVersion should be non-empty, got %v", out[3])
	}

	t.Logf("supernode getParams via proxy: minStake=%s, minVersion=%s", minStake, version)
}

// testContractProxiesActionGetActionFee deploys a proxy that forwards
// getActionFee(100) to the action precompile. This validates that ABI-encoded
// parameters survive the contract→precompile STATICCALL forwarding path and
// that the fee arithmetic is correct.
func testContractProxiesActionGetActionFee(t *testing.T, node *evmtest.Node) {
	t.Helper()

	proxyAddr := deployStaticCallProxy(t, node, 0x0901)

	dataSizeKbs := uint64(100)
	calldata, err := actionprecompile.ABI.Pack(actionprecompile.GetActionFeeMethod, dataSizeKbs)
	if err != nil {
		t.Fatalf("pack getActionFee: %v", err)
	}

	resultBz := ethCallProxy(t, node, proxyAddr, calldata)

	out, err := actionprecompile.ABI.Unpack(actionprecompile.GetActionFeeMethod, resultBz)
	if err != nil {
		t.Fatalf("unpack getActionFee: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 return values (baseFee, perKbFee, totalFee), got %d", len(out))
	}

	baseFee, _ := out[0].(*big.Int)
	perKbFee, _ := out[1].(*big.Int)
	totalFee, _ := out[2].(*big.Int)

	if baseFee == nil || perKbFee == nil || totalFee == nil {
		t.Fatalf("fee values must not be nil: base=%v perKb=%v total=%v", out[0], out[1], out[2])
	}

	// totalFee should equal baseFee + perKbFee * dataSizeKbs
	expected := new(big.Int).Add(baseFee, new(big.Int).Mul(perKbFee, big.NewInt(int64(dataSizeKbs))))
	if totalFee.Cmp(expected) != 0 {
		t.Fatalf("fee arithmetic mismatch: total=%s, expected baseFee(%s) + perKbFee(%s)*%d = %s",
			totalFee, baseFee, perKbFee, dataSizeKbs, expected)
	}

	t.Logf("action getActionFee(100) via proxy: base=%s perKb=%s total=%s", baseFee, perKbFee, totalFee)
}

// testContractQueriesBothPrecompiles deploys two proxy contracts — one for
// each Lumera custom precompile — and queries both in the same test. This
// validates that multiple precompiles are independently callable from
// deployed contracts within the same block context.
func testContractQueriesBothPrecompiles(t *testing.T, node *evmtest.Node) {
	t.Helper()

	// Deploy proxies for both precompiles
	actionProxy := deployStaticCallProxy(t, node, 0x0901)
	supernodeProxy := deployStaticCallProxy(t, node, 0x0902)

	// 1. Query action precompile: getActionFee(50)
	actionCalldata, err := actionprecompile.ABI.Pack(actionprecompile.GetActionFeeMethod, uint64(50))
	if err != nil {
		t.Fatalf("pack action getActionFee: %v", err)
	}
	actionResult := ethCallProxy(t, node, actionProxy, actionCalldata)
	actionOut, err := actionprecompile.ABI.Unpack(actionprecompile.GetActionFeeMethod, actionResult)
	if err != nil {
		t.Fatalf("unpack action getActionFee: %v", err)
	}
	if len(actionOut) != 3 {
		t.Fatalf("expected 3 action fee values, got %d", len(actionOut))
	}
	totalFee, _ := actionOut[2].(*big.Int)
	if totalFee == nil || totalFee.Sign() <= 0 {
		t.Fatalf("action totalFee should be > 0, got %v", actionOut[2])
	}

	// 2. Query supernode precompile: listSuperNodes(0, 10)
	snCalldata, err := supernodeprecompile.ABI.Pack(supernodeprecompile.ListSuperNodesMethod, uint64(0), uint64(10))
	if err != nil {
		t.Fatalf("pack supernode listSuperNodes: %v", err)
	}
	snResult := ethCallProxy(t, node, supernodeProxy, snCalldata)
	snOut, err := supernodeprecompile.ABI.Unpack(supernodeprecompile.ListSuperNodesMethod, snResult)
	if err != nil {
		t.Fatalf("unpack supernode listSuperNodes: %v", err)
	}
	if len(snOut) != 2 {
		t.Fatalf("expected 2 listSuperNodes values (nodes[], total), got %d", len(snOut))
	}

	// total is uint64 — valid even if 0 on a fresh chain
	total, ok := snOut[1].(uint64)
	if !ok {
		t.Fatalf("total should be uint64, got %T", snOut[1])
	}

	t.Logf("dual-precompile query: action fee(50KB)=%s, supernode count=%d", totalFee, total)
}
