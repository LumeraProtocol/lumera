package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/pastelnetwork/pastel/x/supernode/types"
)

func (k msgServer) StopSupernode(goCtx context.Context, msg *types.MsgStopSupernode) (*types.MsgStopSupernodeResponse, error) {
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

	switch supernode.States[len(supernode.States)-1].State {
	case types.SuperNodeStateStopped:
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "supernode is already stopped")
	case types.SuperNodeStateDisabled:
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "supernode is disabled")
	}

	supernode.States = append(supernode.States, &types.SuperNodeStateRecord{
		State:  types.SuperNodeStateStopped,
		Height: ctx.BlockHeight(),
	})

	if err := k.SetSuperNode(ctx, supernode); err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeSupernodeStopped,
			sdk.NewAttribute(types.AttributeKeyValidatorAddress, msg.ValidatorAddress),
			sdk.NewAttribute(types.AttributeKeyReason, msg.Reason),
		),
	)

	return &types.MsgStopSupernodeResponse{}, nil
}
