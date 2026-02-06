package keeper

import (
	"context"

	"github.com/cosmos/cosmos-sdk/runtime"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func (k Keeper) GetParams(ctx context.Context) (params types.Params) {
	// Params are stored as a single blob under `types.ParamsKey`. They are initially set at
	// genesis (`keeper.InitGenesis`), and can later be updated via governance (`MsgUpdateParams`)
	// subject to immutability constraints (epoch cadence).
	store := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	bz := store.Get(types.ParamsKey)
	if bz == nil {
		return types.DefaultParams()
	}

	k.cdc.MustUnmarshal(bz, &params)
	return params.WithDefaults()
}

func (k Keeper) SetParams(ctx context.Context, params types.Params) error {
	// Always store params with defaults applied, so zero values in state never imply "unset".
	store := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	params = params.WithDefaults()

	bz, err := k.cdc.Marshal(&params)
	if err != nil {
		return err
	}

	store.Set(types.ParamsKey, bz)
	return nil
}
