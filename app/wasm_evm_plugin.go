package app

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	wasmvmtypes "github.com/CosmWasm/wasmvm/v3/types"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/cosmos/evm/x/vm/keeper"
	"github.com/cosmos/evm/x/vm/statedb"

	"github.com/LumeraProtocol/lumera/precompiles/crossruntime"
)

// DefaultCrossRuntimeGasCap is the maximum gas an individual cross-runtime
// EVM call may consume. This prevents a single wasm->EVM call from burning
// the entire block gas limit.
const DefaultCrossRuntimeGasCap uint64 = 3_000_000

// ---------------------------------------------------------------------------
// JSON message/query types for the CosmWasm Custom envelope
// ---------------------------------------------------------------------------

// EVMCustomMsg is the top-level JSON envelope for CosmWasm -> EVM messages.
type EVMCustomMsg struct {
	EVMCall *EVMCallMsg `json:"evm_call,omitempty"`
}

// EVMCallMsg describes a state-changing call to an EVM contract.
type EVMCallMsg struct {
	// Contract is the hex-encoded EVM contract address (e.g. "0x1234...").
	Contract string `json:"contract"`
	// Calldata is the hex-encoded EVM calldata (e.g. "0xa9059cbb...").
	Calldata string `json:"calldata"`
}

// EVMCustomQuery is the top-level JSON envelope for CosmWasm -> EVM queries.
type EVMCustomQuery struct {
	EVMCall    *EVMCallQuery    `json:"evm_call,omitempty"`
	EVMAccount *EVMAccountQuery `json:"evm_account,omitempty"`
}

// EVMCallQuery describes a read-only (eth_call equivalent) EVM query.
type EVMCallQuery struct {
	Contract string `json:"contract"`
	Calldata string `json:"calldata"`
}

// EVMAccountQuery queries EVM account info (balance, nonce, code hash).
type EVMAccountQuery struct {
	Address string `json:"address"`
}

// EVMAccountResponse is returned for evm_account queries.
type EVMAccountResponse struct {
	// Balance is the account balance in the EVM extended denomination (18-dec alume).
	Balance string `json:"balance"`
	// Nonce is the account's transaction count.
	Nonce uint64 `json:"nonce"`
	// IsContract is true if the account has deployed code.
	IsContract bool `json:"is_contract"`
}

// ---------------------------------------------------------------------------
// Gas cap helper
// ---------------------------------------------------------------------------

// gasCapForCall returns the EVM gas limit for a cross-runtime call,
// capped at DefaultCrossRuntimeGasCap or the remaining gas on the meter.
func gasCapForCall(ctx sdk.Context) uint64 {
	meter := ctx.GasMeter()
	remaining := meter.Limit() - meter.GasConsumed()
	if remaining > DefaultCrossRuntimeGasCap {
		return DefaultCrossRuntimeGasCap
	}
	return remaining
}

// parseHexBytes decodes a "0x"-prefixed or plain hex string to bytes.
func parseHexBytes(s string) ([]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	if s == "" {
		return []byte{}, nil
	}
	return hex.DecodeString(s)
}

// parseEVMAddress strictly parses a hex-encoded EVM address, rejecting
// malformed input that common.HexToAddress would silently zero-pad or truncate.
func parseEVMAddress(s string) (common.Address, error) {
	s = strings.TrimPrefix(s, "0x")
	if len(s) != 40 {
		return common.Address{}, fmt.Errorf("invalid EVM address: expected 40 hex chars, got %d", len(s))
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return common.Address{}, fmt.Errorf("invalid EVM address hex: %w", err)
	}
	return common.BytesToAddress(b), nil
}

// ---------------------------------------------------------------------------
// EVM Message Handler (state-changing calls from CosmWasm)
// ---------------------------------------------------------------------------

// evmMessageHandler implements wasmkeeper.Messenger for EVM custom messages.
type evmMessageHandler struct {
	evmKeeper *keeper.Keeper
	next      wasmkeeper.Messenger
}

// NewEVMMessageHandler returns a WithMessageHandlerDecorator function that
// intercepts CosmosMsg.Custom payloads containing evm_call and routes them
// to the EVM keeper. Non-matching messages fall through to the next handler.
func NewEVMMessageHandler(evmKeeper *keeper.Keeper) func(old wasmkeeper.Messenger) wasmkeeper.Messenger {
	return func(old wasmkeeper.Messenger) wasmkeeper.Messenger {
		return &evmMessageHandler{
			evmKeeper: evmKeeper,
			next:      old,
		}
	}
}

func (h *evmMessageHandler) DispatchMsg(
	ctx sdk.Context,
	contractAddr sdk.AccAddress,
	contractIBCPortID string,
	msg wasmvmtypes.CosmosMsg,
) ([]sdk.Event, [][]byte, [][]*codectypes.Any, error) {
	if msg.Custom == nil {
		return h.next.DispatchMsg(ctx, contractAddr, contractIBCPortID, msg)
	}

	var evmMsg EVMCustomMsg
	if err := json.Unmarshal(msg.Custom, &evmMsg); err != nil {
		return h.next.DispatchMsg(ctx, contractAddr, contractIBCPortID, msg)
	}
	if evmMsg.EVMCall == nil {
		return h.next.DispatchMsg(ctx, contractAddr, contractIBCPortID, msg)
	}

	// Check reentrancy guard
	ctx, err := crossruntime.CheckAndIncrementDepth(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	// Convert wasm contract address (the caller) to EVM address
	callerEVMAddr := crossruntime.AccAddrToEVMAddr(contractAddr)

	// Parse target contract address (strict validation)
	targetAddr, err := parseEVMAddress(evmMsg.EVMCall.Contract)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid target contract: %w", err)
	}

	// Decode calldata from hex
	calldata, err := parseHexBytes(evmMsg.EVMCall.Calldata)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid calldata hex: %w", err)
	}

	// Get nonce for the caller account
	acct := h.evmKeeper.GetAccountOrEmpty(ctx, callerEVMAddr)

	// Create stateDB for this call
	txConfig := statedb.NewEmptyTxConfig()
	evmStateDB := statedb.New(ctx, h.evmKeeper, txConfig)

	// Build EVM core message with gas cap
	gasCap := gasCapForCall(ctx)
	evmCoreMsg := core.Message{
		From:       callerEVMAddr,
		To:         &targetAddr,
		Nonce:      acct.Nonce,
		Value:      big.NewInt(0), // Phase 1: non-payable
		GasLimit:   gasCap,
		GasPrice:   big.NewInt(0),
		GasTipCap:  big.NewInt(0),
		GasFeeCap:  big.NewInt(0),
		Data:       calldata,
		AccessList: ethtypes.AccessList{},
	}

	// Execute: commit=true, callFromPrecompile=false, internal=true
	res, err := h.evmKeeper.ApplyMessage(ctx, evmStateDB, evmCoreMsg, nil, true, false, true)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("evm call failed: %w", err)
	}

	// Charge gas (always, even on VM error — gas was consumed)
	if res.GasUsed > 0 {
		ctx.GasMeter().ConsumeGas(res.GasUsed, "wasm->evm call")
	}

	// Check for EVM-level failure (revert, out-of-gas, etc.)
	if res.Failed() {
		if res.VmError != "" {
			return nil, nil, nil, fmt.Errorf("evm execution reverted: %s", res.VmError)
		}
		return nil, nil, nil, fmt.Errorf("evm execution failed")
	}

	return nil, [][]byte{res.Ret}, nil, nil
}

// ---------------------------------------------------------------------------
// EVM Query Handler Decorator (read-only calls from CosmWasm)
// ---------------------------------------------------------------------------

// NewEVMQueryHandlerDecorator returns a WithQueryHandlerDecorator function
// that intercepts QueryRequest.Custom payloads containing evm_call or
// evm_account and routes them to the EVM keeper. Non-matching queries
// fall through to the wrapped handler.
func NewEVMQueryHandlerDecorator(evmKeeper *keeper.Keeper) func(old wasmkeeper.WasmVMQueryHandler) wasmkeeper.WasmVMQueryHandler {
	return func(old wasmkeeper.WasmVMQueryHandler) wasmkeeper.WasmVMQueryHandler {
		return wasmkeeper.WasmVMQueryHandlerFn(
			func(ctx sdk.Context, caller sdk.AccAddress, request wasmvmtypes.QueryRequest) ([]byte, error) {
				if request.Custom == nil {
					return old.HandleQuery(ctx, caller, request)
				}

				var evmQuery EVMCustomQuery
				if err := json.Unmarshal(request.Custom, &evmQuery); err != nil {
					return old.HandleQuery(ctx, caller, request)
				}

				switch {
				case evmQuery.EVMCall != nil:
					// Check reentrancy guard (queries count toward cross-runtime depth)
					ctx, err := crossruntime.CheckAndIncrementDepth(ctx)
					if err != nil {
						return nil, err
					}
					return handleEVMCallQuery(ctx, evmKeeper, caller, evmQuery.EVMCall)
				case evmQuery.EVMAccount != nil:
					// evm_account is a simple state read, no EVM execution — no reentrancy risk.
					// But still enforce the guard for consistency with the "max depth = 1" design.
					ctx, err := crossruntime.CheckAndIncrementDepth(ctx)
					if err != nil {
						return nil, err
					}
					return handleEVMAccountQuery(ctx, evmKeeper, evmQuery.EVMAccount)
				default:
					return old.HandleQuery(ctx, caller, request)
				}
			})
	}
}

// handleEVMCallQuery performs a read-only eth_call equivalent.
func handleEVMCallQuery(
	ctx sdk.Context,
	evmKeeper *keeper.Keeper,
	caller sdk.AccAddress,
	q *EVMCallQuery,
) ([]byte, error) {
	callerEVMAddr := crossruntime.AccAddrToEVMAddr(caller)
	targetAddr, err := parseEVMAddress(q.Contract)
	if err != nil {
		return nil, fmt.Errorf("invalid target contract: %w", err)
	}

	calldata, err := parseHexBytes(q.Calldata)
	if err != nil {
		return nil, fmt.Errorf("invalid calldata hex: %w", err)
	}

	acct := evmKeeper.GetAccountOrEmpty(ctx, callerEVMAddr)

	txConfig := statedb.NewEmptyTxConfig()
	evmStateDB := statedb.New(ctx, evmKeeper, txConfig)

	gasCap := gasCapForCall(ctx)
	evmCoreMsg := core.Message{
		From:       callerEVMAddr,
		To:         &targetAddr,
		Nonce:      acct.Nonce,
		Value:      big.NewInt(0),
		GasLimit:   gasCap,
		GasPrice:   big.NewInt(0),
		GasTipCap:  big.NewInt(0),
		GasFeeCap:  big.NewInt(0),
		Data:       calldata,
		AccessList: ethtypes.AccessList{},
	}

	// Read-only: commit=false, callFromPrecompile=false, internal=true
	res, err := evmKeeper.ApplyMessage(ctx, evmStateDB, evmCoreMsg, nil, false, false, true)
	if err != nil {
		return nil, fmt.Errorf("evm query failed: %w", err)
	}

	// Charge EVM gas back to the wasm query gas meter
	if res.GasUsed > 0 {
		ctx.GasMeter().ConsumeGas(res.GasUsed, "wasm->evm query")
	}

	if res.Failed() {
		if res.VmError != "" {
			return nil, fmt.Errorf("evm query reverted: %s", res.VmError)
		}
		return nil, fmt.Errorf("evm query failed")
	}

	// Return hex-encoded response as JSON string
	result := fmt.Sprintf(`{"result":"0x%s"}`, hex.EncodeToString(res.Ret))
	return []byte(result), nil
}

// handleEVMAccountQuery returns account info for an EVM address.
func handleEVMAccountQuery(
	ctx sdk.Context,
	evmKeeper *keeper.Keeper,
	q *EVMAccountQuery,
) ([]byte, error) {
	addr, err := parseEVMAddress(q.Address)
	if err != nil {
		return nil, fmt.Errorf("invalid account address: %w", err)
	}
	acct := evmKeeper.GetAccountOrEmpty(ctx, addr)

	isContract := len(acct.CodeHash) > 0 && common.BytesToHash(acct.CodeHash) != common.BytesToHash(emptyCodeHash)

	resp := EVMAccountResponse{
		Balance:    acct.Balance.ToBig().String(),
		Nonce:      acct.Nonce,
		IsContract: isContract,
	}

	return json.Marshal(resp)
}

// emptyCodeHash is the keccak256 of empty bytes — accounts without code have this hash.
var emptyCodeHash = common.FromHex("c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470")

// ---------------------------------------------------------------------------
// Wasm keeper option builders
// ---------------------------------------------------------------------------

// EVMWasmPluginOpts returns wasmkeeper.Option values that wire the EVM
// message handler and query handler decorator into the wasm keeper.
// These must be appended to wasmOpts BEFORE the wasm keeper is created.
func EVMWasmPluginOpts(evmKeeper *keeper.Keeper) []wasmkeeper.Option {
	return []wasmkeeper.Option{
		wasmkeeper.WithMessageHandlerDecorator(
			NewEVMMessageHandler(evmKeeper),
		),
		wasmkeeper.WithQueryHandlerDecorator(
			NewEVMQueryHandlerDecorator(evmKeeper),
		),
	}
}

// Ensure evmMessageHandler satisfies the Messenger interface at compile time.
var _ wasmkeeper.Messenger = (*evmMessageHandler)(nil)

// Suppress unused import lint for wasmtypes (used for ErrUnknownMsg awareness).
var _ = wasmtypes.ErrUnknownMsg
