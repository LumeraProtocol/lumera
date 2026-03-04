//go:build integration
// +build integration

package vm_test

import (
	"strings"
	"testing"
	"time"

	lcfg "github.com/LumeraProtocol/lumera/config"
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
)

type bankBalanceQueryResponse struct {
	Balance struct {
		Denom  string `json:"denom"`
		Amount string `json:"amount"`
	} `json:"balance"`
}

// TestVMQueryAccountAcceptsHexAndBech32 ensures VM account query accepts both
// address formats and returns the same account snapshot.
func testVMQueryAccountAcceptsHexAndBech32(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	bech32Addr := node.KeyInfo().Address
	hexAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo()).Hex()

	byBech32 := mustQueryEVMAccount(t, node, bech32Addr, 0)
	byHex := mustQueryEVMAccount(t, node, hexAddr, 0)

	if strings.TrimSpace(byBech32.Balance) != strings.TrimSpace(byHex.Balance) {
		t.Fatalf("account balance mismatch by address format: bech32=%s hex=%s", byBech32.Balance, byHex.Balance)
	}
	if strings.TrimSpace(byBech32.Nonce) != strings.TrimSpace(byHex.Nonce) {
		t.Fatalf("account nonce mismatch by address format: bech32=%s hex=%s", byBech32.Nonce, byHex.Nonce)
	}
	if !strings.EqualFold(byBech32.CodeHash, byHex.CodeHash) {
		t.Fatalf("account code_hash mismatch by address format: bech32=%s hex=%s", byBech32.CodeHash, byHex.CodeHash)
	}
}

// TestVMBalanceBankMatchesBankQuery verifies `query evm balance-bank` returns
// the same coin amount as the canonical bank query for the same account/denom.
func testVMBalanceBankMatchesBankQuery(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	bech32Addr := node.KeyInfo().Address
	hexAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo()).Hex()

	evmOut := mustRunNodeCommand(t, node,
		"query", "evm", "balance-bank", hexAddr, lcfg.ChainDenom,
		"--node", node.CometRPCURL(),
		"--output", "json",
		"--home", node.HomeDir(),
		"--log_no_color",
	)
	var evmResp bankBalanceQueryResponse
	if err := decodeCLIJSON(evmOut, &evmResp); err != nil {
		t.Fatalf("decode query evm balance-bank response: %v\n%s", err, evmOut)
	}

	bankOut := mustRunNodeCommand(t, node,
		"query", "bank", "balance", bech32Addr, lcfg.ChainDenom,
		"--node", node.CometRPCURL(),
		"--output", "json",
		"--home", node.HomeDir(),
		"--log_no_color",
	)
	var bankResp bankBalanceQueryResponse
	if err := decodeCLIJSON(bankOut, &bankResp); err != nil {
		t.Fatalf("decode query bank balance response: %v\n%s", err, bankOut)
	}

	if evmResp.Balance.Denom != lcfg.ChainDenom {
		t.Fatalf("unexpected evm balance denom: got=%s want=%s", evmResp.Balance.Denom, lcfg.ChainDenom)
	}
	if bankResp.Balance.Denom != lcfg.ChainDenom {
		t.Fatalf("unexpected bank balance denom: got=%s want=%s", bankResp.Balance.Denom, lcfg.ChainDenom)
	}
	if strings.TrimSpace(evmResp.Balance.Amount) != strings.TrimSpace(bankResp.Balance.Amount) {
		t.Fatalf("balance-bank mismatch with bank query: evm=%s bank=%s", evmResp.Balance.Amount, bankResp.Balance.Amount)
	}
}

// TestVMStorageQueryKeyFormatEquivalence verifies storage slot key input
// normalization by querying the same slot using short and full hex keys.
func testVMStorageQueryKeyFormatEquivalence(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	deployTxHash := sendContractCreationTx(t, node, storageSetterContractCreationCode())
	deployReceipt := node.WaitForReceipt(t, deployTxHash, 45*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, deployReceipt, deployTxHash)

	contractAddress := evmtest.MustStringField(t, deployReceipt, "contractAddress")
	if strings.EqualFold(contractAddress, "0x0000000000000000000000000000000000000000") {
		t.Fatalf("unexpected zero contractAddress in deployment receipt: %#v", deployReceipt)
	}

	callTxHash := sendContractMethodTx(t, node, contractAddress, "0x")
	callReceipt := node.WaitForReceipt(t, callTxHash, 45*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, callReceipt, callTxHash)

	gotShort := mustQueryEVMStorage(t, node, contractAddress, "0x0", 0)
	gotPadded := mustQueryEVMStorage(t, node, contractAddress, "0x00", 0)
	gotFull := mustQueryEVMStorage(t, node, contractAddress, "0x0000000000000000000000000000000000000000000000000000000000000000", 0)

	if !strings.EqualFold(gotShort, gotPadded) || !strings.EqualFold(gotShort, gotFull) {
		t.Fatalf("storage slot mismatch by key format: short=%s padded=%s full=%s", gotShort, gotPadded, gotFull)
	}

	wantSlot0 := "0x" + strings.Repeat("0", 62) + "2a"
	if !strings.EqualFold(gotShort, wantSlot0) {
		t.Fatalf("unexpected slot0 value: got=%s want=%s", gotShort, wantSlot0)
	}
}
