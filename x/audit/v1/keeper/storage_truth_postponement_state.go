package keeper

import (
	"encoding/binary"

	storetypes "cosmossdk.io/store/types"
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
	store.Delete(types.StorageTruthPostponementStrongKey(supernodeAccount))
}

// hasStorageTruthStrongPostponeMarker reports whether the postponement record
// at supernodeAccount was created from the strong-suspicion band.
// Per F121-F12 — distinguishes recovery requirements between bands.
func (k Keeper) hasStorageTruthStrongPostponeMarker(ctx sdk.Context, supernodeAccount string) bool {
	store := k.kvStore(ctx)
	return store.Has(types.StorageTruthPostponementStrongKey(supernodeAccount))
}

// setStorageTruthStrongPostponeMarker records that the postponement was triggered
// by the strong-suspicion band. Recovery uses StrongRecoveryCleanPassCount param.
func (k Keeper) setStorageTruthStrongPostponeMarker(ctx sdk.Context, supernodeAccount string) {
	store := k.kvStore(ctx)
	store.Set(types.StorageTruthPostponementStrongKey(supernodeAccount), []byte{1})
}

// GetAllStorageTruthPostponements returns all active postponement markers.
// Per 121-F7 — needed for genesis export so postponements survive chain restart.
func (k Keeper) GetAllStorageTruthPostponements(ctx sdk.Context) []types.StorageTruthPostponement {
	store := k.kvStore(ctx)
	prefix := types.StorageTruthPostponementPrefix()
	it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer it.Close()

	var out []types.StorageTruthPostponement
	for ; it.Valid(); it.Next() {
		key := it.Key()
		account := string(key[len(prefix):])
		bz := it.Value()
		if len(bz) != 8 {
			continue
		}
		epochID := binary.BigEndian.Uint64(bz)
		out = append(out, types.StorageTruthPostponement{
			SupernodeAccount:    account,
			PostponedAtEpochId: epochID,
		})
	}
	return out
}
