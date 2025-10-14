package keeper

import (
	"context"
	"strconv"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// DeregisterSupernode permanently disables a supernode (sets state to Disabled - terminal state)
// A disabled supernode cannot be reactivated and must be re-registered to participate again
func (k msgServer) DeregisterSupernode(goCtx context.Context, msg *types.MsgDeregisterSupernode) (*types.MsgDeregisterSupernodeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Convert validator address string to ValAddress
	valOperAddr, err := sdk.ValAddressFromBech32(msg.ValidatorAddress)
	if err != nil {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid validator address: %s", err)
	}

	// Verify the signer is authorized
	if err := VerifyValidatorOperator(valOperAddr, msg.Creator); err != nil {
		return nil, err
	}

	// Get existing supernode
	supernode, found := k.QuerySuperNode(ctx, valOperAddr)
	if !found {
		return nil, errorsmod.Wrapf(sdkerrors.ErrNotFound, "no supernode found for validator %s", msg.ValidatorAddress)
	}

	if len(supernode.States) == 0 {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "supernode is in an invalid state")
	}

	// Capture the previous state to expose as an attribute
	prevState := supernode.States[len(supernode.States)-1].State
	if prevState != types.SuperNodeStateDisabled {
		// Update supernode state
		supernode.States = append(supernode.States, &types.SuperNodeStateRecord{
			State:  types.SuperNodeStateDisabled,
			Height: ctx.BlockHeight(),
		})
	}

	// Re-save the supernode using SetSuperNode
	if err := k.SetSuperNode(ctx, supernode); err != nil {
		return nil, err
	}

	// Emit event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeSupernodeDeRegistered,
			sdk.NewAttribute(types.AttributeKeyValidatorAddress, msg.ValidatorAddress),
			sdk.NewAttribute(types.AttributeKeyOldState, prevState.String()),
			sdk.NewAttribute(types.AttributeKeyHeight, strconv.FormatInt(ctx.BlockHeight(), 10)),
		),
	)

	return &types.MsgDeregisterSupernodeResponse{}, nil
}
