package wasm

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/precompiles/crossruntime"
)

// queryWasm queries a CosmWasm contract's smart query endpoint (read-only).
func (p Precompile) queryWasm(
	ctx sdk.Context,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("query: expected 2 args, got %d", len(args))
	}

	// Check reentrancy guard (queries count toward cross-runtime depth)
	ctx, err := crossruntime.CheckAndIncrementDepth(ctx)
	if err != nil {
		return nil, err
	}

	contractAddr := args[0].(string)
	msgBytes := args[1].([]byte)

	wasmAddr, err := sdk.AccAddressFromBech32(contractAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid wasm contract address %q: %w", contractAddr, err)
	}

	p.Logger(ctx).Debug(
		"query called",
		"method", method.Name,
		"wasm_contract", contractAddr,
	)

	resp, err := p.wasmKeeper.QuerySmart(ctx, wasmAddr, msgBytes)
	if err != nil {
		return nil, fmt.Errorf("wasm query failed: %w", err)
	}

	return method.Outputs.Pack(resp)
}

// contractInfoWasm returns metadata about a CosmWasm contract.
func (p Precompile) contractInfoWasm(
	ctx sdk.Context,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("contractInfo: expected 1 arg, got %d", len(args))
	}

	// Check reentrancy guard (queries count toward cross-runtime depth)
	ctx, err := crossruntime.CheckAndIncrementDepth(ctx)
	if err != nil {
		return nil, err
	}

	contractAddr := args[0].(string)

	wasmAddr, err := sdk.AccAddressFromBech32(contractAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid wasm contract address %q: %w", contractAddr, err)
	}

	info := p.wasmKeeper.GetContractInfo(ctx, wasmAddr)
	if info == nil {
		return nil, fmt.Errorf("contract not found: %s", contractAddr)
	}

	p.Logger(ctx).Debug(
		"query called",
		"method", method.Name,
		"wasm_contract", contractAddr,
		"code_id", info.CodeID,
	)

	return method.Outputs.Pack(info.CodeID, info.Creator, info.Admin, info.Label)
}

// rawQueryWasm queries a raw storage key from a CosmWasm contract.
func (p Precompile) rawQueryWasm(
	ctx sdk.Context,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("rawQuery: expected 2 args, got %d", len(args))
	}

	// Check reentrancy guard (queries count toward cross-runtime depth)
	ctx, err := crossruntime.CheckAndIncrementDepth(ctx)
	if err != nil {
		return nil, err
	}

	contractAddr := args[0].(string)
	key := args[1].([]byte)

	wasmAddr, err := sdk.AccAddressFromBech32(contractAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid wasm contract address %q: %w", contractAddr, err)
	}

	p.Logger(ctx).Debug(
		"query called",
		"method", method.Name,
		"wasm_contract", contractAddr,
	)

	value := p.wasmKeeper.QueryRaw(ctx, wasmAddr, key)

	return method.Outputs.Pack(value)
}
