package action

import (
	"embed"
	"fmt"

	"cosmossdk.io/core/address"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	sdk "github.com/cosmos/cosmos-sdk/types"

	actionkeeper "github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
)

// ActionPrecompileAddress is the hex address of the action precompile.
const ActionPrecompileAddress = "0x0000000000000000000000000000000000000901"

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

// Precompile defines the action module precompile contract.
type Precompile struct {
	cmn.Precompile
	abi.ABI

	actionKeeper   actionkeeper.Keeper
	actionMsgSvr   actiontypes.MsgServer
	actionQuerySvr actiontypes.QueryServer
	addrCdc        address.Codec
}

// NewPrecompile creates a new action precompile instance.
func NewPrecompile(
	actionKeeper actionkeeper.Keeper,
	bankKeeper cmn.BankKeeper,
	addrCdc address.Codec,
) *Precompile {
	return &Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:           storetypes.KVGasConfig(),
			TransientKVGasConfig:  storetypes.TransientGasConfig(),
			ContractAddress:       common.HexToAddress(ActionPrecompileAddress),
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		ABI:            ABI,
		actionKeeper:   actionKeeper,
		actionMsgSvr:   actionkeeper.NewMsgServerImpl(actionKeeper),
		actionQuerySvr: actionkeeper.NewQueryServerImpl(actionKeeper),
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
	// Type-specific transactions
	case RequestCascadeMethod:
		return p.RequestCascade(ctx, contract, stateDB, method, args)
	case RequestSenseMethod:
		return p.RequestSense(ctx, contract, stateDB, method, args)
	case FinalizeCascadeMethod:
		return p.FinalizeCascade(ctx, contract, stateDB, method, args)
	case FinalizeSenseMethod:
		return p.FinalizeSense(ctx, contract, stateDB, method, args)
	// Generic transaction
	case ApproveActionMethod:
		return p.ApproveAction(ctx, contract, stateDB, method, args)
	// Queries
	case GetActionMethod:
		return p.GetAction(ctx, method, args)
	case GetActionFeeMethod:
		return p.GetActionFee(ctx, method, args)
	case GetActionsByCreatorMethod:
		return p.GetActionsByCreator(ctx, method, args)
	case GetActionsByStateMethod:
		return p.GetActionsByState(ctx, method, args)
	case GetActionsBySuperNodeMethod:
		return p.GetActionsBySuperNode(ctx, method, args)
	case GetParamsMethod:
		return p.GetParams(ctx, method, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}
}

// IsTransaction returns true for state-changing methods.
func (Precompile) IsTransaction(method *abi.Method) bool {
	switch method.Name {
	case RequestCascadeMethod, RequestSenseMethod,
		FinalizeCascadeMethod, FinalizeSenseMethod,
		ApproveActionMethod:
		return true
	default:
		return false
	}
}

// Logger returns a precompile-specific logger.
func (p Precompile) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("evm extension", "action")
}
