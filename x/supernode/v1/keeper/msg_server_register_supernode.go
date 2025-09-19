package keeper

import (
	"context"

	types2 "github.com/LumeraProtocol/lumera/x/supernode/v1/types"

	errorsmod "cosmossdk.io/errors"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k msgServer) RegisterSupernode(goCtx context.Context, msg *types2.MsgRegisterSupernode) (*types2.MsgRegisterSupernodeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Convert validator address string to ValAddress
	valOperAddr, err := sdk.ValAddressFromBech32(msg.ValidatorAddress)
	if err != nil {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid validator address: %s", err)
	}

	// Authorization check
	if err := VerifyValidatorOperator(valOperAddr, msg.Creator); err != nil {
		return nil, err
	}

	//  Check if supernode exists
	existingSupernode, found := k.QuerySuperNode(ctx, valOperAddr)
	if found {
		// Check if it's disabled (deregistered) - allow re-registration
		if len(existingSupernode.States) > 0 && existingSupernode.States[len(existingSupernode.States)-1].State == types2.SuperNodeStateDisabled {

			// Get validator and perform eligibility checks for re-registration
			validator, err := k.stakingKeeper.Validator(ctx, valOperAddr)
			if err != nil {
				return nil, errorsmod.Wrapf(sdkerrors.ErrNotFound, "validator not found for operator address %s: %s", msg.ValidatorAddress, err)
			}

			// Check if validator is jailed
			if validator.IsJailed() {
				return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest,
					"validator %s is jailed and cannot re-register a supernode", msg.ValidatorAddress)
			}

			// Check eligibility - use existing supernode account for validation
			if err := k.CheckValidatorSupernodeEligibility(ctx, validator, msg.ValidatorAddress, existingSupernode.SupernodeAccount); err != nil {
				return nil, err
			}

			// Re-registration only changes state from Disabled to Active
			// Use UpdateSupernode for updating other fields
			existingSupernode.States = append(existingSupernode.States, &types2.SuperNodeStateRecord{
				State:  types2.SuperNodeStateActive,
				Height: ctx.BlockHeight(),
			})

			// Save the updated supernode
			if err := k.SetSuperNode(ctx, existingSupernode); err != nil {
				return nil, errorsmod.Wrapf(sdkerrors.ErrIO, "error updating supernode: %s", err)
			}

			// Emit event with existing supernode details
			currentIP := ""
			if len(existingSupernode.PrevIpAddresses) > 0 {
				currentIP = existingSupernode.PrevIpAddresses[len(existingSupernode.PrevIpAddresses)-1].Address
			}

			ctx.EventManager().EmitEvent(
				sdk.NewEvent(
					types2.EventTypeSupernodeRegistered,
					sdk.NewAttribute(types2.AttributeKeyValidatorAddress, msg.ValidatorAddress),
					sdk.NewAttribute(types2.AttributeKeyIPAddress, currentIP),
					sdk.NewAttribute(types2.AttributeKeySupernodeAccount, existingSupernode.SupernodeAccount),
					sdk.NewAttribute(types2.AttributeKeyReRegistered, "true"),
					sdk.NewAttribute(types2.AttributeKeyOldState, "disabled"),
					sdk.NewAttribute(types2.AttributeKeyP2PPort, existingSupernode.P2PPort),
				),
			)

			return &types2.MsgRegisterSupernodeResponse{}, nil
		}

		// If not disabled, cannot register
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "supernode already exists for validator %s", msg.ValidatorAddress)
	}

	// Get validator
	validator, err := k.stakingKeeper.Validator(ctx, valOperAddr)
	if err != nil {
		return nil, errorsmod.Wrapf(sdkerrors.ErrNotFound, "validator not found for operator address %s: %s", msg.ValidatorAddress, err)
	}

	// State-dependent validations
	if validator.IsJailed() {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest,
			"validator %s is jailed and cannot register a supernode", msg.ValidatorAddress)
	}

	if err := k.CheckValidatorSupernodeEligibility(ctx, validator, msg.ValidatorAddress, msg.SupernodeAccount); err != nil {
		return nil, err
	}

	// Create new SuperNode
	supernode := types2.SuperNode{
		ValidatorAddress: msg.ValidatorAddress,
		SupernodeAccount: msg.SupernodeAccount,
		Evidence:         []*types2.Evidence{},
		Metrics: &types2.MetricsAggregate{
			Metrics:     make(map[string]float64),
			ReportCount: 0,
		},
		States: []*types2.SuperNodeStateRecord{
			{
				State:  types2.SuperNodeStateActive,
				Height: ctx.BlockHeight(),
			},
		},
		PrevIpAddresses: []*types2.IPAddressHistory{
			{
				Address: msg.IpAddress,
				Height:  ctx.BlockHeight(),
			},
		},
		PrevSupernodeAccounts: []*types2.SupernodeAccountHistory{
			{
				Account: msg.SupernodeAccount,
				Height:  ctx.BlockHeight(),
			},
		},
		P2PPort: msg.P2PPort,
	}

	// Validate the SuperNode struct
	if err := supernode.Validate(); err != nil {
		return nil, err
	}

	// Use SetSuperNode to store the SuperNode
	if err := k.SetSuperNode(ctx, supernode); err != nil {
		return nil, errorsmod.Wrapf(sdkerrors.ErrIO, "error setting supernode: %s", err)
	}

	// Emit event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types2.EventTypeSupernodeRegistered,
			sdk.NewAttribute(types2.AttributeKeyValidatorAddress, msg.ValidatorAddress),
			sdk.NewAttribute(types2.AttributeKeyIPAddress, msg.IpAddress),
			sdk.NewAttribute(types2.AttributeKeySupernodeAccount, msg.SupernodeAccount),
			sdk.NewAttribute(types2.AttributeKeyP2PPort, msg.P2PPort),
		),
	)

	return &types2.MsgRegisterSupernodeResponse{}, nil
}
