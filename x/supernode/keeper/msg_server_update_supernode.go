package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	"github.com/LumeraProtocol/lumera/x/supernode/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
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

	if err := VerifyValidatorOperator(valOperAddr, msg.Creator); err != nil {
		return nil, err
	}

	// Update fields
	if msg.IpAddress != "" {
		if len(supernode.PrevIpAddresses) == 0 || supernode.PrevIpAddresses[len(supernode.PrevIpAddresses)-1].Address != msg.IpAddress {
			supernode.PrevIpAddresses = append(supernode.PrevIpAddresses, &types.IPAddressHistory{
				Address: msg.IpAddress,
				Height:  ctx.BlockHeight(),
			})
		}
	}

	if msg.SupernodeAccount != "" {
		supernode.SupernodeAccount = msg.SupernodeAccount
	}

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
