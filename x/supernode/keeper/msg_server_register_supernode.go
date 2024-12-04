package keeper

import (
	"context"
	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pastelnetwork/pastel/x/supernode/types"
)

func (k msgServer) RegisterSupernode(goCtx context.Context, msg *types.MsgRegisterSupernode) (*types.MsgRegisterSupernodeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Convert validator address string to ValAddress
	valOperAddr, err := sdk.ValAddressFromBech32(msg.ValidatorAddress)
	if err != nil {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid validator address: %s", err)
	}

	// Get the validator by operator address using the correct method
	validator, err := k.stakingKeeper.Validator(ctx, valOperAddr)
	if err != nil {
		return nil, errorsmod.Wrapf(sdkerrors.ErrNotFound, "validator not found for operator address %s: %s", msg.ValidatorAddress, err)
	}

	// First check: reject if validator is jailed
	if validator.IsJailed() {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest,
			"validator %s is jailed and cannot register a supernode", msg.ValidatorAddress)
	}

	// Convert creator address (cosmos1...) to AccAddress for comparison
	creatorAddr, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address: %s", err)
	}

	// Verify the creator matches the validator's operator account
	valAccAddr := sdk.AccAddress(valOperAddr) // Convert ValAddress to AccAddress for comparison
	if !creatorAddr.Equals(valAccAddr) {
		return nil, errorsmod.Wrapf(sdkerrors.ErrUnauthorized,
			"creator account %s is not the validator operator account %s",
			creatorAddr, valAccAddr)
	}

	// Get store from storeService
	store := k.storeService.OpenKVStore(ctx)

	// Check if a SuperNode already exists for this validator
	key := types.GetSupernodeKey(valOperAddr)
	has, err := store.Has(key)
	if err != nil {
		return nil, err
	}
	if has {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "supernode already exists for validator %s", msg.ValidatorAddress)
	}

	// Check eligibility: either bonded (in active set) OR has minimum stake
	isBonded := validator.IsBonded()
	if !isBonded {
		// Not in active set, check minimum stake
		minStake := k.GetParams(ctx).MinimumStakeForSn
		stake := validator.GetTokens()
		minStakeInt := math.NewIntFromUint64(minStake)
		if stake.LT(minStakeInt) {
			return nil, errorsmod.Wrapf(
				sdkerrors.ErrInvalidRequest,
				"validator %s is not in active set and does not meet minimum stake requirement. Required: %d, Got: %s",
				msg.ValidatorAddress,
				minStake,
				stake,
			)
		}
	}

	// Create new SuperNode
	supernode := types.SuperNode{
		ValidatorAddress: msg.ValidatorAddress,
		IpAddress:        msg.IpAddress,
		State:            types.Active,
		Evidence:         []*types.Evidence{},
		LastTimeActive:   ctx.BlockTime(),
		StartedAt:        ctx.BlockTime(),
		Version:          msg.Version,
		Metrics: &types.MetricsAggregate{
			Metrics:     make(map[string]float64),
			ReportCount: 0,
			LastUpdated: ctx.BlockTime(),
		},
	}

	// Validate the SuperNode struct
	if err := supernode.Validate(); err != nil {
		return nil, err
	}

	// Store the SuperNode
	bz := k.cdc.MustMarshal(&supernode)
	err = store.Set(key, bz)
	if err != nil {
		return nil, err
	}

	// Emit event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeSupernodeRegistered,
			sdk.NewAttribute(types.AttributeKeyValidatorAddress, msg.ValidatorAddress),
			sdk.NewAttribute(types.AttributeKeyIPAddress, msg.IpAddress),
			sdk.NewAttribute(types.AttributeKeyVersion, msg.Version),
		),
	)

	return &types.MsgRegisterSupernodeResponse{}, nil
}
