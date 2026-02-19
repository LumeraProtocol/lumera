//go:build integration
// +build integration

package precompiles_test

import (
	"strings"
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	stakingprecompile "github.com/cosmos/evm/precompiles/staking"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// TestStakingPrecompileValidatorViaEthCall verifies staking static precompile
// validator query returns active validator data for the local test validator.
func testStakingPrecompileValidatorViaEthCall(t *testing.T, node *evmtest.Node) {
	t.Helper()
	evmtest.WaitForBlockNumberAtLeast(t, node.RPCURL(), 1, 20*time.Second)

	validatorHex := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	input, err := stakingprecompile.ABI.Pack(stakingprecompile.ValidatorMethod, validatorHex)
	if err != nil {
		t.Fatalf("pack staking validator input: %v", err)
	}

	result := mustEthCallPrecompile(t, node.RPCURL(), evmtypes.StakingPrecompileAddress, input)
	var out struct {
		Validator stakingprecompile.ValidatorInfo `abi:"validator"`
	}
	if err := stakingprecompile.ABI.UnpackIntoInterface(&out, stakingprecompile.ValidatorMethod, result); err != nil {
		t.Fatalf("unpack staking validator output: %v", err)
	}

	if strings.TrimSpace(out.Validator.OperatorAddress) == "" {
		t.Fatalf("unexpected empty operatorAddress in staking validator output: %#v", out)
	}
	if !strings.EqualFold(out.Validator.OperatorAddress, validatorHex.Hex()) {
		t.Fatalf("unexpected operatorAddress: got=%s want=%s", out.Validator.OperatorAddress, validatorHex.Hex())
	}
	if out.Validator.Tokens == nil || out.Validator.Tokens.Sign() <= 0 {
		t.Fatalf("unexpected validator tokens in staking output: %#v", out.Validator)
	}
}
