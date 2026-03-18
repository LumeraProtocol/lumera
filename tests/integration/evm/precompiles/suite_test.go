//go:build integration
// +build integration

package precompiles_test

import (
	"testing"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
)

// TestPrecompilesSuite runs precompile integration coverage against a single
// node instance to avoid repeated chain startup overhead per test file.
func TestPrecompilesSuite(t *testing.T) {
	node := evmtest.NewEVMNode(t, "lumera-precompiles-suite", 500)
	node.StartAndWaitRPC()
	defer node.Stop()

	t.Run("BankPrecompileBalancesViaEthCall", func(t *testing.T) {
		testBankPrecompileBalancesViaEthCall(t, node)
	})
	t.Run("DistributionPrecompileQueryPathsViaEthCall", func(t *testing.T) {
		testDistributionPrecompileQueryPathsViaEthCall(t, node)
	})
	t.Run("GovPrecompileQueryPathsViaEthCall", func(t *testing.T) {
		testGovPrecompileQueryPathsViaEthCall(t, node)
	})
	t.Run("StakingPrecompileValidatorViaEthCall", func(t *testing.T) {
		testStakingPrecompileValidatorViaEthCall(t, node)
	})
	t.Run("Bech32PrecompileRoundTripViaEthCall", func(t *testing.T) {
		testBech32PrecompileRoundTripViaEthCall(t, node)
	})
	t.Run("P256PrecompileVerifyViaEthCall", func(t *testing.T) {
		testP256PrecompileVerifyViaEthCall(t, node)
	})
	t.Run("StakingPrecompileDelegateTxPath", func(t *testing.T) {
		testStakingPrecompileDelegateTxPath(t, node)
	})
	t.Run("DistributionPrecompileSetWithdrawAddressTxPath", func(t *testing.T) {
		testDistributionPrecompileSetWithdrawAddressTxPath(t, node)
	})
	t.Run("GovPrecompileCancelProposalTxPathFailsForUnknownProposal", func(t *testing.T) {
		testGovPrecompileCancelProposalTxPathFailsForUnknownProposal(t, node)
	})

	// Slashing precompile tests
	t.Run("SlashingPrecompileGetParamsViaEthCall", func(t *testing.T) {
		testSlashingPrecompileGetParamsViaEthCall(t, node)
	})
	t.Run("SlashingPrecompileGetSigningInfosViaEthCall", func(t *testing.T) {
		testSlashingPrecompileGetSigningInfosViaEthCall(t, node)
	})
	t.Run("SlashingPrecompileUnjailTxPathFailsWhenNotJailed", func(t *testing.T) {
		testSlashingPrecompileUnjailTxPathFailsWhenNotJailed(t, node)
	})

	// ICS20 precompile tests
	t.Run("ICS20PrecompileDenomsViaEthCall", func(t *testing.T) {
		testICS20PrecompileDenomsViaEthCall(t, node)
	})
	t.Run("ICS20PrecompileDenomHashViaEthCall", func(t *testing.T) {
		testICS20PrecompileDenomHashViaEthCall(t, node)
	})
	t.Run("ICS20PrecompileDenomViaEthCall", func(t *testing.T) {
		testICS20PrecompileDenomViaEthCall(t, node)
	})
	// NOTE: ICS20 transfer tx test is omitted because the IBC store ordering
	// bug causes a panic in the node process, which would corrupt subsequent
	// tests in this suite. The ICS20 query tests above use t.Skip when the
	// bug is detected, which is safe. See ics20_test.go for details.

	// Action precompile tests
	t.Run("ActionPrecompileGetParamsViaEthCall", func(t *testing.T) {
		testActionPrecompileGetParamsViaEthCall(t, node)
	})
	t.Run("ActionPrecompileGetActionFeeViaEthCall", func(t *testing.T) {
		testActionPrecompileGetActionFeeViaEthCall(t, node)
	})
	t.Run("ActionPrecompileGetActionsByStateViaEthCall", func(t *testing.T) {
		testActionPrecompileGetActionsByStateViaEthCall(t, node)
	})
	t.Run("ActionPrecompileGetActionsByCreatorViaEthCall", func(t *testing.T) {
		testActionPrecompileGetActionsByCreatorViaEthCall(t, node)
	})

	// Supernode precompile tests
	t.Run("SupernodePrecompileGetParamsViaEthCall", func(t *testing.T) {
		testSupernodePrecompileGetParamsViaEthCall(t, node)
	})
	t.Run("SupernodePrecompileListSuperNodesViaEthCall", func(t *testing.T) {
		testSupernodePrecompileListSuperNodesViaEthCall(t, node)
	})
	t.Run("SupernodePrecompileGetTopSuperNodesForBlockViaEthCall", func(t *testing.T) {
		testSupernodePrecompileGetTopSuperNodesForBlockViaEthCall(t, node)
	})

	// Gas metering accuracy tests
	t.Run("PrecompileGasMeteringAccuracy", func(t *testing.T) {
		testPrecompileGasMeteringAccuracy(t, node)
	})
	t.Run("PrecompileGasEstimateMatchesActual", func(t *testing.T) {
		testPrecompileGasEstimateMatchesActual(t, node)
	})
}
