package keeper

import (
	"encoding/binary"
	"sort"

	"cosmossdk.io/store/prefix"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) GetNextEvidenceID(ctx sdk.Context) uint64 {
	store := k.kvStore(ctx)
	bz := store.Get(types.NextEvidenceIDKey())
	if bz == nil {
		return 1
	}
	if len(bz) != 8 {
		// Avoid evidence ID reuse if the stored counter is malformed by deriving the next ID from state.
		return k.deriveNextEvidenceID(ctx)
	}
	id := binary.BigEndian.Uint64(bz)
	if id == 0 {
		// Evidence IDs start at 1; treat 0 as invalid/corrupt and derive a safe next ID.
		return k.deriveNextEvidenceID(ctx)
	}
	return id
}

func (k Keeper) deriveNextEvidenceID(ctx sdk.Context) uint64 {
	store := prefix.NewStore(k.kvStore(ctx), types.EvidenceRecordPrefix())

	iter := store.Iterator(nil, nil)
	defer iter.Close()

	var maxID uint64
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		if len(key) != 8 {
			continue
		}
		id := binary.BigEndian.Uint64(key)
		if id > maxID {
			maxID = id
		}
	}
	return maxID + 1
}

func (k Keeper) SetNextEvidenceID(ctx sdk.Context, nextID uint64) {
	store := k.kvStore(ctx)
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, nextID)
	store.Set(types.NextEvidenceIDKey(), bz)
}

func (k Keeper) GetEvidence(ctx sdk.Context, evidenceID uint64) (types.Evidence, bool) {
	store := k.kvStore(ctx)
	bz := store.Get(types.EvidenceKey(evidenceID))
	if bz == nil {
		return types.Evidence{}, false
	}
	var ev types.Evidence
	k.cdc.MustUnmarshal(bz, &ev)
	return ev, true
}

func (k Keeper) SetEvidence(ctx sdk.Context, ev types.Evidence) error {
	store := k.kvStore(ctx)
	bz, err := k.cdc.Marshal(&ev)
	if err != nil {
		return err
	}
	store.Set(types.EvidenceKey(ev.EvidenceId), bz)
	return nil
}

func (k Keeper) SetEvidenceBySubjectIndex(ctx sdk.Context, subjectAddress string, evidenceID uint64) {
	store := k.kvStore(ctx)
	store.Set(types.EvidenceBySubjectIndexKey(subjectAddress, evidenceID), []byte{1})
}

func (k Keeper) SetEvidenceByActionIndex(ctx sdk.Context, actionID string, evidenceID uint64) {
	store := k.kvStore(ctx)
	store.Set(types.EvidenceByActionIndexKey(actionID, evidenceID), []byte{1})
}

func (k Keeper) GetAllEvidence(ctx sdk.Context) ([]types.Evidence, error) {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, types.EvidenceRecordPrefix())

	iter := store.Iterator(nil, nil)
	defer iter.Close()

	evidence := make([]types.Evidence, 0)
	for ; iter.Valid(); iter.Next() {
		var ev types.Evidence
		k.cdc.MustUnmarshal(iter.Value(), &ev)
		evidence = append(evidence, ev)
	}

	sort.Slice(evidence, func(i, j int) bool {
		return evidence[i].EvidenceId < evidence[j].EvidenceId
	})

	return evidence, nil
}
