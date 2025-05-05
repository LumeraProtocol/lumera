package keeper

import (
	"context"
	types2 "github.com/LumeraProtocol/lumera/x/action/v1/types"

	"github.com/cosmos/cosmos-sdk/runtime"
)

// GetParams get all parameters as types.Params
func (k *Keeper) GetParams(ctx context.Context) (params types2.Params) {
	store := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	bz := store.Get(types2.ParamsKey)
	if bz == nil {
		return params
	}

	k.cdc.MustUnmarshal(bz, &params)
	return params
}

// SetParams set the params
func (k *Keeper) SetParams(ctx context.Context, params types2.Params) error {
	store := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	bz, err := k.cdc.Marshal(&params)
	if err != nil {
		return err
	}
	store.Set(types2.ParamsKey, bz)

	return nil
}
