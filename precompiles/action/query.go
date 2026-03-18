package action

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"

	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
)

const (
	// GetActionMethod is the ABI method name for querying a single action.
	GetActionMethod = "getAction"
	// GetActionFeeMethod is the ABI method name for querying action fees.
	GetActionFeeMethod = "getActionFee"
	// GetActionsByCreatorMethod is the ABI method name for querying actions by creator.
	GetActionsByCreatorMethod = "getActionsByCreator"
	// GetActionsByStateMethod is the ABI method name for querying actions by state.
	GetActionsByStateMethod = "getActionsByState"
	// GetActionsBySuperNodeMethod is the ABI method name for querying actions by supernode.
	GetActionsBySuperNodeMethod = "getActionsBySuperNode"
	// GetParamsMethod is the ABI method name for querying module parameters.
	GetParamsMethod = "getParams"

	// maxQueryLimit caps paginated results to prevent gas griefing.
	maxQueryLimit = 100
)

// GetAction returns details of a single action by ID.
func (p Precompile) GetAction(
	ctx sdk.Context,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("getAction: expected 1 arg, got %d", len(args))
	}

	actionId := args[0].(string)

	resp, err := p.actionQuerySvr.GetAction(ctx, &actiontypes.QueryGetActionRequest{
		ActionID: actionId,
	})
	if err != nil {
		return nil, err
	}

	keeper := p.actionKeeper
	info, err := actionToABIInfo(p.addrCdc, resp.Action, &keeper)
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(info)
}

// GetActionFee returns the fee breakdown for a given data size.
func (p Precompile) GetActionFee(
	ctx sdk.Context,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("getActionFee: expected 1 arg, got %d", len(args))
	}

	dataSizeKbs := args[0].(uint64)

	params := p.actionKeeper.GetParams(ctx)

	baseFee := params.BaseActionFee.Amount.BigInt()
	perKb := params.FeePerKbyte.Amount.BigInt()
	totalFee := new(big.Int).Add(
		baseFee,
		new(big.Int).Mul(perKb, new(big.Int).SetUint64(dataSizeKbs)),
	)

	return method.Outputs.Pack(baseFee, perKb, totalFee)
}

// GetActionsByCreator returns paginated actions filtered by creator address.
func (p Precompile) GetActionsByCreator(
	ctx sdk.Context,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("getActionsByCreator: expected 3 args, got %d", len(args))
	}

	creatorAddr := args[0].(common.Address)
	offset := args[1].(uint64)
	limit := args[2].(uint64)

	if limit > maxQueryLimit {
		limit = maxQueryLimit
	}

	creator, err := evmAddrToBech32(p.addrCdc, creatorAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid creator address: %w", err)
	}

	resp, err := p.actionQuerySvr.ListActionsByCreator(ctx, &actiontypes.QueryListActionsByCreatorRequest{
		Creator: creator,
		Pagination: &query.PageRequest{
			Offset:     offset,
			Limit:      limit,
			CountTotal: true,
		},
	})
	if err != nil {
		return nil, err
	}

	return p.packActionListResponse(method, resp.Actions, resp.GetPagination().GetTotal())
}

// GetActionsByState returns paginated actions filtered by state.
func (p Precompile) GetActionsByState(
	ctx sdk.Context,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("getActionsByState: expected 3 args, got %d", len(args))
	}

	state := args[0].(uint8)
	offset := args[1].(uint64)
	limit := args[2].(uint64)

	if limit > maxQueryLimit {
		limit = maxQueryLimit
	}

	resp, err := p.actionQuerySvr.ListActions(ctx, &actiontypes.QueryListActionsRequest{
		ActionState: actiontypes.ActionState(int32(state)),
		Pagination: &query.PageRequest{
			Offset:     offset,
			Limit:      limit,
			CountTotal: true,
		},
	})
	if err != nil {
		return nil, err
	}

	return p.packActionListResponse(method, resp.Actions, resp.GetPagination().GetTotal())
}

// GetActionsBySuperNode returns paginated actions assigned to a supernode.
func (p Precompile) GetActionsBySuperNode(
	ctx sdk.Context,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("getActionsBySuperNode: expected 3 args, got %d", len(args))
	}

	snAddr := args[0].(common.Address)
	offset := args[1].(uint64)
	limit := args[2].(uint64)

	if limit > maxQueryLimit {
		limit = maxQueryLimit
	}

	superNode, err := evmAddrToBech32(p.addrCdc, snAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid supernode address: %w", err)
	}

	resp, err := p.actionQuerySvr.ListActionsBySuperNode(ctx, &actiontypes.QueryListActionsBySuperNodeRequest{
		SuperNodeAddress: superNode,
		Pagination: &query.PageRequest{
			Offset:     offset,
			Limit:      limit,
			CountTotal: true,
		},
	})
	if err != nil {
		return nil, err
	}

	return p.packActionListResponse(method, resp.Actions, resp.GetPagination().GetTotal())
}

// GetParams returns the action module parameters.
func (p Precompile) GetParams(
	ctx sdk.Context,
	method *abi.Method,
	_ []interface{},
) ([]byte, error) {
	params := p.actionKeeper.GetParams(ctx)

	baseFee := params.BaseActionFee.Amount.BigInt()
	perKb := params.FeePerKbyte.Amount.BigInt()
	expDuration := int64(params.ExpirationDuration.Seconds())

	return method.Outputs.Pack(
		baseFee,
		perKb,
		params.MaxActionsPerBlock,
		params.MinSuperNodes,
		expDuration,
		params.SuperNodeFeeShare,
		params.FoundationFeeShare,
	)
}

// packActionListResponse converts a list of actions to ABI-packed output with total count.
func (p Precompile) packActionListResponse(
	method *abi.Method,
	actions []*actiontypes.Action,
	total uint64,
) ([]byte, error) {
	keeper := p.actionKeeper
	infos := make([]ActionInfo, 0, len(actions))
	for _, a := range actions {
		info, err := actionToABIInfo(p.addrCdc, a, &keeper)
		if err != nil {
			continue // skip actions that can't be converted
		}
		infos = append(infos, info)
	}

	return method.Outputs.Pack(infos, total)
}
