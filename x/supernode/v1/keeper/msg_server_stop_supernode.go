package keeper

import (
    "context"
    "strconv"

    types2 "github.com/LumeraProtocol/lumera/x/supernode/v1/types"

    errorsmod "cosmossdk.io/errors"
    sdk "github.com/cosmos/cosmos-sdk/types"
    sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// StopSupernode stops an active supernode (transitions from Active to Stopped state)
func (k msgServer) StopSupernode(goCtx context.Context, msg *types2.MsgStopSupernode) (*types2.MsgStopSupernodeResponse, error) {
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
	case types2.SuperNodeStateStopped:
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "supernode is already stopped")
	case types2.SuperNodeStateDisabled:
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "supernode is disabled")
	}

    prevState := supernode.States[len(supernode.States)-1].State
    supernode.States = append(supernode.States, &types2.SuperNodeStateRecord{
        State:  types2.SuperNodeStateStopped,
        Height: ctx.BlockHeight(),
    })

	if err := k.SetSuperNode(ctx, supernode); err != nil {
		return nil, err
	}

    ctx.EventManager().EmitEvent(
        sdk.NewEvent(
            types2.EventTypeSupernodeStopped,
            sdk.NewAttribute(types2.AttributeKeyValidatorAddress, msg.ValidatorAddress),
            sdk.NewAttribute(types2.AttributeKeyReason, msg.Reason),
            sdk.NewAttribute(types2.AttributeKeyOldState, prevState.String()),
            sdk.NewAttribute(types2.AttributeKeyHeight, strconv.FormatInt(ctx.BlockHeight(), 10)),
        ),
    )

	return &types2.MsgStopSupernodeResponse{}, nil
}
