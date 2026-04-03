package app

import (
	"encoding/json"
	"errors"
	"testing"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmvmtypes "github.com/CosmWasm/wasmvm/v3/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/LumeraProtocol/lumera/precompiles/crossruntime"
)

// Compile-time interface checks for test mocks.
var _ wasmkeeper.Messenger = (*mockMessenger)(nil)
var _ wasmkeeper.WasmVMQueryHandler = (*mockQueryHandler)(nil)

// ---------------------------------------------------------------------------
// parseEVMAddress
// ---------------------------------------------------------------------------

func TestParseEVMAddress_Valid(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  common.Address
	}{
		{"with 0x prefix", "0x1234567890abcdef1234567890abcdef12345678",
			common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")},
		{"without 0x prefix", "1234567890abcdef1234567890abcdef12345678",
			common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")},
		{"uppercase hex", "0xABCDEF0123456789ABCDEF0123456789ABCDEF01",
			common.HexToAddress("0xABCDEF0123456789ABCDEF0123456789ABCDEF01")},
		{"zero address", "0x0000000000000000000000000000000000000000",
			common.Address{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseEVMAddress(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %s, want %s", got.Hex(), tc.want.Hex())
			}
		})
	}
}

func TestParseEVMAddress_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"too short", "0x1234"},
		{"too long", "0x1234567890abcdef1234567890abcdef1234567890"},
		{"empty", ""},
		{"only prefix", "0x"},
		{"invalid hex chars", "0xZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ"},
		{"39 chars", "0x234567890abcdef1234567890abcdef12345678"},
		{"41 chars", "0x01234567890abcdef1234567890abcdef12345678"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseEVMAddress(tc.input)
			if err == nil {
				t.Fatalf("expected error for input %q, got nil", tc.input)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseHexBytes
// ---------------------------------------------------------------------------

func TestParseHexBytes_Valid(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int // expected byte length
	}{
		{"with 0x prefix", "0xa9059cbb", 4},
		{"without prefix", "a9059cbb", 4},
		{"empty with 0x", "0x", 0},
		{"empty string", "", 0},
		{"long calldata", "0xa9059cbb0000000000000000000000001234567890abcdef1234567890abcdef12345678", 36},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseHexBytes(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tc.want {
				t.Fatalf("got %d bytes, want %d", len(got), tc.want)
			}
		})
	}
}

func TestParseHexBytes_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"invalid hex chars", "0xZZZZ"},
		{"odd length", "0xabc"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseHexBytes(tc.input)
			if err == nil {
				t.Fatalf("expected error for input %q, got nil", tc.input)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// gasCapForCall
// ---------------------------------------------------------------------------

func testCtxWithGas(limit, consumed uint64) sdk.Context {
	ctx := sdk.NewContext(nil, tmproto.Header{}, false, log.NewNopLogger())
	gasMeter := storetypes.NewGasMeter(limit)
	gasMeter.ConsumeGas(consumed, "test setup")
	return ctx.WithGasMeter(gasMeter)
}

func TestGasCapForCall_UnderCap(t *testing.T) {
	// Remaining gas (500k) < DefaultCrossRuntimeGasCap (3M)
	ctx := testCtxWithGas(1_000_000, 500_000) // remaining = 500k
	cap := gasCapForCall(ctx)
	if cap != 500_000 {
		t.Fatalf("expected 500000 (remaining), got %d", cap)
	}
}

func TestGasCapForCall_OverCap(t *testing.T) {
	// Remaining gas (9M) > DefaultCrossRuntimeGasCap (3M)
	ctx := testCtxWithGas(10_000_000, 1_000_000) // remaining = 9M
	cap := gasCapForCall(ctx)
	if cap != DefaultCrossRuntimeGasCap {
		t.Fatalf("expected %d (cap), got %d", DefaultCrossRuntimeGasCap, cap)
	}
}

func TestGasCapForCall_ExactlyCap(t *testing.T) {
	// Remaining gas exactly equals DefaultCrossRuntimeGasCap
	ctx := testCtxWithGas(DefaultCrossRuntimeGasCap, 0)
	cap := gasCapForCall(ctx)
	if cap != DefaultCrossRuntimeGasCap {
		t.Fatalf("expected %d (cap), got %d", DefaultCrossRuntimeGasCap, cap)
	}
}

func TestGasCapForCall_ZeroRemaining(t *testing.T) {
	ctx := testCtxWithGas(1_000_000, 1_000_000) // remaining = 0
	cap := gasCapForCall(ctx)
	if cap != 0 {
		t.Fatalf("expected 0 (no gas left), got %d", cap)
	}
}

func TestDefaultCrossRuntimeGasCap_Value(t *testing.T) {
	if DefaultCrossRuntimeGasCap != 3_000_000 {
		t.Fatalf("expected 3000000, got %d", DefaultCrossRuntimeGasCap)
	}
}

// ---------------------------------------------------------------------------
// Mock messenger and query handler for handler dispatch tests
// ---------------------------------------------------------------------------

// mockMessenger records whether DispatchMsg was called (passthrough test).
type mockMessenger struct {
	called bool
}

func (m *mockMessenger) DispatchMsg(
	_ sdk.Context, _ sdk.AccAddress, _ string, _ wasmvmtypes.CosmosMsg,
) ([]sdk.Event, [][]byte, [][]*codectypes.Any, error) {
	m.called = true
	return nil, nil, nil, nil
}

// mockQueryHandler records whether HandleQuery was called (passthrough test).
type mockQueryHandler struct {
	called bool
}

func (m *mockQueryHandler) HandleQuery(_ sdk.Context, _ sdk.AccAddress, _ wasmvmtypes.QueryRequest) ([]byte, error) {
	m.called = true
	return []byte(`{"ok":true}`), nil
}

func freshSDKCtx() sdk.Context {
	ctx := sdk.NewContext(nil, tmproto.Header{}, false, log.NewNopLogger())
	return ctx.WithGasMeter(storetypes.NewGasMeter(10_000_000))
}

// ---------------------------------------------------------------------------
// Message handler: passthrough tests
// ---------------------------------------------------------------------------

func TestEVMMessageHandler_NilCustomPassesThrough(t *testing.T) {
	mock := &mockMessenger{}
	handler := &evmMessageHandler{evmKeeper: nil, next: mock}

	ctx := freshSDKCtx()
	msg := wasmvmtypes.CosmosMsg{} // Custom is nil
	_, _, _, err := handler.DispatchMsg(ctx, sdk.AccAddress{}, "", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mock.called {
		t.Fatal("expected passthrough to next handler when Custom is nil")
	}
}

func TestEVMMessageHandler_NonEVMCustomPassesThrough(t *testing.T) {
	mock := &mockMessenger{}
	handler := &evmMessageHandler{evmKeeper: nil, next: mock}

	ctx := freshSDKCtx()
	// Custom JSON that is not an evm_call
	msg := wasmvmtypes.CosmosMsg{
		Custom: json.RawMessage(`{"some_other_module":{"action":"do_something"}}`),
	}
	_, _, _, err := handler.DispatchMsg(ctx, sdk.AccAddress{}, "", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mock.called {
		t.Fatal("expected passthrough for non-EVM custom message")
	}
}

func TestEVMMessageHandler_MalformedJSONPassesThrough(t *testing.T) {
	mock := &mockMessenger{}
	handler := &evmMessageHandler{evmKeeper: nil, next: mock}

	ctx := freshSDKCtx()
	msg := wasmvmtypes.CosmosMsg{
		Custom: json.RawMessage(`{not valid json`),
	}
	_, _, _, err := handler.DispatchMsg(ctx, sdk.AccAddress{}, "", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mock.called {
		t.Fatal("expected passthrough for malformed JSON")
	}
}

func TestEVMMessageHandler_EVMCallNilPassesThrough(t *testing.T) {
	mock := &mockMessenger{}
	handler := &evmMessageHandler{evmKeeper: nil, next: mock}

	ctx := freshSDKCtx()
	// Envelope present but evm_call is null
	msg := wasmvmtypes.CosmosMsg{
		Custom: json.RawMessage(`{"evm_call":null}`),
	}
	_, _, _, err := handler.DispatchMsg(ctx, sdk.AccAddress{}, "", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mock.called {
		t.Fatal("expected passthrough when evm_call is null")
	}
}

// ---------------------------------------------------------------------------
// Message handler: reentrancy guard
// ---------------------------------------------------------------------------

func TestEVMMessageHandler_ReentrancyBlocked(t *testing.T) {
	handler := &evmMessageHandler{evmKeeper: nil, next: &mockMessenger{}}

	// Set depth to max (simulating we're already inside a cross-runtime call)
	ctx := freshSDKCtx()
	ctx = crossruntime.WithIncrementedDepth(ctx)

	msg := wasmvmtypes.CosmosMsg{
		Custom: json.RawMessage(`{"evm_call":{"contract":"0x1234567890abcdef1234567890abcdef12345678","calldata":"0x00"}}`),
	}
	_, _, _, err := handler.DispatchMsg(ctx, sdk.AccAddress(make([]byte, 20)), "", msg)
	if err == nil {
		t.Fatal("expected reentrancy error, got nil")
	}
	if !errors.Is(err, crossruntime.ErrReentrancyNotAllowed) {
		t.Fatalf("expected ErrReentrancyNotAllowed, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Message handler: address validation
// ---------------------------------------------------------------------------

func TestEVMMessageHandler_InvalidContractAddress(t *testing.T) {
	handler := &evmMessageHandler{evmKeeper: nil, next: &mockMessenger{}}

	ctx := freshSDKCtx()
	msg := wasmvmtypes.CosmosMsg{
		Custom: json.RawMessage(`{"evm_call":{"contract":"0xSHORT","calldata":"0x00"}}`),
	}
	_, _, _, err := handler.DispatchMsg(ctx, sdk.AccAddress(make([]byte, 20)), "", msg)
	if err == nil {
		t.Fatal("expected error for invalid EVM address, got nil")
	}
	if !containsString(err.Error(), "invalid target contract") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestEVMMessageHandler_InvalidCalldataHex(t *testing.T) {
	handler := &evmMessageHandler{evmKeeper: nil, next: &mockMessenger{}}

	ctx := freshSDKCtx()
	msg := wasmvmtypes.CosmosMsg{
		Custom: json.RawMessage(`{"evm_call":{"contract":"0x1234567890abcdef1234567890abcdef12345678","calldata":"0xZZZZ"}}`),
	}
	_, _, _, err := handler.DispatchMsg(ctx, sdk.AccAddress(make([]byte, 20)), "", msg)
	if err == nil {
		t.Fatal("expected error for invalid calldata hex, got nil")
	}
	if !containsString(err.Error(), "invalid calldata hex") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Query handler: passthrough tests
// ---------------------------------------------------------------------------

func TestEVMQueryHandler_NilCustomPassesThrough(t *testing.T) {
	mock := &mockQueryHandler{}
	decorator := NewEVMQueryHandlerDecorator(nil)
	handler := decorator(mock)

	ctx := freshSDKCtx()
	req := wasmvmtypes.QueryRequest{} // Custom is nil
	result, err := handler.HandleQuery(ctx, sdk.AccAddress{}, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mock.called {
		t.Fatal("expected passthrough for nil Custom")
	}
	if string(result) != `{"ok":true}` {
		t.Fatalf("unexpected result: %s", string(result))
	}
}

func TestEVMQueryHandler_NonEVMCustomPassesThrough(t *testing.T) {
	mock := &mockQueryHandler{}
	decorator := NewEVMQueryHandlerDecorator(nil)
	handler := decorator(mock)

	ctx := freshSDKCtx()
	req := wasmvmtypes.QueryRequest{
		Custom: json.RawMessage(`{"some_module":{"key":"value"}}`),
	}
	result, err := handler.HandleQuery(ctx, sdk.AccAddress{}, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mock.called {
		t.Fatal("expected passthrough for non-EVM custom query")
	}
	_ = result
}

func TestEVMQueryHandler_MalformedJSONPassesThrough(t *testing.T) {
	mock := &mockQueryHandler{}
	decorator := NewEVMQueryHandlerDecorator(nil)
	handler := decorator(mock)

	ctx := freshSDKCtx()
	req := wasmvmtypes.QueryRequest{
		Custom: json.RawMessage(`{broken json`),
	}
	_, err := handler.HandleQuery(ctx, sdk.AccAddress{}, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mock.called {
		t.Fatal("expected passthrough for malformed JSON")
	}
}

// ---------------------------------------------------------------------------
// Query handler: reentrancy guard
// ---------------------------------------------------------------------------

func TestEVMQueryHandler_EVMCallReentrancyBlocked(t *testing.T) {
	decorator := NewEVMQueryHandlerDecorator(nil)
	handler := decorator(&mockQueryHandler{})

	ctx := freshSDKCtx()
	ctx = crossruntime.WithIncrementedDepth(ctx)

	req := wasmvmtypes.QueryRequest{
		Custom: json.RawMessage(`{"evm_call":{"contract":"0x1234567890abcdef1234567890abcdef12345678","calldata":"0x00"}}`),
	}
	_, err := handler.HandleQuery(ctx, sdk.AccAddress(make([]byte, 20)), req)
	if err == nil {
		t.Fatal("expected reentrancy error for evm_call query, got nil")
	}
	if !errors.Is(err, crossruntime.ErrReentrancyNotAllowed) {
		t.Fatalf("expected ErrReentrancyNotAllowed, got: %v", err)
	}
}

func TestEVMQueryHandler_EVMAccountReentrancyBlocked(t *testing.T) {
	decorator := NewEVMQueryHandlerDecorator(nil)
	handler := decorator(&mockQueryHandler{})

	ctx := freshSDKCtx()
	ctx = crossruntime.WithIncrementedDepth(ctx)

	req := wasmvmtypes.QueryRequest{
		Custom: json.RawMessage(`{"evm_account":{"address":"0x1234567890abcdef1234567890abcdef12345678"}}`),
	}
	_, err := handler.HandleQuery(ctx, sdk.AccAddress(make([]byte, 20)), req)
	if err == nil {
		t.Fatal("expected reentrancy error for evm_account query, got nil")
	}
	if !errors.Is(err, crossruntime.ErrReentrancyNotAllowed) {
		t.Fatalf("expected ErrReentrancyNotAllowed, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle ||
		len(needle) == 0 ||
		(len(haystack) > 0 && searchSubstring(haystack, needle)))
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
