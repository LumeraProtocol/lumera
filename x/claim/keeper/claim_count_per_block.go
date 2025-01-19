package keeper

import (
	"github.com/LumeraProtocol/lumera/x/claim/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"
)

func (k Keeper) GetBlockClaimCount(ctx sdk.Context) uint64 {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, []byte(types.BlockClaimsKey))
	bz := store.Get([]byte(types.BlockClaimsKey))
	if bz == nil {
		return 0
	}
	return sdk.BigEndianToUint64(bz)
}

func (k Keeper) IncrementBlockClaimCount(ctx sdk.Context) {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, []byte(types.BlockClaimsKey))
	count := k.GetBlockClaimCount(ctx) + 1
	store.Set([]byte(types.BlockClaimsKey), sdk.Uint64ToBigEndian(count))
}

func (k Keeper) ResetBlockClaimCount(ctx sdk.Context) {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, []byte(types.BlockClaimsKey))
	store.Set([]byte(types.BlockClaimsKey), sdk.Uint64ToBigEndian(0))
}
