package supernode

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"

	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

const (
	// GetSuperNodeMethod is the ABI method name for querying a single supernode.
	GetSuperNodeMethod = "getSuperNode"
	// GetSuperNodeByAccountMethod is the ABI method name for querying a supernode by account.
	GetSuperNodeByAccountMethod = "getSuperNodeByAccount"
	// ListSuperNodesMethod is the ABI method name for listing supernodes.
	ListSuperNodesMethod = "listSuperNodes"
	// GetTopSuperNodesForBlockMethod is the ABI method name for querying top supernodes.
	GetTopSuperNodesForBlockMethod = "getTopSuperNodesForBlock"
	// GetMetricsMethod is the ABI method name for querying supernode metrics.
	GetMetricsMethod = "getMetrics"
	// GetParamsMethod is the ABI method name for querying module parameters.
	GetParamsMethod = "getParams"

	// maxQueryLimit caps paginated results to prevent gas griefing.
	maxQueryLimit = 100
)

// GetSuperNode returns details of a single supernode by validator address.
func (p Precompile) GetSuperNode(
	ctx sdk.Context,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("getSuperNode: expected 1 arg, got %d", len(args))
	}

	validatorAddress := args[0].(string)

	resp, err := p.snQuerySvr.GetSuperNode(ctx, &sntypes.QueryGetSuperNodeRequest{
		ValidatorAddress: validatorAddress,
	})
	if err != nil {
		return nil, err
	}

	info := supernodeToABIInfo(resp.Supernode)
	return method.Outputs.Pack(info)
}

// GetSuperNodeByAccount returns details of a supernode by its account address.
func (p Precompile) GetSuperNodeByAccount(
	ctx sdk.Context,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("getSuperNodeByAccount: expected 1 arg, got %d", len(args))
	}

	supernodeAddress := args[0].(string)

	resp, err := p.snQuerySvr.GetSuperNodeBySuperNodeAddress(ctx, &sntypes.QueryGetSuperNodeBySuperNodeAddressRequest{
		SupernodeAddress: supernodeAddress,
	})
	if err != nil {
		return nil, err
	}

	info := supernodeToABIInfo(resp.Supernode)
	return method.Outputs.Pack(info)
}

// ListSuperNodes returns a paginated list of all supernodes.
func (p Precompile) ListSuperNodes(
	ctx sdk.Context,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("listSuperNodes: expected 2 args, got %d", len(args))
	}

	offset := args[0].(uint64)
	limit := args[1].(uint64)

	if limit > maxQueryLimit {
		limit = maxQueryLimit
	}

	resp, err := p.snQuerySvr.ListSuperNodes(ctx, &sntypes.QueryListSuperNodesRequest{
		Pagination: &query.PageRequest{
			Offset:     offset,
			Limit:      limit,
			CountTotal: true,
		},
	})
	if err != nil {
		return nil, err
	}

	infos := make([]SuperNodeInfo, 0, len(resp.Supernodes))
	for _, sn := range resp.Supernodes {
		infos = append(infos, supernodeToABIInfo(sn))
	}

	var total uint64
	if resp.Pagination != nil {
		total = resp.Pagination.Total
	}

	return method.Outputs.Pack(infos, total)
}

// GetTopSuperNodesForBlock returns supernodes ranked by XOR distance from block hash.
func (p Precompile) GetTopSuperNodesForBlock(
	ctx sdk.Context,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("getTopSuperNodesForBlock: expected 3 args, got %d", len(args))
	}

	blockHeight := args[0].(int32)
	limit := args[1].(int32)
	state := args[2].(uint8)

	// Convert state uint8 to the string the query server expects
	stateStr := ""
	if state > 0 {
		stateVal := sntypes.SuperNodeState(int32(state))
		stateStr = stateVal.String()
	}

	resp, err := p.snQuerySvr.GetTopSuperNodesForBlock(ctx, &sntypes.QueryGetTopSuperNodesForBlockRequest{
		BlockHeight: blockHeight,
		Limit:       limit,
		State:       stateStr,
	})
	if err != nil {
		return nil, err
	}

	infos := make([]SuperNodeInfo, 0, len(resp.Supernodes))
	for _, sn := range resp.Supernodes {
		infos = append(infos, supernodeToABIInfo(sn))
	}

	return method.Outputs.Pack(infos)
}

// GetMetrics returns the latest metrics report for a supernode.
func (p Precompile) GetMetrics(
	ctx sdk.Context,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("getMetrics: expected 1 arg, got %d", len(args))
	}

	validatorAddress := args[0].(string)

	resp, err := p.snQuerySvr.GetMetrics(ctx, &sntypes.QueryGetMetricsRequest{
		ValidatorAddress: validatorAddress,
	})
	if err != nil {
		return nil, err
	}

	var metrics MetricsReport
	var reportCount uint64
	var lastReportHeight int64

	if resp.MetricsState != nil {
		metrics = metricsToABI(resp.MetricsState.Metrics)
		reportCount = resp.MetricsState.ReportCount
		lastReportHeight = resp.MetricsState.Height
	}

	return method.Outputs.Pack(metrics, reportCount, lastReportHeight)
}

// GetParams returns the supernode module parameters.
func (p Precompile) GetParams(
	ctx sdk.Context,
	method *abi.Method,
	_ []interface{},
) ([]byte, error) {
	params := p.snKeeper.GetParams(ctx)

	minimumStake := params.MinimumStakeForSn.Amount.BigInt()

	return method.Outputs.Pack(
		minimumStake,
		params.ReportingThreshold,
		params.SlashingThreshold,
		params.MinSupernodeVersion,
		params.MinCpuCores,
		params.MinMemGb,
		params.MinStorageGb,
	)
}
