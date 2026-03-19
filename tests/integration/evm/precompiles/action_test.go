//go:build integration
// +build integration

package precompiles_test

import (
	"math/big"
	"testing"
	"time"

	actionprecompile "github.com/LumeraProtocol/lumera/precompiles/action"
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
)

// testActionPrecompileGetParamsViaEthCall verifies the action precompile
// `getParams()` query returns valid module parameters via eth_call.
func testActionPrecompileGetParamsViaEthCall(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	input, err := actionprecompile.ABI.Pack(actionprecompile.GetParamsMethod)
	if err != nil {
		t.Fatalf("pack getParams input: %v", err)
	}

	result := mustEthCallPrecompile(t, node, actionprecompile.ActionPrecompileAddress, input)
	out, err := actionprecompile.ABI.Unpack(actionprecompile.GetParamsMethod, result)
	if err != nil {
		t.Fatalf("unpack getParams output: %v", err)
	}

	// getParams returns: baseActionFee, feePerKbyte, maxActionsPerBlock, minSuperNodes,
	//                     expirationDuration, superNodeFeeShare, foundationFeeShare
	if len(out) != 7 {
		t.Fatalf("expected 7 return values from getParams, got %d", len(out))
	}

	baseActionFee, ok := out[0].(*big.Int)
	if !ok || baseActionFee == nil {
		t.Fatalf("unexpected baseActionFee type: %#v", out[0])
	}
	// Default baseActionFee is 10000 ulume
	if baseActionFee.Cmp(big.NewInt(0)) <= 0 {
		t.Fatalf("expected positive baseActionFee, got %s", baseActionFee.String())
	}

	feePerKbyte, ok := out[1].(*big.Int)
	if !ok || feePerKbyte == nil {
		t.Fatalf("unexpected feePerKbyte type: %#v", out[1])
	}
	if feePerKbyte.Cmp(big.NewInt(0)) <= 0 {
		t.Fatalf("expected positive feePerKbyte, got %s", feePerKbyte.String())
	}

	maxActionsPerBlock, ok := out[2].(uint64)
	if !ok {
		t.Fatalf("unexpected maxActionsPerBlock type: %#v", out[2])
	}
	if maxActionsPerBlock == 0 {
		t.Fatalf("expected non-zero maxActionsPerBlock")
	}

	superNodeFeeShare, ok := out[5].(string)
	if !ok || superNodeFeeShare == "" {
		t.Fatalf("unexpected superNodeFeeShare: %#v", out[5])
	}

	foundationFeeShare, ok := out[6].(string)
	if !ok {
		t.Fatalf("unexpected foundationFeeShare: %#v", out[6])
	}
	_ = foundationFeeShare
}

// testActionPrecompileGetActionFeeViaEthCall verifies the action precompile
// `getActionFee(uint64)` query returns correct fee breakdown.
func testActionPrecompileGetActionFeeViaEthCall(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	// Query fee for 100 KB of data
	dataSizeKbs := uint64(100)
	input, err := actionprecompile.ABI.Pack(actionprecompile.GetActionFeeMethod, dataSizeKbs)
	if err != nil {
		t.Fatalf("pack getActionFee input: %v", err)
	}

	result := mustEthCallPrecompile(t, node, actionprecompile.ActionPrecompileAddress, input)
	out, err := actionprecompile.ABI.Unpack(actionprecompile.GetActionFeeMethod, result)
	if err != nil {
		t.Fatalf("unpack getActionFee output: %v", err)
	}

	if len(out) != 3 {
		t.Fatalf("expected 3 return values from getActionFee, got %d", len(out))
	}

	baseFee, ok := out[0].(*big.Int)
	if !ok || baseFee == nil {
		t.Fatalf("unexpected baseFee type: %#v", out[0])
	}

	perKbFee, ok := out[1].(*big.Int)
	if !ok || perKbFee == nil {
		t.Fatalf("unexpected perKbFee type: %#v", out[1])
	}

	totalFee, ok := out[2].(*big.Int)
	if !ok || totalFee == nil {
		t.Fatalf("unexpected totalFee type: %#v", out[2])
	}

	// totalFee should equal baseFee + (perKbFee * dataSizeKbs)
	expectedTotal := new(big.Int).Add(
		baseFee,
		new(big.Int).Mul(perKbFee, new(big.Int).SetUint64(dataSizeKbs)),
	)
	if totalFee.Cmp(expectedTotal) != 0 {
		t.Fatalf("totalFee mismatch: got %s, expected %s (baseFee=%s, perKbFee=%s, dataSize=%d)",
			totalFee.String(), expectedTotal.String(), baseFee.String(), perKbFee.String(), dataSizeKbs)
	}

	if totalFee.Cmp(big.NewInt(0)) <= 0 {
		t.Fatalf("expected positive totalFee for %d KB, got %s", dataSizeKbs, totalFee.String())
	}
}

// testActionPrecompileGetActionsByStateViaEthCall verifies the action precompile
// `getActionsByState(uint8,uint64,uint64)` query returns empty list when no actions exist.
func testActionPrecompileGetActionsByStateViaEthCall(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	// Query pending actions (state=1), expect empty on fresh chain
	input, err := actionprecompile.ABI.Pack(
		actionprecompile.GetActionsByStateMethod,
		uint8(1),   // ActionStatePending
		uint64(0),  // offset
		uint64(10), // limit
	)
	if err != nil {
		t.Fatalf("pack getActionsByState input: %v", err)
	}

	result := mustEthCallPrecompile(t, node, actionprecompile.ActionPrecompileAddress, input)
	out, err := actionprecompile.ABI.Unpack(actionprecompile.GetActionsByStateMethod, result)
	if err != nil {
		t.Fatalf("unpack getActionsByState output: %v", err)
	}

	if len(out) != 2 {
		t.Fatalf("expected 2 return values from getActionsByState, got %d", len(out))
	}

	// The first return is the ActionInfo[] array, second is total count
	total, ok := out[1].(uint64)
	if !ok {
		t.Fatalf("unexpected total type: %#v", out[1])
	}
	if total != 0 {
		t.Fatalf("expected 0 pending actions on fresh chain, got %d", total)
	}
}

// testActionPrecompileGetActionsByCreatorViaEthCall verifies the action precompile
// `getActionsByCreator(address,uint64,uint64)` query.
func testActionPrecompileGetActionsByCreatorViaEthCall(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	// Use a random address that has no actions
	emptyAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	input, err := actionprecompile.ABI.Pack(
		actionprecompile.GetActionsByCreatorMethod,
		emptyAddr,
		uint64(0),  // offset
		uint64(10), // limit
	)
	if err != nil {
		t.Fatalf("pack getActionsByCreator input: %v", err)
	}

	result := mustEthCallPrecompile(t, node, actionprecompile.ActionPrecompileAddress, input)
	out, err := actionprecompile.ABI.Unpack(actionprecompile.GetActionsByCreatorMethod, result)
	if err != nil {
		t.Fatalf("unpack getActionsByCreator output: %v", err)
	}

	if len(out) != 2 {
		t.Fatalf("expected 2 return values, got %d", len(out))
	}
}
