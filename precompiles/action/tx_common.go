package action

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"

	sdk "github.com/cosmos/cosmos-sdk/types"

	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
)

const (
	// ApproveActionMethod is the ABI method name for approving any action type.
	ApproveActionMethod = "approveAction"
)

// ApproveAction approves a completed action (type-agnostic).
func (p Precompile) ApproveAction(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("approveAction: expected 1 arg, got %d", len(args))
	}

	actionId := args[0].(string)

	creator, err := evmAddrToBech32(p.addrCdc, contract.Caller())
	if err != nil {
		return nil, fmt.Errorf("invalid caller address: %w", err)
	}

	msg := &actiontypes.MsgApproveAction{
		Creator:  creator,
		ActionId: actionId,
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"creator", creator,
		"action_id", actionId,
	)

	if _, err := p.actionMsgSvr.ApproveAction(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitActionApproved(ctx, stateDB, actionId, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}
