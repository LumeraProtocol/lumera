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
	_, found := k.QuerySuperNode(ctx, valOperAddr)
	if found {
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

	if err := k.CheckValidatorSupernodeEligibility(ctx, validator, msg.ValidatorAddress); err != nil {
		return nil, err
	}

	// Create new SuperNode
	supernode := types2.SuperNode{
		ValidatorAddress: msg.ValidatorAddress,
		SupernodeAccount: msg.SupernodeAccount,
		Evidence:         []*types2.Evidence{},
		Version:          msg.Version,
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
			sdk.NewAttribute(types2.AttributeKeyVersion, msg.Version),
		),
	)

	return &types2.MsgRegisterSupernodeResponse{}, nil
}
