package keeper

import (
	"context"

	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/pastelnetwork/pasteld/x/pastelid/types"
)

// SetPastelidEntry set a specific pastelidEntry in the store from its index
func (k Keeper) SetPastelidEntry(ctx context.Context, pastelidEntry types.PastelidEntry) {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, types.KeyPrefix(types.PastelidEntryKeyPrefix))
	b := k.cdc.MustMarshal(&pastelidEntry)
	store.Set(types.PastelidEntryKey(
		pastelidEntry.Address,
	), b)
}

// GetPastelidEntry returns a pastelidEntry from its index
func (k Keeper) GetPastelidEntry(
	ctx context.Context,
	address string,

) (val types.PastelidEntry, found bool) {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, types.KeyPrefix(types.PastelidEntryKeyPrefix))

	b := store.Get(types.PastelidEntryKey(
		address,
	))
	if b == nil {
		return val, false
	}

	k.cdc.MustUnmarshal(b, &val)
	return val, true
}

// RemovePastelidEntry removes a pastelidEntry from the store
func (k Keeper) RemovePastelidEntry(
	ctx context.Context,
	address string,

) {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, types.KeyPrefix(types.PastelidEntryKeyPrefix))
	store.Delete(types.PastelidEntryKey(
		address,
	))
}

// GetAllPastelidEntry returns all pastelidEntry
func (k Keeper) GetAllPastelidEntry(ctx context.Context) (list []types.PastelidEntry) {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, types.KeyPrefix(types.PastelidEntryKeyPrefix))
	iterator := storetypes.KVStorePrefixIterator(store, []byte{})

	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		var val types.PastelidEntry
		k.cdc.MustUnmarshal(iterator.Value(), &val)
		list = append(list, val)
	}

	return
}
