package keeper

import (
	"context"

	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

func (k msgServer) UpdateSupernode(goCtx context.Context, msg *sntypes.MsgUpdateSupernode) (*sntypes.MsgUpdateSupernodeResponse, error) {
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
		currentIP := ""
		if len(supernode.PrevIpAddresses) > 0 {
			currentIP = supernode.PrevIpAddresses[len(supernode.PrevIpAddresses)-1].Address
		}

		if currentIP != msg.IpAddress {
			supernode.PrevIpAddresses = append(supernode.PrevIpAddresses, &sntypes.IPAddressHistory{
				Address: msg.IpAddress,
				Height:  ctx.BlockHeight(),
			})

			// Emit event for IP address change
			ctx.EventManager().EmitEvent(
				sdk.NewEvent(
					sntypes.EventTypeSupernodeUpdated,
					sdk.NewAttribute(sntypes.AttributeKeyValidatorAddress, msg.ValidatorAddress),
					sdk.NewAttribute("old_ip_address", currentIP),
					sdk.NewAttribute(sntypes.AttributeKeyIPAddress, msg.IpAddress),
				),
			)
		}
	}

	if msg.SupernodeAccount != "" {
		// Validate the new supernode account address
		if _, err := sdk.AccAddressFromBech32(msg.SupernodeAccount); err != nil {
			return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid supernode account address: %s", err)
		}

		// Track supernode account history
		if supernode.SupernodeAccount != msg.SupernodeAccount {
			oldAccount := supernode.SupernodeAccount

			// Store the previous account in history
			supernode.PrevSupernodeAccounts = append(supernode.PrevSupernodeAccounts, &sntypes.SupernodeAccountHistory{
				Account: oldAccount,
				Height:  ctx.BlockHeight(),
			})

			// Update the account
			supernode.SupernodeAccount = msg.SupernodeAccount

			// Emit event for account change
			ctx.EventManager().EmitEvent(
				sdk.NewEvent(
					sntypes.EventTypeSupernodeUpdated,
					sdk.NewAttribute(sntypes.AttributeKeyValidatorAddress, msg.ValidatorAddress),
					sdk.NewAttribute(sntypes.AttributeKeyOldAccount, oldAccount),
					sdk.NewAttribute(sntypes.AttributeKeyNewAccount, msg.SupernodeAccount),
				),
			)
		} else {
			supernode.SupernodeAccount = msg.SupernodeAccount
		}
	}

	if msg.Version != "" && supernode.Version != msg.Version {
		oldVersion := supernode.Version
		supernode.Version = msg.Version

		// Emit event for version change
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				sntypes.EventTypeSupernodeUpdated,
				sdk.NewAttribute(sntypes.AttributeKeyValidatorAddress, msg.ValidatorAddress),
				sdk.NewAttribute("old_version", oldVersion),
				sdk.NewAttribute(sntypes.AttributeKeyVersion, msg.Version),
			),
		)
	}

	// Re-save
	if err := k.SetSuperNode(ctx, supernode); err != nil {
		return nil, err
	}

	return &sntypes.MsgUpdateSupernodeResponse{}, nil
}
