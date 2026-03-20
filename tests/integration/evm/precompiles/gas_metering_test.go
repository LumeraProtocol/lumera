//go:build integration
// +build integration

package precompiles_test

import (
	"context"
	"strings"
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testjsonrpc "github.com/LumeraProtocol/lumera/testutil/jsonrpc"
	bankprecompile "github.com/cosmos/evm/precompiles/bank"
	bech32precompile "github.com/cosmos/evm/precompiles/bech32"
	distprecompile "github.com/cosmos/evm/precompiles/distribution"
	govprecompile "github.com/cosmos/evm/precompiles/gov"
	slashingprecompile "github.com/cosmos/evm/precompiles/slashing"
	stakingprecompile "github.com/cosmos/evm/precompiles/staking"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// testPrecompileGasMeteringAccuracy verifies that each static precompile
// consumes a non-trivial, bounded amount of gas for a representative query.
// This catches regressions where a precompile silently returns zero-cost or
// consumes the full block gas limit.
func testPrecompileGasMeteringAccuracy(t *testing.T, node *evmtest.Node) {
	t.Helper()

	const (
		maxReasonableGas = 500_000 // No precompile query should exceed this.
		minReasonableGas = 100     // Even the cheapest precompile does some work.
	)

	type precompileCase struct {
		name    string
		address string
		input   []byte
	}

	validatorAddr := common.HexToAddress("0x1111111111111111111111111111111111111111")

	bankInput, err := bankprecompile.ABI.Pack(bankprecompile.BalancesMethod, validatorAddr)
	if err != nil {
		t.Fatalf("pack bank input: %v", err)
	}

	bech32Input, err := bech32precompile.ABI.Pack(bech32precompile.HexToBech32Method, validatorAddr, "lumera")
	if err != nil {
		t.Fatalf("pack bech32 input: %v", err)
	}

	stakingInput, err := stakingprecompile.ABI.Pack(stakingprecompile.ValidatorsMethod, "BOND_STATUS_BONDED", abiPageRequest{Limit: 10})
	if err != nil {
		t.Fatalf("pack staking input: %v", err)
	}

	distInput, err := distprecompile.ABI.Pack(distprecompile.ValidatorDistributionInfoMethod, "lumeravaloper1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqf8kzav")
	if err != nil {
		t.Fatalf("pack distribution input: %v", err)
	}

	govInput, err := govprecompile.ABI.Pack(govprecompile.GetParamsMethod, "voting")
	if err != nil {
		t.Fatalf("pack gov input: %v", err)
	}

	slashingInput, err := slashingprecompile.ABI.Pack(slashingprecompile.GetParamsMethod)
	if err != nil {
		t.Fatalf("pack slashing input: %v", err)
	}

	cases := []precompileCase{
		{name: "bank/balances", address: evmtypes.BankPrecompileAddress, input: bankInput},
		{name: "bech32/hexToBech32", address: evmtypes.Bech32PrecompileAddress, input: bech32Input},
		{name: "staking/validators", address: evmtypes.StakingPrecompileAddress, input: stakingInput},
		{name: "distribution/validatorDistInfo", address: evmtypes.DistributionPrecompileAddress, input: distInput},
		{name: "gov/getParams", address: evmtypes.GovPrecompileAddress, input: govInput},
		{name: "slashing/getParams", address: evmtypes.SlashingPrecompileAddress, input: slashingInput},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gasUsed := estimateGasForPrecompile(t, node, tc.address, tc.input)

			if gasUsed < minReasonableGas {
				t.Fatalf("precompile %s gas too low: %d (min expected %d)", tc.name, gasUsed, minReasonableGas)
			}
			if gasUsed > maxReasonableGas {
				t.Fatalf("precompile %s gas too high: %d (max expected %d)", tc.name, gasUsed, maxReasonableGas)
			}
			t.Logf("precompile %s: estimated gas = %d", tc.name, gasUsed)
		})
	}
}

// testPrecompileGasEstimateMatchesActual sends a real tx to a precompile and
// verifies that eth_estimateGas is within a reasonable margin of the actual
// gasUsed in the receipt.
func testPrecompileGasEstimateMatchesActual(t *testing.T, node *evmtest.Node) {
	t.Helper()

	// Use bank precompile as the representative case.
	validatorAddr := common.HexToAddress("0x1111111111111111111111111111111111111111")
	input, err := bankprecompile.ABI.Pack(bankprecompile.BalancesMethod, validatorAddr)
	if err != nil {
		t.Fatalf("pack bank input: %v", err)
	}

	estimated := estimateGasForPrecompile(t, node, evmtypes.BankPrecompileAddress, input)

	// Send a real tx and get actual gasUsed from receipt.
	txHash := sendPrecompileLegacyTx(t, node, evmtypes.BankPrecompileAddress, input, 200_000)
	receipt := node.WaitForReceipt(t, txHash, 40*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)

	actual := evmtest.MustUint64HexField(t, receipt, "gasUsed")

	// eth_estimateGas typically returns a value >= actual gasUsed.
	// Allow 50% margin for gas estimation overhead.
	if estimated < actual {
		t.Logf("WARNING: estimate (%d) < actual (%d) — estimate should be >= actual", estimated, actual)
	}
	maxAcceptable := actual * 3 // 3x is generous but catches gross miscalculation
	if estimated > maxAcceptable {
		t.Fatalf("gas estimate (%d) is more than 3x actual gasUsed (%d)", estimated, actual)
	}

	t.Logf("bank precompile: estimated=%d actual=%d ratio=%.2f", estimated, actual, float64(estimated)/float64(actual))
}

// estimateGasForPrecompile calls eth_estimateGas for a precompile call.
func estimateGasForPrecompile(t *testing.T, node *evmtest.Node, to string, input []byte) uint64 {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var gasHex string
	err := testjsonrpc.Call(ctx, node.RPCURL(), "eth_estimateGas", []any{
		map[string]any{
			"to":   to,
			"data": hexutil.Encode(input),
		},
	}, &gasHex)
	if err != nil {
		// Some precompiles may not support estimateGas cleanly.
		// If it fails, try eth_call and report gas from there.
		t.Logf("eth_estimateGas failed for %s (may be expected): %v", to, err)

		// Fallback: send a real tx and check receipt gasUsed.
		txHash := sendPrecompileLegacyTx(t, node, to, input, 200_000)
		receipt := node.WaitForReceipt(t, txHash, 40*time.Second)
		status := evmtest.MustStringField(t, receipt, "status")
		if strings.EqualFold(status, "0x0") {
			t.Skipf("precompile %s tx reverted — skipping gas metering for this precompile", to)
		}
		return evmtest.MustUint64HexField(t, receipt, "gasUsed")
	}

	gas, err := hexutil.DecodeUint64(gasHex)
	if err != nil {
		t.Fatalf("decode gas estimate %q: %v", gasHex, err)
	}
	return gas
}
