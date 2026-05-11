package supernode

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

	snkeeper "github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// SupernodePrecompileAddress is the hex address of the supernode precompile.
const SupernodePrecompileAddress = "0x0000000000000000000000000000000000000902"

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

// Precompile defines the supernode module precompile contract.
type Precompile struct {
	cmn.Precompile
	abi.ABI

	snKeeper   sntypes.SupernodeKeeper
	snMsgSvr   sntypes.MsgServer
	snQuerySvr sntypes.QueryServer
	addrCdc    address.Codec
}

// NewPrecompile creates a new supernode precompile instance.
func NewPrecompile(
	snKeeper sntypes.SupernodeKeeper,
	bankKeeper cmn.BankKeeper,
	addrCdc address.Codec,
) *Precompile {
	return &Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:           storetypes.KVGasConfig(),
			TransientKVGasConfig:  storetypes.TransientGasConfig(),
			ContractAddress:       common.HexToAddress(SupernodePrecompileAddress),
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		ABI:        ABI,
		snKeeper:   snKeeper,
		snMsgSvr:   snkeeper.NewMsgServerImpl(snKeeper),
		snQuerySvr: snkeeper.NewQueryServerImpl(snKeeper),
		addrCdc:    addrCdc,
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
	// Transactions
	case RegisterSupernodeMethod:
		return p.RegisterSupernode(ctx, contract, stateDB, method, args)
	case DeregisterSupernodeMethod:
		return p.DeregisterSupernode(ctx, contract, stateDB, method, args)
	case StartSupernodeMethod:
		return p.StartSupernode(ctx, contract, stateDB, method, args)
	case StopSupernodeMethod:
		return p.StopSupernode(ctx, contract, stateDB, method, args)
	case UpdateSupernodeMethod:
		return p.UpdateSupernode(ctx, contract, stateDB, method, args)
	case ReportMetricsMethod:
		return p.ReportMetrics(ctx, contract, stateDB, method, args)
	// Queries
	case GetSuperNodeMethod:
		return p.GetSuperNode(ctx, method, args)
	case GetSuperNodeByAccountMethod:
		return p.GetSuperNodeByAccount(ctx, method, args)
	case ListSuperNodesMethod:
		return p.ListSuperNodes(ctx, method, args)
	case GetTopSuperNodesForBlockMethod:
		return p.GetTopSuperNodesForBlock(ctx, method, args)
	case GetMetricsMethod:
		return p.GetMetrics(ctx, method, args)
	case GetParamsMethod:
		return p.GetParams(ctx, method, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}
}

// IsTransaction returns true for state-changing methods.
func (Precompile) IsTransaction(method *abi.Method) bool {
	switch method.Name {
	case RegisterSupernodeMethod, DeregisterSupernodeMethod,
		StartSupernodeMethod, StopSupernodeMethod,
		UpdateSupernodeMethod, ReportMetricsMethod:
		return true
	default:
		return false
	}
}

// Logger returns a precompile-specific logger.
func (p Precompile) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("evm extension", "supernode")
}
