//go:build integration
// +build integration

package precompiles_test

import (
	"strings"
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	bech32precompile "github.com/cosmos/evm/precompiles/bech32"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
)

// TestBech32PrecompileRoundTripViaEthCall verifies static bech32 precompile
// conversion methods via JSON-RPC eth_call.
//
// Workflow:
// 1. Convert validator account hex -> bech32 using precompile call.
// 2. Convert the returned bech32 -> hex via precompile call.
// 3. Assert both directions preserve the same address.
func testBech32PrecompileRoundTripViaEthCall(t *testing.T, node *evmtest.Node) {
	t.Helper()
	evmtest.WaitForBlockNumberAtLeast(t, node.RPCURL(), 1, 20*time.Second)

	accHex := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	bech32Addr := node.KeyInfo().Address
	bech32Prefix := strings.SplitN(bech32Addr, "1", 2)[0]

	hexToBech32Input, err := bech32precompile.ABI.Pack(
		bech32precompile.HexToBech32Method,
		accHex,
		bech32Prefix,
	)
	if err != nil {
		t.Fatalf("pack hexToBech32 input: %v", err)
	}

	hexToBech32Result := mustEthCallPrecompile(t, node.RPCURL(), evmtypes.Bech32PrecompileAddress, hexToBech32Input)
	out, err := bech32precompile.ABI.Unpack(bech32precompile.HexToBech32Method, hexToBech32Result)
	if err != nil {
		t.Fatalf("unpack hexToBech32 output: %v", err)
	}
	gotBech32, ok := out[0].(string)
	if !ok {
		t.Fatalf("unexpected hexToBech32 output type: %#v", out)
	}
	if gotBech32 != bech32Addr {
		t.Fatalf("hexToBech32 mismatch: got=%q want=%q", gotBech32, bech32Addr)
	}

	bech32ToHexInput, err := bech32precompile.ABI.Pack(bech32precompile.Bech32ToHexMethod, gotBech32)
	if err != nil {
		t.Fatalf("pack bech32ToHex input: %v", err)
	}

	bech32ToHexResult := mustEthCallPrecompile(t, node.RPCURL(), evmtypes.Bech32PrecompileAddress, bech32ToHexInput)
	out, err = bech32precompile.ABI.Unpack(bech32precompile.Bech32ToHexMethod, bech32ToHexResult)
	if err != nil {
		t.Fatalf("unpack bech32ToHex output: %v", err)
	}
	gotHex, ok := out[0].(common.Address)
	if !ok {
		t.Fatalf("unexpected bech32ToHex output type: %#v", out)
	}
	if gotHex != accHex {
		t.Fatalf("bech32ToHex mismatch: got=%s want=%s", gotHex.Hex(), accHex.Hex())
	}
}
