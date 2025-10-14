package keeper

import (
	"fmt"
	"strconv"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store/prefix"

	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/query"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// SetSuperNode sets a supernode record in the store
func (k Keeper) SetSuperNode(ctx sdk.Context, supernode types.SuperNode) error {
	if err := supernode.Validate(); err != nil {
		return err
	}

	// Convert context store to a KVStore interface
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	// Create a prefix store so that all keys are under SuperNodeKey
	store := prefix.NewStore(storeAdapter, []byte(types.SuperNodeKey))

	// Marshal the SuperNode into bytes
	b, err := k.cdc.Marshal(&supernode)
	if err != nil {
		return err
	}

	// Use the validator address as the key (since it's unique).
	valOperAddr, err := sdk.ValAddressFromBech32(supernode.ValidatorAddress)
	if err != nil {
		return errorsmod.Wrapf(err, "invalid validator address: %s", err)
	}

	// Set the supernode record under [SuperNodeKeyPrefix + valOperAddr]
	// Note: prefix.NewStore automatically prepends the prefix we defined above.
	store.Set(valOperAddr, b)

	return nil
}

// QuerySuperNode returns the supernode record for a given validator address
func (k Keeper) QuerySuperNode(ctx sdk.Context, valOperAddr sdk.ValAddress) (sn types.SuperNode, exists bool) {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, []byte(types.SuperNodeKey))

	bz := store.Get(valOperAddr)
	if bz == nil {
		return types.SuperNode{}, false
	}

	if err := k.cdc.Unmarshal(bz, &sn); err != nil {
		k.logger.Error(fmt.Sprintf("failed to unmarshal supernode: %s", err))
		return types.SuperNode{}, false
	}

	return sn, true
}

// GetAllSuperNodes returns all supernodes, optionally filtered by state
func (k Keeper) GetAllSuperNodes(ctx sdk.Context, stateFilters ...types.SuperNodeState) ([]types.SuperNode, error) {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, []byte(types.SuperNodeKey))

	iterator := store.Iterator(nil, nil)
	defer iterator.Close()

	var supernodes []types.SuperNode
	filtering := shouldFilter(stateFilters...)

	for ; iterator.Valid(); iterator.Next() {
		bz := iterator.Value()
		var sn types.SuperNode
		if err := k.cdc.Unmarshal(bz, &sn); err != nil {
			return nil, fmt.Errorf("failed to unmarshal supernode: %w", err)
		}

		// skip if no states at all
		if len(sn.States) == 0 {
			continue
		}

		// if we're not filtering or the current state is in the filter list, add it
		if !filtering || stateIn(sn.States[len(sn.States)-1].State, stateFilters...) {
			supernodes = append(supernodes, sn)
		}
	}

	return supernodes, nil
}

// GetSuperNodesPaginated returns paginated supernodes, optionally filtered by state
func (k Keeper) GetSuperNodesPaginated(ctx sdk.Context, pagination *query.PageRequest, stateFilters ...types.SuperNodeState) ([]*types.SuperNode, *query.PageResponse, error) {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, []byte(types.SuperNodeKey))

	var supernodes []*types.SuperNode
	filtering := shouldFilter(stateFilters...)

	pageRes, err := query.Paginate(store, pagination, func(key, value []byte) error {
		var sn types.SuperNode
		if err := k.cdc.Unmarshal(value, &sn); err != nil {
			return err
		}

		if len(sn.States) == 0 {
			return nil
		}

		if !filtering || stateIn(sn.States[len(sn.States)-1].State, stateFilters...) {
			supernodes = append(supernodes, &sn)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return supernodes, pageRes, nil
}

func (k Keeper) GetMinStake(ctx sdk.Context) sdkmath.Int {
	minStake := k.GetParams(ctx).MinimumStakeForSn
	minStakeInt := minStake.Amount
	return minStakeInt
}

// SetSuperNodeActive sets a validator's SuperNode status to active and emits an event.
// If reason is non-empty, it is included as an event attribute.
func (k Keeper) SetSuperNodeActive(ctx sdk.Context, valAddr sdk.ValAddress, reason string) error {
	valOperAddr, err := sdk.ValAddressFromBech32(valAddr.String())
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid validator address: %s", err)
	}

	supernode, found := k.QuerySuperNode(ctx, valOperAddr)
	if !found {
		return errorsmod.Wrapf(sdkerrors.ErrNotFound, "no supernode found for validator")
	}

	if len(supernode.States) == 0 {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "supernode is in an invalid state")
	}

	currentState := supernode.States[len(supernode.States)-1].State

	switch currentState {
	case types.SuperNodeStateDisabled:
		// Cannot enable if disabled - must be re-registered
		return nil // Silently ignore - disabled supernodes need re-registration
	case types.SuperNodeStateStopped:
		// Only enable if currently stopped
		supernode.States = append(supernode.States, &types.SuperNodeStateRecord{
			State:  types.SuperNodeStateActive,
			Height: ctx.BlockHeight(),
		})
	case types.SuperNodeStateActive:
		// Already active, nothing to do
		return nil
	}
	if err := k.SetSuperNode(ctx, supernode); err != nil {
		k.logger.With("module", fmt.Sprintf("error updating supernode state: %s", valAddr)).Error(fmt.Sprintf("x/%s", types.ModuleName))
		return errorsmod.Wrapf(sdkerrors.ErrIO, "error updating supernode state")
	}
	// Emit event for watchers
	attrs := []sdk.Attribute{
		sdk.NewAttribute(types.AttributeKeyValidatorAddress, supernode.ValidatorAddress),
		sdk.NewAttribute(types.AttributeKeyOldState, currentState.String()),
		sdk.NewAttribute(types.AttributeKeyHeight, strconv.FormatInt(ctx.BlockHeight(), 10)),
	}
	if reason != "" {
		attrs = append(attrs, sdk.NewAttribute(types.AttributeKeyReason, reason))
	}
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeSupernodeStarted,
			attrs...,
		),
	)

	return nil
}

// SetSuperNodeStopped sets a validator's SuperNode status to stopped and emits an event.
// If reason is non-empty, it is included as an event attribute.
func (k Keeper) SetSuperNodeStopped(ctx sdk.Context, valAddr sdk.ValAddress, reason string) error {
	valOperAddr, err := sdk.ValAddressFromBech32(valAddr.String())
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid validator address: %s", err)
	}

	supernode, found := k.QuerySuperNode(ctx, valOperAddr)
	if !found {
		return errorsmod.Wrapf(sdkerrors.ErrNotFound, "no supernode found for validator")
	}

	if len(supernode.States) == 0 {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "supernode is in an invalid state")
	}

	currentState := supernode.States[len(supernode.States)-1].State
	// Only stop if currently active - ignore if already stopped or disabled
	if currentState == types.SuperNodeStateActive {
		supernode.States = append(supernode.States, &types.SuperNodeStateRecord{
			State:  types.SuperNodeStateStopped,
			Height: ctx.BlockHeight(),
		})
	} else {
		// If already stopped or disabled, return without error
		return nil
	}

	if err := k.SetSuperNode(ctx, supernode); err != nil {
		k.logger.With("module", fmt.Sprintf("error updating supernode state: %s", valAddr)).Error(fmt.Sprintf("x/%s", types.ModuleName))
		return errorsmod.Wrapf(sdkerrors.ErrIO, "error updating supernode state")
	}

	// Emit event
	attrs := []sdk.Attribute{
		sdk.NewAttribute(types.AttributeKeyValidatorAddress, supernode.ValidatorAddress),
		sdk.NewAttribute(types.AttributeKeyOldState, currentState.String()),
		sdk.NewAttribute(types.AttributeKeyHeight, strconv.FormatInt(ctx.BlockHeight(), 10)),
	}
	if reason != "" {
		attrs = append(attrs, sdk.NewAttribute(types.AttributeKeyReason, reason))
	}
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeSupernodeStopped,
			attrs...,
		),
	)

	return nil
}

func (k Keeper) IsSuperNodeActive(ctx sdk.Context, valAddr sdk.ValAddress) bool {
	valOperAddr, err := sdk.ValAddressFromBech32(valAddr.String())
	if err != nil {
		return false
	}

	supernode, found := k.QuerySuperNode(ctx, valOperAddr)
	if !found {
		return false
	}

	if len(supernode.States) == 0 {
		return false
	}

	return supernode.States[len(supernode.States)-1].State == types.SuperNodeStateActive
}

// CheckValidatorSupernodeEligibility ensures the validator has enough stake from either self-delegation
// or delegation from the supernode account.
// If supernodeAccount is provided, it will check for delegation from that account.
// If supernodeAccount is nil, it will try to find the supernode in the state and check for delegation from its account.
func (k Keeper) CheckValidatorSupernodeEligibility(ctx sdk.Context, validator stakingtypes.ValidatorI, valAddr string, supernodeAccount string) error {

	// 1. Get chain's configured minimum self-stake
	minStake := k.GetParams(ctx).MinimumStakeForSn
	minStakeInt := minStake

	// 2. Convert operator address (valAddr) into types
	valOperatorAddr, err := sdk.ValAddressFromBech32(valAddr)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid validator address: %s", valAddr)
	}
	valAccAddr := sdk.AccAddress(valOperatorAddr)

	// 3. Get self-delegation record
	selfDelegation, err := k.stakingKeeper.Delegation(ctx, valAccAddr, valOperatorAddr)
	if err != nil || selfDelegation == nil {
		return errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest,
			"validator %s has no self-delegation; cannot meet minimum self-stake requirement",
			valAddr,
		)
	}

	// 4. Guard: if validator's DelegatorShares == 0, we can't compute tokens from shares
	if validator.GetDelegatorShares().IsZero() {
		return errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest,
			"validator %s has zero delegator shares; no self-stake available",
			valAddr,
		)
	}

	// 5. Convert the self-delegation shares to actual tokens (decimal)
	selfDelegatedTokens := validator.TokensFromShares(selfDelegation.GetShares())

	// 6. Convert decimal -> integer
	selfDelegatedTokensInt := selfDelegatedTokens.TruncateInt()

	// 7. Check if there's a supernode account with delegation to this validator
	// Initialize to zero
	supernodeDelegatedTokensInt := sdkmath.ZeroInt()

	// If supernodeAccount is provided, use it directly
	if supernodeAccount != "" {
		// Get the supernode account address
		supernodeAccAddr, err := sdk.AccAddressFromBech32(supernodeAccount)
		if err == nil {
			// Check if there's a delegation from the supernode account to this validator
			supernodeDelegation, err := k.stakingKeeper.Delegation(ctx, supernodeAccAddr, valOperatorAddr)
			if err == nil && supernodeDelegation != nil {
				// Convert the supernode delegation shares to tokens
				supernodeDelegatedTokens := validator.TokensFromShares(supernodeDelegation.GetShares())
				supernodeDelegatedTokensInt = supernodeDelegatedTokens.TruncateInt()
			}
		}
	} else {
		// If supernodeAccount is not provided, try to find the supernode in the state
		supernode, found := k.QuerySuperNode(ctx, valOperatorAddr)
		if found && supernode.SupernodeAccount != "" {
			// Get the supernode account address
			supernodeAccAddr, err := sdk.AccAddressFromBech32(supernode.SupernodeAccount)
			if err == nil {
				// Check if there's a delegation from the supernode account to this validator
				supernodeDelegation, err := k.stakingKeeper.Delegation(ctx, supernodeAccAddr, valOperatorAddr)
				if err == nil && supernodeDelegation != nil {
					// Convert the supernode delegation shares to tokens
					supernodeDelegatedTokens := validator.TokensFromShares(supernodeDelegation.GetShares())
					supernodeDelegatedTokensInt = supernodeDelegatedTokens.TruncateInt()
				}
			}
		}
	}

	// 8. Add self-delegation and supernode delegation
	totalDelegatedTokensInt := selfDelegatedTokensInt.Add(supernodeDelegatedTokensInt)

	// 9. Compare total delegation to minimum stake requirement
	if totalDelegatedTokensInt.LT(minStakeInt.Amount) {
		return errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest,
			"validator %s does not meet minimum stake requirement. Required: %s, got: %s (self: %s, supernode: %s)",
			valAddr,
			minStake.Amount.String(),
			totalDelegatedTokensInt.String(),
			selfDelegatedTokensInt.String(),
			supernodeDelegatedTokensInt.String(),
		)
	}

	return nil
}

func (k Keeper) IsEligibleAndNotJailedValidator(ctx sdk.Context, valAddr sdk.ValAddress) bool {
	validator, err := k.stakingKeeper.Validator(ctx, valAddr)
	if err != nil || validator == nil {
		return false
	}

	// Check advanced rules (like min self-stake, not jailed, etc.)
	if validator.IsJailed() {
		// If you want to allow jailed but not sure, typically it's false
		return false
	}

	err = k.CheckValidatorSupernodeEligibility(ctx, validator, valAddr.String(), "")
	return err == nil
}

func stateIn(state types.SuperNodeState, stateFilters ...types.SuperNodeState) bool {
	for _, sf := range stateFilters {
		if sf == state {
			return true
		}
	}
	return false
}

func shouldFilter(stateFilters ...types.SuperNodeState) bool {
	if len(stateFilters) == 0 {
		return false
	}
	// If SuperNodeStateUnspecified is present, it means no filtering
	for _, sf := range stateFilters {
		if sf == types.SuperNodeStateUnspecified {
			return false
		}
	}
	return true
}
func VerifyValidatorOperator(valOperAddr sdk.ValAddress, creator string) error {
	creatorAddr, err := sdk.AccAddressFromBech32(creator)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address: %s", err)
	}

	valAccAddr := sdk.AccAddress(valOperAddr)
	if !creatorAddr.Equals(valAccAddr) {
		return errorsmod.Wrapf(sdkerrors.ErrUnauthorized,
			"creator account %s is not the validator operator account %s",
			creatorAddr, valAccAddr)
	}

	return nil
}
