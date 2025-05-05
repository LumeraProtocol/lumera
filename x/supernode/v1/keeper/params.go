package keeper

import (
	types2 "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// GetParams get all parameters as types.Params
func (k Keeper) GetParams(ctx sdk.Context) (params types2.Params) {
	store := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	bz := store.Get(types2.ParamsKey)
	if bz == nil {
		return params // return default or empty params if not set
	}

	k.cdc.MustUnmarshal(bz, &params)
	return params
}

// SetParams set the params
func (k Keeper) SetParams(ctx sdk.Context, params types2.Params) error {
	store := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	bz, err := k.cdc.Marshal(&params)
	if err != nil {
		return err
	}
	store.Set(types2.ParamsKey, bz)

	return nil
}
