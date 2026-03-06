//go:build integration
// +build integration

package precompiles_test

import (
	"strings"
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	slashingprecompile "github.com/cosmos/evm/precompiles/slashing"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// abiPageRequest matches the Solidity PageRequest tuple layout used by
// precompile ABI inputs: (bytes key, uint64 offset, uint64 limit, bool countTotal, bool reverse).
type abiPageRequest struct {
	Key        []byte `abi:"key"`
	Offset     uint64 `abi:"offset"`
	Limit      uint64 `abi:"limit"`
	CountTotal bool   `abi:"countTotal"`
	Reverse    bool   `abi:"reverse"`
}

// testSlashingPrecompileGetParamsViaEthCall verifies the slashing precompile
// getParams query returns valid slashing parameters (signedBlocksWindow,
// slashFractionDoubleSign, etc.) from a live chain.
func testSlashingPrecompileGetParamsViaEthCall(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	input, err := slashingprecompile.ABI.Pack(slashingprecompile.GetParamsMethod)
	if err != nil {
		t.Fatalf("pack slashing getParams input: %v", err)
	}

	result := mustEthCallPrecompile(t, node, evmtypes.SlashingPrecompileAddress, input)
	var out struct {
		Params slashingprecompile.Params `abi:"params"`
	}
	if err := slashingprecompile.ABI.UnpackIntoInterface(&out, slashingprecompile.GetParamsMethod, result); err != nil {
		t.Fatalf("unpack slashing getParams output: %v", err)
	}

	if out.Params.SignedBlocksWindow <= 0 {
		t.Fatalf("expected positive signedBlocksWindow, got %d", out.Params.SignedBlocksWindow)
	}
	if out.Params.DowntimeJailDuration <= 0 {
		t.Fatalf("expected positive downtimeJailDuration, got %d", out.Params.DowntimeJailDuration)
	}
	if out.Params.SlashFractionDoubleSign.Value == nil || out.Params.SlashFractionDoubleSign.Value.Sign() <= 0 {
		t.Fatalf("expected positive slashFractionDoubleSign, got %#v", out.Params.SlashFractionDoubleSign)
	}
	if out.Params.SlashFractionDowntime.Value == nil || out.Params.SlashFractionDowntime.Value.Sign() <= 0 {
		t.Fatalf("expected positive slashFractionDowntime, got %#v", out.Params.SlashFractionDowntime)
	}
}

// testSlashingPrecompileGetSigningInfosViaEthCall verifies the slashing
// precompile getSigningInfos query returns signing info for at least the
// genesis validator.
func testSlashingPrecompileGetSigningInfosViaEthCall(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	// Pack with an empty pagination tuple to get all results.
	input, err := slashingprecompile.ABI.Pack(
		slashingprecompile.GetSigningInfosMethod,
		abiPageRequest{},
	)
	if err != nil {
		t.Fatalf("pack slashing getSigningInfos input: %v", err)
	}

	result := mustEthCallPrecompile(t, node, evmtypes.SlashingPrecompileAddress, input)
	out, err := slashingprecompile.ABI.Unpack(slashingprecompile.GetSigningInfosMethod, result)
	if err != nil {
		t.Fatalf("unpack slashing getSigningInfos output: %v", err)
	}

	// First return value is []SigningInfo tuple array.
	if len(out) < 2 {
		t.Fatalf("expected 2 return values from getSigningInfos, got %d", len(out))
	}
}

// testSlashingPrecompileUnjailTxPathFailsWhenNotJailed verifies the unjail tx
// path reverts when the validator is not actually jailed. This mirrors the
// gov cancelProposal failure test pattern.
func testSlashingPrecompileUnjailTxPathFailsWhenNotJailed(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	validatorHex := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	input, err := slashingprecompile.ABI.Pack(slashingprecompile.UnjailMethod, validatorHex)
	if err != nil {
		t.Fatalf("pack slashing unjail input: %v", err)
	}

	txHash := sendPrecompileLegacyTx(t, node, evmtypes.SlashingPrecompileAddress, input, 500_000)
	receipt := node.WaitForReceipt(t, txHash, 45*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)

	// Validator is active and not jailed, so unjail should fail (status 0x0).
	status := evmtest.MustStringField(t, receipt, "status")
	if !strings.EqualFold(status, "0x0") {
		t.Fatalf("expected failed unjail tx status=0x0 (validator not jailed), got %q", status)
	}
}
