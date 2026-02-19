//go:build integration
// +build integration

package precompiles_test

import (
	"math/big"
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	bankprecompile "github.com/cosmos/evm/precompiles/bank"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
)

// TestBankPrecompileBalancesViaEthCall verifies the bank static precompile
// `balances(address)` query over JSON-RPC eth_call for both funded and empty
// accounts.
//
// Note: Bank precompile returns only balances that have ERC20 token-pair
// mappings. On Lumera defaults this set may be empty, which is still a valid
// response.
func testBankPrecompileBalancesViaEthCall(t *testing.T, node *evmtest.Node) {
	t.Helper()
	evmtest.WaitForBlockNumberAtLeast(t, node.RPCURL(), 1, 20*time.Second)

	fundedAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	fundedInput, err := bankprecompile.ABI.Pack(bankprecompile.BalancesMethod, fundedAddr)
	if err != nil {
		t.Fatalf("pack balances(address funded): %v", err)
	}

	fundedResult := mustEthCallPrecompile(t, node.RPCURL(), evmtypes.BankPrecompileAddress, fundedInput)
	var fundedBalances []bankprecompile.Balance
	if err := bankprecompile.ABI.UnpackIntoInterface(&fundedBalances, bankprecompile.BalancesMethod, fundedResult); err != nil {
		t.Fatalf("unpack funded balances: %v", err)
	}
	// If mappings exist, output must be structurally valid and contain
	// non-negative balances.
	for _, bal := range fundedBalances {
		if bal.ContractAddress == (common.Address{}) {
			t.Fatalf("unexpected zero contract address in balances: %#v", fundedBalances)
		}
		if bal.Amount == nil || bal.Amount.Cmp(big.NewInt(0)) < 0 {
			t.Fatalf("unexpected invalid balance amount in %#v", fundedBalances)
		}
	}

	emptyAddr := common.HexToAddress("0x1111111111111111111111111111111111111111")
	emptyInput, err := bankprecompile.ABI.Pack(bankprecompile.BalancesMethod, emptyAddr)
	if err != nil {
		t.Fatalf("pack balances(address empty): %v", err)
	}

	emptyResult := mustEthCallPrecompile(t, node.RPCURL(), evmtypes.BankPrecompileAddress, emptyInput)
	var emptyBalances []bankprecompile.Balance
	if err := bankprecompile.ABI.UnpackIntoInterface(&emptyBalances, bankprecompile.BalancesMethod, emptyResult); err != nil {
		t.Fatalf("unpack empty balances: %v", err)
	}
	if len(emptyBalances) != 0 {
		t.Fatalf("expected zero balances for empty account, got %#v", emptyBalances)
	}
}
