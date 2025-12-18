package keeper

import (
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// SetMetricsState stores the latest SupernodeMetricsState for a validator.
func (k Keeper) SetMetricsState(ctx sdk.Context, state types.SupernodeMetricsState) error {
	store := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))

	valAddr, err := sdk.ValAddressFromBech32(state.ValidatorAddress)
	if err != nil {
		return err
	}

	bz, err := k.cdc.Marshal(&state)
	if err != nil {
		return err
	}

	store.Set(types.GetMetricsStateKey(valAddr), bz)
	return nil
}

// GetMetricsState retrieves the latest SupernodeMetricsState for a validator, if any.
func (k Keeper) GetMetricsState(ctx sdk.Context, valAddr sdk.ValAddress) (types.SupernodeMetricsState, bool) {
	store := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))

	key := types.GetMetricsStateKey(valAddr)
	bz := store.Get(key)
	if bz == nil {
		return types.SupernodeMetricsState{}, false
	}

	var state types.SupernodeMetricsState
	if err := k.cdc.Unmarshal(bz, &state); err != nil {
		k.logger.Error("failed to unmarshal SupernodeMetricsState", "err", err)
		return types.SupernodeMetricsState{}, false
	}

	return state, true
}

