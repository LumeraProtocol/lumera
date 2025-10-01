package keeper

import (
	"context"
	"strconv"

	errorsmod "cosmossdk.io/errors"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// StartSupernode activates a stopped supernode (transitions from Stopped to Active state)
func (k msgServer) StartSupernode(goCtx context.Context, msg *types.MsgStartSupernode) (*types.MsgStartSupernodeResponse, error) {
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

	// State-specific checks for better UX
	switch currentState {
	case types.SuperNodeStateDisabled:
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "supernode is disabled and must be re-registered")
	case types.SuperNodeStateActive:
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "supernode is already started")
	case types.SuperNodeStateStopped:
		// OK to proceed
	case types.SuperNodeStatePenalized:
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "supernode is penalized and cannot be started")
	default:
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "cannot start supernode from state=%s", currentState.String())
	}

	supernode.States = append(supernode.States, &types.SuperNodeStateRecord{
		State:  types.SuperNodeStateActive,
		Height: ctx.BlockHeight(),
	})

	if err := k.SetSuperNode(ctx, supernode); err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeSupernodeStarted,
			sdk.NewAttribute(types.AttributeKeyValidatorAddress, msg.ValidatorAddress),
			sdk.NewAttribute(types.AttributeKeyOldState, currentState.String()),
			sdk.NewAttribute(types.AttributeKeyHeight, strconv.FormatInt(ctx.BlockHeight(), 10)),
			sdk.NewAttribute(types.AttributeKeyReason, "tx_start"),
		),
	)

	return &types.MsgStartSupernodeResponse{}, nil
}
