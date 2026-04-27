package keeper

import (
	"encoding/binary"
	"encoding/json"

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

	// Epoch params snapshots: eps/<u64be(epoch_id)>
	if err := prunePrefixByWindowIDLeadingU64(store, types.EpochParamsSnapshotPrefix(), minKeepEpochID); err != nil {
		return err
	}

	// Reports: r/<u64be(epoch_id)><reporter>
	if err := prunePrefixByWindowIDLeadingU64(store, types.ReportPrefix(), minKeepEpochID); err != nil {
		return err
	}

	// Indices:
	// - ri/<reporter>/<u64be(epoch_id)>
	// - hr/<reporter>/<u64be(epoch_id)>
	// - sc/<supernode>/<u64be(epoch_id)>/<reporter>
	pruneReporterTrailingWindowID(store, types.ReportIndexRootPrefix(), minKeepEpochID)
	pruneReporterTrailingWindowID(store, types.HostReportIndexRootPrefix(), minKeepEpochID)
	pruneSupernodeWindowReporter(store, types.StorageChallengeReportIndexRootPrefix(), minKeepEpochID)

	// Evidence epoch counts: eve/<u64be(epoch_id)>/...
	if err := prunePrefixByWindowIDLeadingU64(store, types.EvidenceEpochCountPrefix(), minKeepEpochID); err != nil {
		return err
	}

	// Recheck evidence dedup: st/rce/<u64be(epoch_id)>/... (epoch-leading, 121-F6)
	if err := prunePrefixByWindowIDLeadingU64(store, []byte("st/rce/"), minKeepEpochID); err != nil {
		return err
	}

	// Storage-truth fact indexes (121-F6): all keyed as <prefix><account>/<u64be(epoch_id)>/...
	// Node failure records: st/nf/<supernode_account>/<u64be(epoch_id)>/...
	pruneSupernodeWindowReporter(store, []byte("st/nf/"), minKeepEpochID)
	// Reporter result records: st/rrs/<reporter_account>/<u64be(epoch_id)>/...
	pruneSupernodeWindowReporter(store, []byte("st/rrs/"), minKeepEpochID)
	// Failed heal records: st/fh/<supernode_account>/<u64be(epoch_id)>/...
	pruneSupernodeWindowReporter(store, []byte("st/fh/"), minKeepEpochID)

	// CP3.5 secondary indexes for indexed contradiction-check lookups.
	// Reporter result by target index: st/rrs-tt/<target>/<u64be(epoch_id)>/<ticket_id>0x00<reporter>
	// Same shape as st/rrs/ (account-then-epoch), reuse helper.
	pruneSupernodeWindowReporter(store, []byte("st/rrs-tt/"), minKeepEpochID)
	// Transcript by target/bucket/epoch index: st/spt-tbe/<target>/<u32be(bucket)>/<u64be(epoch_id)>/<transcript_hash>
	pruneTargetBucketEpoch(store, []byte("st/spt-tbe/"), minKeepEpochID)
	// Primary transcript store: st/spt/<transcript_hash> -> JSON{epoch_id, ...}.
	// Records are not epoch-keyed, so decode value to filter.
	pruneStorageProofTranscripts(store, []byte("st/spt/"), minKeepEpochID)

	// Per 120-F3 — terminal heal-ops pruned to bound chain state growth.
	if err := k.pruneTerminalHealOps(ctx, currentEpochID, keepLastEpochEntries); err != nil {
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
//	sc/<supernode>"/"<u64be(epoch_id)>"/"<reporter>
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

// pruneTargetBucketEpoch prunes keys shaped like:
//
//	<prefix><target>"/"<u32be(bucket)>"/"<u64be(epoch_id)>"/"<transcript_hash>
//
// The 8-byte epoch sits after target + '/' + 4-byte bucket + '/'.
func pruneTargetBucketEpoch(store storetypes.KVStore, prefix []byte, minKeepWindowID uint64) {
	it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer it.Close()

	var toDelete [][]byte

	for ; it.Valid(); it.Next() {
		key := it.Key()
		rest := key[len(prefix):]
		// rest = <target> '/' <u32be bucket> '/' <u64be epoch> '/' <hash>
		sep := bytesIndexByte(rest, '/')
		if sep <= 0 {
			continue
		}
		// after first '/': 4-byte bucket + '/' + 8-byte epoch + '/' + hash >= 14
		if len(rest) < sep+1+4+1+8+1 {
			continue
		}
		epochStart := sep + 1 + 4 + 1
		epochEnd := epochStart + 8
		epochID := binary.BigEndian.Uint64(rest[epochStart:epochEnd])
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

// pruneStorageProofTranscripts prunes the primary transcript store st/spt/<hash> -> JSON
// by decoding the embedded epoch_id field. Records older than minKeepWindowID are deleted.
// Per roomote 122 review — bounds long-term state growth.
func pruneStorageProofTranscripts(store storetypes.KVStore, prefix []byte, minKeepWindowID uint64) {
	it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer it.Close()

	// Minimal struct to decode just the epoch_id field; tolerant of unknown fields.
	type epochProbe struct {
		EpochID uint64 `json:"epoch_id"`
	}

	var toDelete [][]byte
	for ; it.Valid(); it.Next() {
		var rec epochProbe
		if err := json.Unmarshal(it.Value(), &rec); err != nil {
			// Malformed record — leave in place; pruning must not lose data on parse error.
			continue
		}
		if rec.EpochID >= minKeepWindowID {
			continue
		}
		kc := make([]byte, len(it.Key()))
		copy(kc, it.Key())
		toDelete = append(toDelete, kc)
	}

	for _, k := range toDelete {
		store.Delete(k)
	}
}

// pruneTerminalHealOps deletes heal-ops that have reached a terminal status
// (VERIFIED, FAILED, EXPIRED) and whose scheduled epoch is old enough to be
// outside the keep window. All associated index entries are also removed.
// Per 120-F3 — terminal heal-ops pruned to bound chain state growth.
func (k Keeper) pruneTerminalHealOps(ctx sdk.Context, currentEpochID, keepLastEpochEntries uint64) error {
	store := k.kvStore(ctx)
	prefix := types.HealOpPrefix()
	it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer it.Close()

	var toDelete []types.HealOp
	for ; it.Valid(); it.Next() {
		var healOp types.HealOp
		k.cdc.MustUnmarshal(it.Value(), &healOp)
		if !isHealOpFinalStatus(healOp.Status) {
			continue
		}
		cutoffEpoch := healOp.ScheduledEpochId + keepLastEpochEntries
		if cutoffEpoch >= currentEpochID {
			continue
		}
		toDelete = append(toDelete, healOp)
	}

	for _, healOp := range toDelete {
		store.Delete(types.HealOpKey(healOp.HealOpId))
		store.Delete(types.HealOpByTicketIndexKey(healOp.TicketId, healOp.HealOpId))
		store.Delete(types.HealOpByStatusIndexKey(healOp.Status, healOp.HealOpId))

		// Remove all verification sub-keys for this heal op.
		verPrefix := types.HealOpVerificationPrefix(healOp.HealOpId)
		vit := store.Iterator(verPrefix, storetypes.PrefixEndBytes(verPrefix))
		var verKeys [][]byte
		for ; vit.Valid(); vit.Next() {
			kc := make([]byte, len(vit.Key()))
			copy(kc, vit.Key())
			verKeys = append(verKeys, kc)
		}
		vit.Close()
		for _, vk := range verKeys {
			store.Delete(vk)
		}
	}
	return nil
}
