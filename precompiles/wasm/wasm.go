package wasm

import (
	"embed"
	"fmt"

	"cosmossdk.io/core/address"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ vm.PrecompiledContract = &Precompile{}

var (
	//go:embed abi.json
	f   embed.FS
	ABI abi.ABI
)

func init() {
	var err error
	ABI, err = cmn.LoadABI(f, "abi.json")
	if err != nil {
		panic(err)
	}
}

// Precompile defines the CosmWasm precompile contract.
type Precompile struct {
	cmn.Precompile
	abi.ABI

	wasmKeeper     *wasmkeeper.Keeper
	wasmPermKeeper *wasmkeeper.PermissionedKeeper
	addrCdc        address.Codec
}

// NewPrecompile creates a new CosmWasm precompile instance.
func NewPrecompile(
	wasmKeeper *wasmkeeper.Keeper,
	bankKeeper cmn.BankKeeper,
	addrCdc address.Codec,
) *Precompile {
	permKeeper := wasmkeeper.NewDefaultPermissionKeeper(wasmKeeper)
	return &Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:           storetypes.KVGasConfig(),
			TransientKVGasConfig:  storetypes.TransientGasConfig(),
			ContractAddress:       common.HexToAddress(WasmPrecompileAddress),
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		ABI:            ABI,
		wasmKeeper:     wasmKeeper,
		wasmPermKeeper: permKeeper,
		addrCdc:        addrCdc,
	}
}

// RequiredGas returns the minimum gas needed to execute this precompile.
func (p Precompile) RequiredGas(input []byte) uint64 {
	if len(input) < 4 {
		return 0
	}

	method, err := p.MethodById(input[:4])
	if err != nil {
		return 0
	}

	return p.Precompile.RequiredGas(input, p.IsTransaction(method))
}

// Run delegates to RunNativeAction for snapshot/revert management.
func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	return p.RunNativeAction(evm, contract, func(ctx sdk.Context) ([]byte, error) {
		return p.Execute(ctx, evm.StateDB, contract, readonly)
	})
}

// Execute dispatches to the appropriate handler based on the ABI method.
func (p Precompile) Execute(ctx sdk.Context, stateDB vm.StateDB, contract *vm.Contract, readOnly bool) ([]byte, error) {
	method, args, err := cmn.SetupABI(p.ABI, contract, readOnly, p.IsTransaction)
	if err != nil {
		return nil, err
	}

	switch method.Name {
	// State-changing
	case ExecuteMethod:
		return p.executeWasm(ctx, contract, stateDB, method, args)
	// Queries
	case QueryMethod:
		return p.queryWasm(ctx, method, args)
	case ContractInfoMethod:
		return p.contractInfoWasm(ctx, method, args)
	case RawQueryMethod:
		return p.rawQueryWasm(ctx, method, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}
}

// IsTransaction returns true for state-changing methods.
func (Precompile) IsTransaction(method *abi.Method) bool {
	switch method.Name {
	case ExecuteMethod:
		return true
	default:
		return false
	}
}

// Logger returns a precompile-specific logger.
func (p Precompile) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("evm extension", "wasm")
}
