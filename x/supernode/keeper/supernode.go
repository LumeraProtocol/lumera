package keeper

import (
	"fmt"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store/prefix"

	"github.com/LumeraProtocol/lumera/x/supernode/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/query"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
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
		// skip if no states at all
		if len(sn.States) == 0 {
			continue
		}

		// if we're not filtering or the current state is in the filter list, add it
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
	minStakeInt := sdkmath.NewIntFromUint64(minStake)
	return minStakeInt
}

// EnableSuperNode enables a validator's SuperNode status
func (k Keeper) EnableSuperNode(ctx sdk.Context, valAddr sdk.ValAddress) error {
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

	if supernode.States[len(supernode.States)-1].State != types.SuperNodeStateActive {
		supernode.States = append(supernode.States, &types.SuperNodeStateRecord{
			State:  types.SuperNodeStateActive,
			Height: ctx.BlockHeight(),
		})
	}
	if err := k.SetSuperNode(ctx, supernode); err != nil {
		k.logger.With("module", fmt.Sprintf("error updating supernode state: %s", valAddr)).Error(fmt.Sprintf("x/%s", types.ModuleName))
		return errorsmod.Wrapf(sdkerrors.ErrNotFound, "eror updating supernode state")
	}
	// Emit event for watchers
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeSupernodeStarted,
			sdk.NewAttribute(types.AttributeKeyValidatorAddress, supernode.ValidatorAddress),
			sdk.NewAttribute(types.AttributeKeyReason, "enable_supernode"),
		),
	)

	return nil
}

// DisableSuperNode disables a validator's SuperNode status
func (k Keeper) DisableSuperNode(ctx sdk.Context, valAddr sdk.ValAddress) error {
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

	if supernode.States[len(supernode.States)-1].State != types.SuperNodeStateDisabled {
		supernode.States = append(supernode.States, &types.SuperNodeStateRecord{
			State:  types.SuperNodeStateDisabled,
			Height: ctx.BlockHeight(),
		})
	}

	if err := k.SetSuperNode(ctx, supernode); err != nil {
		k.logger.With("module", fmt.Sprintf("error updating supernode state: %s", valAddr)).Error(fmt.Sprintf("x/%s", types.ModuleName))
		return errorsmod.Wrapf(sdkerrors.ErrNotFound, "eror updating supernode state")
	}

	// Emit event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeSupernodeStopped,
			sdk.NewAttribute(types.AttributeKeyValidatorAddress, supernode.ValidatorAddress),
			sdk.NewAttribute(types.AttributeKeyReason, "disable_supernode"),
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

// CheckValidatorSupernodeEligibility ensures the validator has enough self-stake.
func (k Keeper) CheckValidatorSupernodeEligibility(ctx sdk.Context, validator stakingtypes.ValidatorI, valAddr string) error {

	// 1. Get chain's configured minimum self-stake
	minStake := k.GetParams(ctx).MinimumStakeForSn
	minStakeInt := sdkmath.NewIntFromUint64(minStake)

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

	// 7. Compare two Ints: selfDelegatedTokensInt vs. minStakeInt
	if selfDelegatedTokensInt.LT(minStakeInt) {
		return errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest,
			"validator %s does not meet minimum self stake requirement. Required: %d, got: %s",
			valAddr,
			minStake,
			selfDelegatedTokensInt.String(),
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

	err = k.CheckValidatorSupernodeEligibility(ctx, validator, valAddr.String())
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
