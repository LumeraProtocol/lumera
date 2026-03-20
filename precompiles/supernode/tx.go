package supernode

import (
	"fmt"

	"cosmossdk.io/core/address"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	sdk "github.com/cosmos/cosmos-sdk/types"

	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

const (
	// RegisterSupernodeMethod is the ABI method name for registering a supernode.
	RegisterSupernodeMethod = "registerSupernode"
	// DeregisterSupernodeMethod is the ABI method name for deregistering a supernode.
	DeregisterSupernodeMethod = "deregisterSupernode"
	// StartSupernodeMethod is the ABI method name for starting a supernode.
	StartSupernodeMethod = "startSupernode"
	// StopSupernodeMethod is the ABI method name for stopping a supernode.
	StopSupernodeMethod = "stopSupernode"
	// UpdateSupernodeMethod is the ABI method name for updating a supernode.
	UpdateSupernodeMethod = "updateSupernode"
	// ReportMetricsMethod is the ABI method name for reporting supernode metrics.
	ReportMetricsMethod = "reportMetrics"
)

// evmAddrToBech32 converts an EVM hex address to a Bech32 address string.
func evmAddrToBech32(addrCdc address.Codec, addr common.Address) (string, error) {
	return addrCdc.BytesToString(addr.Bytes())
}

// RegisterSupernode registers a new supernode or re-registers from Disabled state.
func (p Precompile) RegisterSupernode(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 4 {
		return nil, fmt.Errorf("registerSupernode: expected 4 args, got %d", len(args))
	}

	validatorAddress := args[0].(string)
	ipAddress := args[1].(string)
	supernodeAccount := args[2].(string)
	p2pPort := args[3].(string)

	creator, err := evmAddrToBech32(p.addrCdc, contract.Caller())
	if err != nil {
		return nil, fmt.Errorf("invalid caller address: %w", err)
	}

	msg := &sntypes.MsgRegisterSupernode{
		Creator:          creator,
		ValidatorAddress: validatorAddress,
		IpAddress:        ipAddress,
		SupernodeAccount: supernodeAccount,
		P2PPort:          p2pPort,
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"creator", creator,
		"validator", validatorAddress,
	)

	if _, err := p.snMsgSvr.RegisterSupernode(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitSupernodeRegistered(ctx, stateDB, validatorAddress, contract.Caller(), uint8(sntypes.SuperNodeStateActive)); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// DeregisterSupernode deregisters an existing supernode.
func (p Precompile) DeregisterSupernode(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("deregisterSupernode: expected 1 arg, got %d", len(args))
	}

	validatorAddress := args[0].(string)

	creator, err := evmAddrToBech32(p.addrCdc, contract.Caller())
	if err != nil {
		return nil, fmt.Errorf("invalid caller address: %w", err)
	}

	// Get the current state before deregistering for the event
	valAddr, err := sdk.ValAddressFromBech32(validatorAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid validator address: %w", err)
	}
	var oldState uint8
	if sn, found := p.snKeeper.QuerySuperNode(ctx, valAddr); found && len(sn.States) > 0 {
		oldState = uint8(sn.States[len(sn.States)-1].State)
	}

	msg := &sntypes.MsgDeregisterSupernode{
		Creator:          creator,
		ValidatorAddress: validatorAddress,
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"creator", creator,
		"validator", validatorAddress,
	)

	if _, err := p.snMsgSvr.DeregisterSupernode(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitSupernodeDeregistered(ctx, stateDB, validatorAddress, contract.Caller(), oldState); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// StartSupernode activates a stopped supernode.
func (p Precompile) StartSupernode(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("startSupernode: expected 1 arg, got %d", len(args))
	}

	validatorAddress := args[0].(string)

	creator, err := evmAddrToBech32(p.addrCdc, contract.Caller())
	if err != nil {
		return nil, fmt.Errorf("invalid caller address: %w", err)
	}

	msg := &sntypes.MsgStartSupernode{
		Creator:          creator,
		ValidatorAddress: validatorAddress,
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"creator", creator,
		"validator", validatorAddress,
	)

	if _, err := p.snMsgSvr.StartSupernode(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitSupernodeStateChanged(ctx, stateDB, validatorAddress, contract.Caller(), uint8(sntypes.SuperNodeStateActive)); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// StopSupernode stops an active supernode.
func (p Precompile) StopSupernode(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("stopSupernode: expected 2 args, got %d", len(args))
	}

	validatorAddress := args[0].(string)
	reason := args[1].(string)

	creator, err := evmAddrToBech32(p.addrCdc, contract.Caller())
	if err != nil {
		return nil, fmt.Errorf("invalid caller address: %w", err)
	}

	msg := &sntypes.MsgStopSupernode{
		Creator:          creator,
		ValidatorAddress: validatorAddress,
		Reason:           reason,
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"creator", creator,
		"validator", validatorAddress,
		"reason", reason,
	)

	if _, err := p.snMsgSvr.StopSupernode(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitSupernodeStateChanged(ctx, stateDB, validatorAddress, contract.Caller(), uint8(sntypes.SuperNodeStateStopped)); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// UpdateSupernode updates configuration fields of a supernode.
func (p Precompile) UpdateSupernode(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 5 {
		return nil, fmt.Errorf("updateSupernode: expected 5 args, got %d", len(args))
	}

	validatorAddress := args[0].(string)
	ipAddress := args[1].(string)
	note := args[2].(string)
	supernodeAccount := args[3].(string)
	p2pPort := args[4].(string)

	creator, err := evmAddrToBech32(p.addrCdc, contract.Caller())
	if err != nil {
		return nil, fmt.Errorf("invalid caller address: %w", err)
	}

	msg := &sntypes.MsgUpdateSupernode{
		Creator:          creator,
		ValidatorAddress: validatorAddress,
		IpAddress:        ipAddress,
		Note:             note,
		SupernodeAccount: supernodeAccount,
		P2PPort:          p2pPort,
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"creator", creator,
		"validator", validatorAddress,
	)

	if _, err := p.snMsgSvr.UpdateSupernode(ctx, msg); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// ReportMetrics reports supernode metrics and returns compliance result.
func (p Precompile) ReportMetrics(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("reportMetrics: expected 3 args, got %d", len(args))
	}

	validatorAddress := args[0].(string)
	// args[1] (supernodeAccount) from calldata is intentionally ignored.
	// We derive the authoritative supernode account from the EVM caller to
	// prevent any account from reporting metrics on behalf of another.
	supernodeAccount, err := evmAddrToBech32(p.addrCdc, contract.Caller())
	if err != nil {
		return nil, fmt.Errorf("invalid caller address: %w", err)
	}
	metricsArg := args[2].(struct {
		VersionMajor     uint32 `abi:"versionMajor"`
		VersionMinor     uint32 `abi:"versionMinor"`
		VersionPatch     uint32 `abi:"versionPatch"`
		CpuCoresTotal    uint32 `abi:"cpuCoresTotal"`
		CpuUsagePercent  uint64 `abi:"cpuUsagePercent"`
		MemTotalGb       uint64 `abi:"memTotalGb"`
		MemUsagePercent  uint64 `abi:"memUsagePercent"`
		MemFreeGb        uint64 `abi:"memFreeGb"`
		DiskTotalGb      uint64 `abi:"diskTotalGb"`
		DiskUsagePercent uint64 `abi:"diskUsagePercent"`
		DiskFreeGb       uint64 `abi:"diskFreeGb"`
		UptimeSeconds    uint64 `abi:"uptimeSeconds"`
		PeersCount       uint32 `abi:"peersCount"`
	})

	metrics := sntypes.SupernodeMetrics{
		VersionMajor:     metricsArg.VersionMajor,
		VersionMinor:     metricsArg.VersionMinor,
		VersionPatch:     metricsArg.VersionPatch,
		CpuCoresTotal:    float64(metricsArg.CpuCoresTotal),
		CpuUsagePercent:  float64(metricsArg.CpuUsagePercent),
		MemTotalGb:       float64(metricsArg.MemTotalGb),
		MemUsagePercent:  float64(metricsArg.MemUsagePercent),
		MemFreeGb:        float64(metricsArg.MemFreeGb),
		DiskTotalGb:      float64(metricsArg.DiskTotalGb),
		DiskUsagePercent: float64(metricsArg.DiskUsagePercent),
		DiskFreeGb:       float64(metricsArg.DiskFreeGb),
		UptimeSeconds:    float64(metricsArg.UptimeSeconds),
		PeersCount:       metricsArg.PeersCount,
	}

	msg := &sntypes.MsgReportSupernodeMetrics{
		ValidatorAddress: validatorAddress,
		SupernodeAccount: supernodeAccount,
		Metrics:          metrics,
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"validator", validatorAddress,
		"supernode_account", supernodeAccount,
	)

	resp, err := p.snMsgSvr.ReportSupernodeMetrics(ctx, msg)
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(resp.Compliant, resp.Issues)
}
