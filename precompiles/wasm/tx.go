package wasm

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/precompiles/crossruntime"
)

// executeWasm executes a CosmWasm contract from EVM. Non-payable in Phase 1.
func (p Precompile) executeWasm(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("execute: expected 2 args, got %d", len(args))
	}

	contractAddr := args[0].(string)
	msgBytes := args[1].([]byte)

	// Check reentrancy guard
	ctx, err := crossruntime.CheckAndIncrementDepth(ctx)
	if err != nil {
		return nil, err
	}

	// Convert EVM caller to sdk.AccAddress (20-byte truncation, matching EVM address derivation)
	callerAddr := sdk.AccAddress(contract.Caller().Bytes())

	// Convert bech32 contract address to sdk.AccAddress
	wasmAddr, err := sdk.AccAddressFromBech32(contractAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid wasm contract address %q: %w", contractAddr, err)
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"caller", callerAddr.String(),
		"wasm_contract", contractAddr,
	)

	// Execute the CosmWasm contract (non-payable: no funds)
	resp, err := p.wasmPermKeeper.Execute(ctx, wasmAddr, callerAddr, msgBytes, sdk.Coins{})
	if err != nil {
		return nil, fmt.Errorf("wasm execute failed: %w", err)
	}

	// Emit WasmExecuted event
	if err := p.EmitWasmExecuted(ctx, stateDB, contract.Caller(), contractAddr, resp); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(resp)
}
