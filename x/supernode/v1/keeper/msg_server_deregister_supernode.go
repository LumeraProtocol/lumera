package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// DeregisterSupernode defines a method to deregister a supernode
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

	if supernode.States[len(supernode.States)-1].State != types.SuperNodeStateDisabled {
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
		),
	)

	return &types.MsgDeregisterSupernodeResponse{}, nil
}
