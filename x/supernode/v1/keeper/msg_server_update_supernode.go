package keeper

import (
	"context"

	types2 "github.com/LumeraProtocol/lumera/x/supernode/v1/types"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

func (k msgServer) UpdateSupernode(goCtx context.Context, msg *types2.MsgUpdateSupernode) (*types2.MsgUpdateSupernodeResponse, error) {
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

	// Update fields
	if msg.IpAddress != "" {
		if len(supernode.PrevIpAddresses) == 0 || supernode.PrevIpAddresses[len(supernode.PrevIpAddresses)-1].Address != msg.IpAddress {
			supernode.PrevIpAddresses = append(supernode.PrevIpAddresses, &types2.IPAddressHistory{
				Address: msg.IpAddress,
				Height:  ctx.BlockHeight(),
			})
		}
	}

	if msg.SupernodeAccount != "" {
		supernode.SupernodeAccount = msg.SupernodeAccount
	}

	if msg.Note != "" {
		supernode.Note = msg.Note
	}

	// Re-save
	if err := k.SetSuperNode(ctx, supernode); err != nil {
		return nil, err
	}

	// Emit event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types2.EventTypeSupernodeUpdated,
			sdk.NewAttribute(types2.AttributeKeyValidatorAddress, msg.ValidatorAddress),
			sdk.NewAttribute(types2.AttributeKeyVersion, supernode.Note),
		),
	)

	return &types2.MsgUpdateSupernodeResponse{}, nil
}
