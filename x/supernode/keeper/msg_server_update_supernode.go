package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/pastelnetwork/pastel/x/supernode/types"
)

func (k msgServer) UpdateSupernode(goCtx context.Context, msg *types.MsgUpdateSupernode) (*types.MsgUpdateSupernodeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	valOperAddr, err := sdk.ValAddressFromBech32(msg.ValidatorAddress)
	if err != nil {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid validator address: %s", err)
	}

	supernode, found := k.QuerySuperNode(ctx, valOperAddr)
	if !found {
		return nil, errorsmod.Wrapf(sdkerrors.ErrNotFound, "no supernode found for validator %s", msg.ValidatorAddress)
	}

	if err := k.verifyValidatorOperator(ctx, valOperAddr, msg.Creator); err != nil {
		return nil, err
	}

	// Update fields
	/*if msg.IpAddress != "" && supernode.IpAddress != msg.IpAddress {
		supernode.PrevIpAddresses = append(supernode.PrevIpAddresses, &types.IPAddressHistory{
			Address: supernode.IpAddress,
		})
		supernode.IpAddress = msg.IpAddress
	}
	*/
	if msg.Version != "" {
		supernode.Version = msg.Version
	}

	// Re-save
	if err := k.SetSuperNode(ctx, supernode); err != nil {
		return nil, err
	}

	// Emit event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeSupernodeUpdated,
			sdk.NewAttribute(types.AttributeKeyValidatorAddress, msg.ValidatorAddress),
			sdk.NewAttribute(types.AttributeKeyVersion, supernode.Version),
		),
	)

	return &types.MsgUpdateSupernodeResponse{}, nil
}
