package keeper

import (
	"context"
	"strconv"
	"strings"

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

	// Track changes for event emission
	var fieldsUpdated []string
	changedIP := false
	changedAccount := false
	changedNote := false
	changedP2P := false
	oldIP := ""
	newIP := ""
	oldAccount := ""
	newAccount := ""
	oldP2P := supernode.P2PPort

	// Update fields
	if msg.IpAddress != "" {
		currentIP := ""
		if len(supernode.PrevIpAddresses) > 0 {
			currentIP = supernode.PrevIpAddresses[len(supernode.PrevIpAddresses)-1].Address
		}
		if currentIP != msg.IpAddress {
			changedIP = true
			oldIP = currentIP
			newIP = msg.IpAddress
			supernode.PrevIpAddresses = append(supernode.PrevIpAddresses, &types2.IPAddressHistory{
				Address: msg.IpAddress,
				Height:  ctx.BlockHeight(),
			})
		}
	}

	if msg.SupernodeAccount != "" {
		// Validate the new supernode account address
		if _, err := sdk.AccAddressFromBech32(msg.SupernodeAccount); err != nil {
			return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid supernode account address: %s", err)
		}

		// Track supernode account history if changed
		if supernode.SupernodeAccount != msg.SupernodeAccount {
			changedAccount = true
			oldAccount = supernode.SupernodeAccount
			newAccount = msg.SupernodeAccount

			// Store the new account in history with recorded block height
			historyLen := len(supernode.PrevSupernodeAccounts)
			if historyLen == 0 || supernode.PrevSupernodeAccounts[historyLen-1].Account != msg.SupernodeAccount {
				supernode.PrevSupernodeAccounts = append(supernode.PrevSupernodeAccounts, &types2.SupernodeAccountHistory{
					Account: msg.SupernodeAccount,
					Height:  ctx.BlockHeight(),
				})
			}

			// Update the account
			supernode.SupernodeAccount = msg.SupernodeAccount
		}
	}

	if msg.Note != "" {
		if supernode.Note != msg.Note {
			changedNote = true
			supernode.Note = msg.Note
		}
	}

	// Update P2P port if provided
	if msg.P2PPort != "" {
		if supernode.P2PPort != msg.P2PPort {
			changedP2P = true
			supernode.P2PPort = msg.P2PPort
		}
	}

	// Re-save
	if err := k.SetSuperNode(ctx, supernode); err != nil {
		return nil, err
	}

	// Build fields_updated and emit consolidated event with contextual attributes
	if changedIP {
		fieldsUpdated = append(fieldsUpdated, types2.AttributeKeyIPAddress)
	}
	if changedAccount {
		fieldsUpdated = append(fieldsUpdated, types2.AttributeKeySupernodeAccount)
	}
	if changedNote {
		fieldsUpdated = append(fieldsUpdated, "note")
	}
	if changedP2P {
		fieldsUpdated = append(fieldsUpdated, types2.AttributeKeyP2PPort)
	}

	// Always emit an update event, even if no fields changed, for observability
	attrs := []sdk.Attribute{
		sdk.NewAttribute(types2.AttributeKeyValidatorAddress, msg.ValidatorAddress),
		sdk.NewAttribute(types2.AttributeKeyFieldsUpdated, strings.Join(fieldsUpdated, ",")),
		sdk.NewAttribute(types2.AttributeKeyHeight, strconv.FormatInt(ctx.BlockHeight(), 10)),
	}
	if changedAccount {
		attrs = append(attrs,
			sdk.NewAttribute(types2.AttributeKeyOldAccount, oldAccount),
			sdk.NewAttribute(types2.AttributeKeyNewAccount, newAccount),
		)
	}
	if changedP2P {
		attrs = append(attrs,
			sdk.NewAttribute(types2.AttributeKeyOldP2PPort, oldP2P),
			sdk.NewAttribute(types2.AttributeKeyP2PPort, supernode.P2PPort),
		)
	}
	if changedIP {
		attrs = append(attrs,
			sdk.NewAttribute(types2.AttributeKeyOldIPAddress, oldIP),
			sdk.NewAttribute(types2.AttributeKeyIPAddress, newIP),
		)
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types2.EventTypeSupernodeUpdated,
			attrs...,
		),
	)

	return &types2.MsgUpdateSupernodeResponse{}, nil
}
