package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/pastelnetwork/pastel/x/supernode/types"
)

// DeregisterSupernode defines a method to deregister a supernode
func (k msgServer) DeregisterSupernode(goCtx context.Context, msg *types.MsgDeregisterSupernode) (*types.MsgDeregisterSupernodeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Convert validator address string to ValAddress
	valOperAddr, err := sdk.ValAddressFromBech32(msg.ValidatorAddress)
	if err != nil {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid validator address: %s", err)
	}

	// Get existing supernode
	supernode, found := k.QuerySuperNode(ctx, valOperAddr)
	if !found {
		return nil, errorsmod.Wrapf(sdkerrors.ErrNotFound, "no supernode found for validator %s", msg.ValidatorAddress)
	}

	// Verify the signer is authorized
	if err := k.verifyValidatorOperator(ctx, valOperAddr, msg.Creator); err != nil {
		return nil, err
	}

	// Update supernode state
	supernode.State = types.SuperNodeStateDisabled

	// Re-save the supernode using SetSuperNode
	if err := k.SetSuperNode(ctx, supernode); err != nil {
		return nil, err
	}

	// Emit event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeSupernodeDeRegistered,
			sdk.NewAttribute(types.AttributeKeyValidatorAddress, msg.ValidatorAddress),
		),
	)

	return &types.MsgDeregisterSupernodeResponse{}, nil
}
