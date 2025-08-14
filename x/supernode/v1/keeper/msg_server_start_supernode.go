package keeper

import (
	"context"

	types2 "github.com/LumeraProtocol/lumera/x/supernode/v1/types"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// StartSupernode activates a stopped supernode (transitions from Stopped to Active state)
func (k msgServer) StartSupernode(goCtx context.Context, msg *types2.MsgStartSupernode) (*types2.MsgStartSupernodeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	valOperAddr, err := sdk.ValAddressFromBech32(msg.ValidatorAddress)
	if err != nil {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid validator address: %s", err)
	}

	supernode, found := k.QuerySuperNode(ctx, valOperAddr)
	if !found {
		return nil, errorsmod.Wrapf(sdkerrors.ErrNotFound, "no supernode found for validator %s", msg.ValidatorAddress)
	}

	if err := VerifyValidatorOperator(valOperAddr, msg.Creator); err != nil {
		return nil, err
	}

	if len(supernode.States) == 0 {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "supernode is in an invalid state")
	}
	
	currentState := supernode.States[len(supernode.States)-1].State
	
	// Check if supernode is disabled (terminal state)
	if currentState == types2.SuperNodeStateDisabled {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "supernode is disabled and must be re-registered")
	}
	
	// Check if already active
	if currentState == types2.SuperNodeStateActive {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "supernode is already started")
	}
	
	// Can only start from stopped state
	if currentState != types2.SuperNodeStateStopped {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "supernode must be in stopped state to start")
	}
	
	supernode.States = append(supernode.States, &types2.SuperNodeStateRecord{
		State:  types2.SuperNodeStateActive,
		Height: ctx.BlockHeight(),
	})

	if err := k.SetSuperNode(ctx, supernode); err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types2.EventTypeSupernodeStarted,
			sdk.NewAttribute(types2.AttributeKeyValidatorAddress, msg.ValidatorAddress),
		),
	)

	return &types2.MsgStartSupernodeResponse{}, nil
}
