package keeper

import (
	"encoding/binary"

	storetypes "cosmossdk.io/store/types"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) getActionFinalizationPostponedAtEpochID(ctx sdk.Context, supernodeAccount string) (uint64, bool) {
	store := k.kvStore(ctx)
	bz := store.Get(types.ActionFinalizationPostponementKey(supernodeAccount))
	if len(bz) != 8 {
		return 0, false
	}
	return binary.BigEndian.Uint64(bz), true
}

func (k Keeper) setActionFinalizationPostponedAtEpochID(ctx sdk.Context, supernodeAccount string, epochID uint64) {
	store := k.kvStore(ctx)
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, epochID)
	store.Set(types.ActionFinalizationPostponementKey(supernodeAccount), bz)
}

func (k Keeper) clearActionFinalizationPostponedAtEpochID(ctx sdk.Context, supernodeAccount string) {
	store := k.kvStore(ctx)
	store.Delete(types.ActionFinalizationPostponementKey(supernodeAccount))
}

// GetAllActionFinalizationPostponements returns all active action-finalization
// postponement markers for genesis export. Per final-gate F-B2.
func (k Keeper) GetAllActionFinalizationPostponements(ctx sdk.Context) []types.GenesisActionFinalizationPostponement {
	store := k.kvStore(ctx)
	prefix := types.ActionFinalizationPostponementPrefix()
	it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer func() { _ = it.Close() }()

	var out []types.GenesisActionFinalizationPostponement
	for ; it.Valid(); it.Next() {
		key := it.Key()
		bz := it.Value()
		if len(key) <= len(prefix) || len(bz) != 8 {
			continue
		}
		out = append(out, types.GenesisActionFinalizationPostponement{
			SupernodeAccount:   string(key[len(prefix):]),
			PostponedAtEpochId: binary.BigEndian.Uint64(bz),
		})
	}
	return out
}
