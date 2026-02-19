//go:build integration
// +build integration

package precisebank_test

import (
	"context"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
	"time"

	lcfg "github.com/LumeraProtocol/lumera/config"
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	testtext "github.com/LumeraProtocol/lumera/testutil/text"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// TestPreciseBankFractionalBalanceQueryMatrix validates core fractional-balance
// query behavior against live node state.
//
// Matrix:
//  1. Fresh key with no activity reports zero fractional balance.
//  2. Active EVM sender reports fractional balance that matches:
//     a) eth_getBalance modulo conversion-factor
//     b) bank integer balance + precisebank fractional split recomposition.
func testPreciseBankFractionalBalanceQueryMatrix(t *testing.T, node *evmtest.Node) {
	t.Helper()
	evmtest.WaitForBlockNumberAtLeast(t, node.RPCURL(), 1, 20*time.Second)

	untouchedAddr := mustAddKeyAddress(t, node, "precisebank-untouched")
	untouchedFractional := mustQueryPrecisebankFractionalBalance(t, node, untouchedAddr)
	if untouchedFractional.Sign() != 0 {
		t.Fatalf("untouched key fractional balance should be zero, got %s", untouchedFractional.String())
	}

	txHash := evmtest.SendOneLegacyTx(t, node.RPCURL(), node.KeyInfo())
	receipt := evmtest.WaitForReceipt(t, node.RPCURL(), txHash, node.WaitCh(), node.OutputBuffer(), 40*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)

	senderBech32 := node.KeyInfo().Address
	senderHex := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo()).Hex()

	senderInteger := mustQueryBankBalanceDenom(t, node, senderBech32, lcfg.ChainDenom)
	senderFractional := mustQueryPrecisebankFractionalBalance(t, node, senderBech32)
	senderExtended := mustGetEthBalance(t, node.RPCURL(), senderHex)

	cf := conversionFactorBigInt()
	if senderFractional.Sign() < 0 || senderFractional.Cmp(cf) >= 0 {
		t.Fatalf("fractional balance out of range [0, cf): fractional=%s cf=%s", senderFractional.String(), cf.String())
	}

	recomposed := new(big.Int).Mul(new(big.Int).Set(senderInteger), cf)
	recomposed.Add(recomposed, senderFractional)
	if recomposed.Cmp(senderExtended) != 0 {
		t.Fatalf(
			"extended split mismatch: bank*cf+fractional=%s eth_getBalance=%s (bank=%s fractional=%s cf=%s)",
			recomposed.String(),
			senderExtended.String(),
			senderInteger.String(),
			senderFractional.String(),
			cf.String(),
		)
	}

	expectedFractional := new(big.Int).Mod(new(big.Int).Set(senderExtended), cf)
	if expectedFractional.Cmp(senderFractional) != 0 {
		t.Fatalf(
			"fractional query should match eth balance modulo conversion factor: got=%s want=%s",
			senderFractional.String(),
			expectedFractional.String(),
		)
	}
}

// TestPreciseBankRemainderQueryPersistsAcrossRestart verifies remainder query
// shape/range and persistence through node restart.
func TestPreciseBankRemainderQueryPersistsAcrossRestart(t *testing.T) {
	t.Helper()

	node := evmtest.NewEVMNode(t, "lumera-precisebank-remainder-restart", 280)
	node.StartAndWaitRPC()
	defer node.Stop()

	testPreciseBankRemainderQueryPersistsAcrossRestart(t, node)
}

func testPreciseBankRemainderQueryPersistsAcrossRestart(t *testing.T, node *evmtest.Node) {
	t.Helper()
	evmtest.WaitForBlockNumberAtLeast(t, node.RPCURL(), 1, 20*time.Second)

	// Produce at least one EVM fee event before reading remainder.
	txHash := evmtest.SendOneLegacyTx(t, node.RPCURL(), node.KeyInfo())
	receipt := evmtest.WaitForReceipt(t, node.RPCURL(), txHash, node.WaitCh(), node.OutputBuffer(), 40*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)

	before := mustQueryPrecisebankRemainder(t, node)
	cf := conversionFactorBigInt()
	if before.Sign() < 0 || before.Cmp(cf) >= 0 {
		t.Fatalf("remainder out of range [0, cf): remainder=%s cf=%s", before.String(), cf.String())
	}

	node.RestartAndWaitRPC()

	after := mustQueryPrecisebankRemainder(t, node)
	if after.Cmp(before) != 0 {
		t.Fatalf("remainder changed across restart: before=%s after=%s", before.String(), after.String())
	}
}

// TestPreciseBankModuleAccountFractionalBalanceIsZero ensures the reserve module
// account never exposes a fractional balance to consumers.
func TestPreciseBankModuleAccountFractionalBalanceIsZero(t *testing.T) {
	t.Helper()

	node := evmtest.NewEVMNode(t, "lumera-precisebank-module-fractional-zero", 280)
	node.StartAndWaitRPC()
	defer node.Stop()

	testPreciseBankModuleAccountFractionalBalanceIsZero(t, node)
}

func testPreciseBankModuleAccountFractionalBalanceIsZero(t *testing.T, node *evmtest.Node) {
	t.Helper()
	evmtest.WaitForBlockNumberAtLeast(t, node.RPCURL(), 1, 20*time.Second)

	moduleAddr := mustQueryModuleAccountAddress(t, node, precisebanktypes.ModuleName)
	before := mustQueryPrecisebankFractionalBalance(t, node, moduleAddr)
	if before.Sign() != 0 {
		t.Fatalf("precisebank module fractional balance must start at zero, got %s", before.String())
	}

	// Execute EVM activity, then re-check module fractional visibility.
	txHash := evmtest.SendOneLegacyTx(t, node.RPCURL(), node.KeyInfo())
	receipt := evmtest.WaitForReceipt(t, node.RPCURL(), txHash, node.WaitCh(), node.OutputBuffer(), 40*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)

	after := mustQueryPrecisebankFractionalBalance(t, node, moduleAddr)
	if after.Sign() != 0 {
		t.Fatalf("precisebank module fractional balance must remain zero after tx activity, got %s", after.String())
	}

	node.RestartAndWaitRPC()
	afterRestart := mustQueryPrecisebankFractionalBalance(t, node, moduleAddr)
	if afterRestart.Sign() != 0 {
		t.Fatalf("precisebank module fractional balance must remain zero after restart, got %s", afterRestart.String())
	}
}

// TestPreciseBankFractionalBalanceRejectsInvalidAddress validates query input
// handling for malformed bech32 addresses.
func testPreciseBankFractionalBalanceRejectsInvalidAddress(t *testing.T, node *evmtest.Node) {
	t.Helper()
	evmtest.WaitForBlockNumberAtLeast(t, node.RPCURL(), 1, 20*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	out, err := evmtest.RunCommand(
		ctx,
		node.RepoRoot(),
		node.BinPath(),
		"query", "precisebank", "fractional-balance", "not_a_bech32_address",
		"--node", node.CometRPCURL(),
		"--output", "json",
		"--home", node.HomeDir(),
		"--log_no_color",
	)
	if err == nil {
		t.Fatalf("expected invalid address query to fail, got success output:\n%s", out)
	}
	if !testtext.ContainsAny(out, "decoding bech32", "invalid address", "invalid request") {
		t.Fatalf("unexpected invalid-address error output:\n%s", out)
	}
}

// queryCoin matches CLI query coin JSON payloads.
type queryCoin struct {
	Denom  string `json:"denom"`
	Amount string `json:"amount"`
}

// mustAddKeyAddress creates a local test key and returns its bech32 address.
func mustAddKeyAddress(t *testing.T, node *evmtest.Node, keyName string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	out, err := evmtest.RunCommand(
		ctx,
		node.RepoRoot(),
		node.BinPath(),
		"keys", "add", keyName,
		"--home", node.HomeDir(),
		"--keyring-backend", "test",
		"--output", "json",
		"--log_no_color",
	)
	if err != nil {
		t.Fatalf("keys add %s failed: %v\n%s", keyName, err, out)
	}

	var keyInfo testaccounts.TestKeyInfo
	if err := json.Unmarshal([]byte(out), &keyInfo); err != nil {
		t.Fatalf("decode keys add output: %v\n%s", err, out)
	}
	testaccounts.MustNormalizeAndValidateTestKeyInfo(t, &keyInfo)
	return keyInfo.Address
}

// mustQueryPrecisebankFractionalBalance runs `query precisebank fractional-balance`.
func mustQueryPrecisebankFractionalBalance(t *testing.T, node *evmtest.Node, addr string) *big.Int {
	t.Helper()

	out := mustRunNodeQuery(t, node,
		"query", "precisebank", "fractional-balance", addr,
		"--node", node.CometRPCURL(),
		"--output", "json",
		"--home", node.HomeDir(),
		"--log_no_color",
	)

	var resp struct {
		FractionalBalance queryCoin `json:"fractional_balance"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("decode fractional-balance response: %v\n%s", err, out)
	}

	if resp.FractionalBalance.Denom != lcfg.ChainEVMExtendedDenom {
		t.Fatalf("unexpected fractional denom: got %q want %q", resp.FractionalBalance.Denom, lcfg.ChainEVMExtendedDenom)
	}

	amount, ok := new(big.Int).SetString(strings.TrimSpace(resp.FractionalBalance.Amount), 10)
	if !ok {
		t.Fatalf("invalid fractional amount %q", resp.FractionalBalance.Amount)
	}
	return amount
}

// mustQueryPrecisebankRemainder runs `query precisebank remainder`.
func mustQueryPrecisebankRemainder(t *testing.T, node *evmtest.Node) *big.Int {
	t.Helper()

	out := mustRunNodeQuery(t, node,
		"query", "precisebank", "remainder",
		"--node", node.CometRPCURL(),
		"--output", "json",
		"--home", node.HomeDir(),
		"--log_no_color",
	)

	var resp struct {
		Remainder queryCoin `json:"remainder"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("decode remainder response: %v\n%s", err, out)
	}

	if resp.Remainder.Denom != lcfg.ChainEVMExtendedDenom {
		t.Fatalf("unexpected remainder denom: got %q want %q", resp.Remainder.Denom, lcfg.ChainEVMExtendedDenom)
	}

	amount, ok := new(big.Int).SetString(strings.TrimSpace(resp.Remainder.Amount), 10)
	if !ok {
		t.Fatalf("invalid remainder amount %q", resp.Remainder.Amount)
	}
	return amount
}

// mustQueryBankBalanceDenom runs `query bank balance <addr> <denom>`.
func mustQueryBankBalanceDenom(t *testing.T, node *evmtest.Node, addr, denom string) *big.Int {
	t.Helper()

	out := mustRunNodeQuery(t, node,
		"query", "bank", "balance", addr, denom,
		"--node", node.CometRPCURL(),
		"--output", "json",
		"--home", node.HomeDir(),
		"--log_no_color",
	)

	var resp struct {
		Balance queryCoin `json:"balance"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("decode bank balance response: %v\n%s", err, out)
	}

	if resp.Balance.Denom != denom {
		t.Fatalf("unexpected bank denom: got %q want %q", resp.Balance.Denom, denom)
	}

	amount, ok := new(big.Int).SetString(strings.TrimSpace(resp.Balance.Amount), 10)
	if !ok {
		t.Fatalf("invalid bank balance amount %q", resp.Balance.Amount)
	}
	return amount
}

// mustQueryModuleAccountAddress fetches module account bech32 address.
func mustQueryModuleAccountAddress(t *testing.T, node *evmtest.Node, moduleName string) string {
	t.Helper()

	out := mustRunNodeQuery(t, node,
		"query", "auth", "module-account", moduleName,
		"--node", node.CometRPCURL(),
		"--output", "json",
		"--home", node.HomeDir(),
		"--log_no_color",
	)

	var resp map[string]any
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("decode module-account response: %v\n%s", err, out)
	}

	account, ok := resp["account"].(map[string]any)
	if !ok {
		t.Fatalf("module-account response missing account: %#v", resp)
	}

	// Protobuf JSON shape: account.base_account.address
	if baseAccount, ok := account["base_account"].(map[string]any); ok {
		if address, ok := baseAccount["address"].(string); ok && strings.TrimSpace(address) != "" {
			return strings.TrimSpace(address)
		}
	}

	// Legacy Amino JSON shape: account.value.address
	if value, ok := account["value"].(map[string]any); ok {
		if address, ok := value["address"].(string); ok && strings.TrimSpace(address) != "" {
			return strings.TrimSpace(address)
		}
	}

	t.Fatalf("module-account response missing address: %#v", account)
	return ""
}

// mustGetEthBalance reads `eth_getBalance` at latest block.
func mustGetEthBalance(t *testing.T, rpcURL, addressHex string) *big.Int {
	t.Helper()

	var balanceHex string
	evmtest.MustJSONRPC(t, rpcURL, "eth_getBalance", []any{addressHex, "latest"}, &balanceHex)

	balance, err := hexutil.DecodeBig(balanceHex)
	if err != nil {
		t.Fatalf("decode eth_getBalance %q: %v", balanceHex, err)
	}
	return balance
}

// mustRunNodeQuery runs a query command against the running node and returns raw JSON.
func mustRunNodeQuery(t *testing.T, node *evmtest.Node, args ...string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	out, err := evmtest.RunCommand(ctx, node.RepoRoot(), node.BinPath(), args...)
	if err != nil {
		t.Fatalf("query command failed: %v\nargs=%v\n%s", err, args, out)
	}
	return out
}

func conversionFactorBigInt() *big.Int {
	// Lumera uses 6-decimal integer denom (`ulume`) and 18-decimal extended
	// denom (`alume`), so precisebank conversion factor is 10^(18-6) = 1e12.
	//
	// We use a local constant here instead of precisebanktypes.ConversionFactor()
	// because that helper reads process-global EVM coin config, which is not
	// initialized in this test process (the app runs in a separate node process).
	return big.NewInt(1_000_000_000_000)
}
