//go:build integration
// +build integration

package precompiles_test

import (
	"math/big"
	"strings"
	"testing"
	"time"

	lcfg "github.com/LumeraProtocol/lumera/config"
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	sdk "github.com/cosmos/cosmos-sdk/types"
	distributionprecompile "github.com/cosmos/evm/precompiles/distribution"
	govprecompile "github.com/cosmos/evm/precompiles/gov"
	stakingprecompile "github.com/cosmos/evm/precompiles/staking"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// TestStakingPrecompileDelegateTxPath verifies the staking precompile tx method
// `delegate` can be executed through eth_sendRawTransaction.
//
// Workflow:
// 1. Build delegate calldata for delegator -> genesis validator.
// 2. Broadcast legacy tx to staking precompile.
// 3. Assert successful receipt and non-zero delegation shares via follow-up query.
func testStakingPrecompileDelegateTxPath(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	delegatorHex := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	validatorAddr, err := sdk.Bech32ifyAddressBytes(lcfg.Bech32ValidatorAddressPrefix, delegatorHex.Bytes())
	if err != nil {
		t.Fatalf("build validator address: %v", err)
	}

	delegateInput, err := stakingprecompile.ABI.Pack(stakingprecompile.DelegateMethod, delegatorHex, validatorAddr, big.NewInt(1))
	if err != nil {
		t.Fatalf("pack staking delegate input: %v", err)
	}

	txHash := sendPrecompileLegacyTx(t, node, evmtypes.StakingPrecompileAddress, delegateInput, 500_000)
	receipt := node.WaitForReceipt(t, txHash, 45*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)
	if status := evmtest.MustStringField(t, receipt, "status"); !strings.EqualFold(status, "0x1") {
		t.Fatalf("expected successful staking delegate tx status, got %q (%#v)", status, receipt)
	}

	delegationQueryInput, err := stakingprecompile.ABI.Pack(stakingprecompile.DelegationMethod, delegatorHex, validatorAddr)
	if err != nil {
		t.Fatalf("pack staking delegation query input: %v", err)
	}

	delegationQueryResult := mustEthCallPrecompile(t, node, evmtypes.StakingPrecompileAddress, delegationQueryInput)
	out, err := stakingprecompile.ABI.Unpack(stakingprecompile.DelegationMethod, delegationQueryResult)
	if err != nil {
		t.Fatalf("unpack staking delegation query output: %v", err)
	}
	shares, ok := out[0].(*big.Int)
	if !ok || shares == nil {
		t.Fatalf("unexpected staking delegation shares output: %#v", out)
	}
	if shares.Sign() <= 0 {
		t.Fatalf("expected positive delegation shares after delegate tx, got %s", shares.String())
	}
}

// TestDistributionPrecompileSetWithdrawAddressTxPath verifies distribution
// precompile tx method `setWithdrawAddress` via eth_sendRawTransaction.
func testDistributionPrecompileSetWithdrawAddressTxPath(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	delegatorHex := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	withdrawerAddr := node.KeyInfo().Address

	setWithdrawerInput, err := distributionprecompile.ABI.Pack(distributionprecompile.SetWithdrawAddressMethod, delegatorHex, withdrawerAddr)
	if err != nil {
		t.Fatalf("pack distribution setWithdrawAddress input: %v", err)
	}

	txHash := sendPrecompileLegacyTx(t, node, evmtypes.DistributionPrecompileAddress, setWithdrawerInput, 500_000)
	receipt := node.WaitForReceipt(t, txHash, 45*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)
	if status := evmtest.MustStringField(t, receipt, "status"); !strings.EqualFold(status, "0x1") {
		t.Fatalf("expected successful distribution setWithdrawAddress tx status, got %q (%#v)", status, receipt)
	}

	queryInput, err := distributionprecompile.ABI.Pack(distributionprecompile.DelegatorWithdrawAddressMethod, delegatorHex)
	if err != nil {
		t.Fatalf("pack delegatorWithdrawAddress query input: %v", err)
	}
	queryResult := mustEthCallPrecompile(t, node, evmtypes.DistributionPrecompileAddress, queryInput)
	out, err := distributionprecompile.ABI.Unpack(distributionprecompile.DelegatorWithdrawAddressMethod, queryResult)
	if err != nil {
		t.Fatalf("unpack delegatorWithdrawAddress query output: %v", err)
	}
	got, ok := out[0].(string)
	if !ok {
		t.Fatalf("unexpected delegatorWithdrawAddress output type: %#v", out)
	}
	if got != withdrawerAddr {
		t.Fatalf("unexpected withdraw address after tx: got=%s want=%s", got, withdrawerAddr)
	}
}

// TestGovPrecompileCancelProposalTxPathFailsForUnknownProposal verifies gov
// precompile tx-path failure semantics on a non-existent proposal id.
func testGovPrecompileCancelProposalTxPathFailsForUnknownProposal(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	proposerHex := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	cancelInput, err := govprecompile.ABI.Pack(govprecompile.CancelProposalMethod, proposerHex, uint64(9_999_999))
	if err != nil {
		t.Fatalf("pack gov cancelProposal input: %v", err)
	}

	txHash := sendPrecompileLegacyTx(t, node, evmtypes.GovPrecompileAddress, cancelInput, 500_000)
	receipt := node.WaitForReceipt(t, txHash, 45*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)

	status := evmtest.MustStringField(t, receipt, "status")
	if !strings.EqualFold(status, "0x0") {
		t.Fatalf("expected failed gov cancelProposal tx status=0x0, got %q (%#v)", status, receipt)
	}
}
