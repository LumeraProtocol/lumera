package keeper

import (
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) kvStore(ctx sdk.Context) storetypes.KVStore {
	return runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
}
