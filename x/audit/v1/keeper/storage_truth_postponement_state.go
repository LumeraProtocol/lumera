package keeper

import (
	"encoding/binary"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func (k Keeper) getStorageTruthPostponedAtEpochID(ctx sdk.Context, supernodeAccount string) (uint64, bool) {
	store := k.kvStore(ctx)
	bz := store.Get(types.StorageTruthPostponementKey(supernodeAccount))
	if len(bz) != 8 {
		return 0, false
	}
	return binary.BigEndian.Uint64(bz), true
}

func (k Keeper) setStorageTruthPostponedAtEpochID(ctx sdk.Context, supernodeAccount string, epochID uint64) {
	store := k.kvStore(ctx)
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, epochID)
	store.Set(types.StorageTruthPostponementKey(supernodeAccount), bz)
}

func (k Keeper) clearStorageTruthPostponedAtEpochID(ctx sdk.Context, supernodeAccount string) {
	store := k.kvStore(ctx)
	store.Delete(types.StorageTruthPostponementKey(supernodeAccount))
}
