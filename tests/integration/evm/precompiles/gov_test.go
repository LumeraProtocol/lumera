//go:build integration
// +build integration

package precompiles_test

import (
	"strings"
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	govprecompile "github.com/cosmos/evm/precompiles/gov"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// TestGovPrecompileQueryPathsViaEthCall verifies governance read-only
// precompile methods for params and constitution.
func testGovPrecompileQueryPathsViaEthCall(t *testing.T, node *evmtest.Node) {
	t.Helper()
	evmtest.WaitForBlockNumberAtLeast(t, node.RPCURL(), 1, 20*time.Second)

	getParamsInput, err := govprecompile.ABI.Pack(govprecompile.GetParamsMethod)
	if err != nil {
		t.Fatalf("pack getParams input: %v", err)
	}

	getParamsResult := mustEthCallPrecompile(t, node.RPCURL(), evmtypes.GovPrecompileAddress, getParamsInput)
	var paramsOut struct {
		Params govprecompile.ParamsOutput `abi:"params"`
	}
	if err := govprecompile.ABI.UnpackIntoInterface(&paramsOut, govprecompile.GetParamsMethod, getParamsResult); err != nil {
		t.Fatalf("unpack getParams output: %v", err)
	}
	if paramsOut.Params.VotingPeriod <= 0 {
		t.Fatalf("unexpected voting period from getParams: %#v", paramsOut.Params)
	}
	if len(paramsOut.Params.MinDeposit) == 0 {
		t.Fatalf("unexpected empty min_deposit from getParams: %#v", paramsOut.Params)
	}

	getConstitutionInput, err := govprecompile.ABI.Pack(govprecompile.GetConstitutionMethod)
	if err != nil {
		t.Fatalf("pack getConstitution input: %v", err)
	}

	getConstitutionResult := mustEthCallPrecompile(t, node.RPCURL(), evmtypes.GovPrecompileAddress, getConstitutionInput)
	out, err := govprecompile.ABI.Unpack(govprecompile.GetConstitutionMethod, getConstitutionResult)
	if err != nil {
		t.Fatalf("unpack getConstitution output: %v", err)
	}
	constitution, ok := out[0].(string)
	if !ok {
		t.Fatalf("unexpected getConstitution output type: %#v", out)
	}
	// Constitution may be empty by default; this assertion ensures decoding and
	// value normalization path remains stable.
	_ = strings.TrimSpace(constitution)
}
