package keeper

import (
	"encoding/binary"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// PruneOldWindows bounds audit storage by window_id, keeping only the last params.keep_last_window_entries windows
// (including the provided currentWindowID).
func (k Keeper) PruneOldWindows(ctx sdk.Context, currentWindowID uint64, params types.Params) error {
	params = params.WithDefaults()
	keepLastWindowEntries := params.KeepLastWindowEntries

	// Keep [minKeepWindowID .. currentWindowID].
	var minKeepWindowID uint64
	if currentWindowID+1 > keepLastWindowEntries {
		minKeepWindowID = currentWindowID + 1 - keepLastWindowEntries
	} else {
		minKeepWindowID = 0
	}

	store := k.kvStore(ctx)

	// Snapshots: ws/<u64be(window_id)>
	if err := prunePrefixByWindowIDLeadingU64(store, []byte("ws/"), minKeepWindowID); err != nil {
		return err
	}

	// Reports: r/<u64be(window_id)><reporter>
	if err := prunePrefixByWindowIDLeadingU64(store, []byte("r/"), minKeepWindowID); err != nil {
		return err
	}

	// Indices:
	// - ri/<reporter>/<u64be(window_id)>
	// - ss/<reporter>/<u64be(window_id)>
	// - sr/<supernode>/<u64be(window_id)>/<reporter>
	pruneReporterTrailingWindowID(store, []byte("ri/"), minKeepWindowID)
	pruneReporterTrailingWindowID(store, []byte("ss/"), minKeepWindowID)
	pruneSupernodeWindowReporter(store, []byte("sr/"), minKeepWindowID)

	return nil
}

func prunePrefixByWindowIDLeadingU64(store storetypes.KVStore, prefix []byte, minKeepWindowID uint64) error {
	it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer it.Close()

	var toDelete [][]byte

	for ; it.Valid(); it.Next() {
		key := it.Key()
		if len(key) < len(prefix)+8 {
			// Malformed; skip.
			continue
		}
		windowID := binary.BigEndian.Uint64(key[len(prefix) : len(prefix)+8])
		if windowID >= minKeepWindowID {
			// Keys are ordered by leading u64be(window_id); we can stop.
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
//	<prefix><account>"/"<u64be(window_id)>
//
// by parsing the final 8 bytes as the window id.
func pruneReporterTrailingWindowID(store storetypes.KVStore, prefix []byte, minKeepWindowID uint64) {
	it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer it.Close()

	var toDelete [][]byte

	for ; it.Valid(); it.Next() {
		key := it.Key()
		if len(key) < len(prefix)+1+8 {
			continue
		}
		windowID := binary.BigEndian.Uint64(key[len(key)-8:])
		if windowID >= minKeepWindowID {
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
//	sr/<supernode>"/"<u64be(window_id)>"/"<reporter>
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
		windowIDStart := sep + 1
		windowIDEnd := windowIDStart + 8
		windowID := binary.BigEndian.Uint64(rest[windowIDStart:windowIDEnd])
		if windowID >= minKeepWindowID {
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
