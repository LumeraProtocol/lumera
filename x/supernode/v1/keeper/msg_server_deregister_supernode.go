package keeper

import (
	"context"
	types2 "github.com/LumeraProtocol/lumera/x/supernode/v1/types"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// DeregisterSupernode defines a method to deregister a supernode
func (k msgServer) DeregisterSupernode(goCtx context.Context, msg *types2.MsgDeregisterSupernode) (*types2.MsgDeregisterSupernodeResponse, error) {
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

	if supernode.States[len(supernode.States)-1].State != types2.SuperNodeStateDisabled {
		// Update supernode state
		supernode.States = append(supernode.States, &types2.SuperNodeStateRecord{
			State:  types2.SuperNodeStateDisabled,
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
			types2.EventTypeSupernodeDeRegistered,
			sdk.NewAttribute(types2.AttributeKeyValidatorAddress, msg.ValidatorAddress),
		),
	)

	return &types2.MsgDeregisterSupernodeResponse{}, nil
}
