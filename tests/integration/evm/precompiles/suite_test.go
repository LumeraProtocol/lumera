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
}
