package keeper

import (
	"encoding/binary"

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
