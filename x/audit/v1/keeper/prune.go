package keeper

import (
	"encoding/binary"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// PruneOldEpochs bounds audit storage by epoch_id, keeping only the last params.keep_last_epoch_entries epochs
// (including the provided currentEpochID).
func (k Keeper) PruneOldEpochs(ctx sdk.Context, currentEpochID uint64, params types.Params) error {
	params = params.WithDefaults()
	keepLastEpochEntries := params.KeepLastEpochEntries

	// Keep [minKeepEpochID .. currentEpochID].
	var minKeepEpochID uint64
	if currentEpochID+1 > keepLastEpochEntries {
		minKeepEpochID = currentEpochID + 1 - keepLastEpochEntries
	} else {
		minKeepEpochID = 0
	}

	store := k.kvStore(ctx)

	// Epoch anchors: ea/<u64be(epoch_id)>
	if err := prunePrefixByWindowIDLeadingU64(store, types.EpochAnchorPrefix(), minKeepEpochID); err != nil {
		return err
	}

	// Reports: r/<u64be(epoch_id)><reporter>
	if err := prunePrefixByWindowIDLeadingU64(store, []byte("r/"), minKeepEpochID); err != nil {
		return err
	}

	// Indices:
	// - ri/<reporter>/<u64be(epoch_id)>
	// - ss/<reporter>/<u64be(epoch_id)>
	// - sr/<supernode>/<u64be(epoch_id)>/<reporter>
	pruneReporterTrailingWindowID(store, []byte("ri/"), minKeepEpochID)
	pruneReporterTrailingWindowID(store, []byte("ss/"), minKeepEpochID)
	pruneSupernodeWindowReporter(store, []byte("sr/"), minKeepEpochID)

	// Evidence epoch counts: eve/<u64be(epoch_id)>/...
	if err := prunePrefixByWindowIDLeadingU64(store, types.EvidenceEpochCountPrefix(), minKeepEpochID); err != nil {
		return err
	}

	return nil
}

func prunePrefixByWindowIDLeadingU64(store storetypes.KVStore, prefix []byte, minKeepEpochID uint64) error {
	it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer it.Close()

	var toDelete [][]byte

	for ; it.Valid(); it.Next() {
		key := it.Key()
		if len(key) < len(prefix)+8 {
			// Malformed; skip.
			continue
		}
		epochID := binary.BigEndian.Uint64(key[len(prefix) : len(prefix)+8])
		if epochID >= minKeepEpochID {
			// Keys are ordered by leading u64be(epoch_id); we can stop.
			break
		}
		// Copy key before iterator advances.
		kc := make([]byte, len(key))
		copy(kc, key)
		toDelete = append(toDelete, kc)
	}

	for _, k := range toDelete {
		store.Delete(k)
	}
	return nil
}

// pruneReporterTrailingWindowID prunes keys shaped like:
//
//	<prefix><account>"/"<u64be(epoch_id)>
//
// by parsing the final 8 bytes as the epoch id.
func pruneReporterTrailingWindowID(store storetypes.KVStore, prefix []byte, minKeepWindowID uint64) {
	it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer it.Close()

	var toDelete [][]byte

	for ; it.Valid(); it.Next() {
		key := it.Key()
		if len(key) < len(prefix)+1+8 {
			continue
		}
		epochID := binary.BigEndian.Uint64(key[len(key)-8:])
		if epochID >= minKeepWindowID {
			continue
		}
		kc := make([]byte, len(key))
		copy(kc, key)
		toDelete = append(toDelete, kc)
	}

	for _, k := range toDelete {
		store.Delete(k)
	}
}

// pruneSupernodeWindowReporter prunes keys shaped like:
//
//	sr/<supernode>"/"<u64be(epoch_id)>"/"<reporter>
func pruneSupernodeWindowReporter(store storetypes.KVStore, prefix []byte, minKeepWindowID uint64) {
	it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer it.Close()

	var toDelete [][]byte

	for ; it.Valid(); it.Next() {
		key := it.Key()
		if len(key) < len(prefix)+1+8+1+1 {
			continue
		}
		rest := key[len(prefix):]
		sep := bytesIndexByte(rest, '/')
		if sep <= 0 {
			continue
		}
		if len(rest) < sep+1+8+1 {
			continue
		}
		epochIDStart := sep + 1
		epochIDEnd := epochIDStart + 8
		epochID := binary.BigEndian.Uint64(rest[epochIDStart:epochIDEnd])
		if epochID >= minKeepWindowID {
			continue
		}
		kc := make([]byte, len(key))
		copy(kc, key)
		toDelete = append(toDelete, kc)
	}

	for _, k := range toDelete {
		store.Delete(k)
	}
}

func bytesIndexByte(b []byte, c byte) int {
	for i := range b {
		if b[i] == c {
			return i
		}
	}
	return -1
}
