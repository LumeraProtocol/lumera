package keeper

// Tests for storage-truth secondary-index and primary-store pruning added in
// response to roomote 122 review (long-term unbounded growth of st/spt/,
// st/rrs-tt/, st/spt-tbe/).

import (
	"encoding/binary"
	"encoding/json"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"

	dbm "github.com/cosmos/cosmos-db"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

// newTestKVStore returns an in-memory KVStore suitable for prune helper tests.
func newTestKVStore(t *testing.T) storetypes.KVStore {
	t.Helper()
	db := dbm.NewMemDB()
	storeKey := storetypes.NewKVStoreKey("prune_test")
	rms := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	rms.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, rms.LoadLatestVersion())
	return rms.GetKVStore(storeKey)
}

// Test the rrs-tt secondary index follows the same shape as st/rrs/ and is
// pruned by pruneSupernodeWindowReporter (account-then-epoch).
func TestPruneSupernodeWindowReporter_RrsTt(t *testing.T) {
	kv := newTestKVStore(t)
	prefix := []byte("st/rrs-tt/")

	// Insert 3 keys at epochs 1, 5, 10. Format:
	// "st/rrs-tt/" + target + "/" + u64be(epoch) + "/" + ticket + 0x00 + reporter
	target := "tgt1"
	for _, epoch := range []uint64{1, 5, 10} {
		key := append([]byte{}, prefix...)
		key = append(key, target...)
		key = append(key, '/')
		key = binary.BigEndian.AppendUint64(key, epoch)
		key = append(key, '/')
		key = append(key, "ticketABC"...)
		key = append(key, 0)
		key = append(key, "rep1"...)
		kv.Set(key, []byte("v"))
	}

	pruneSupernodeWindowReporter(kv, prefix, 5) // keep epochs >= 5

	it := kv.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer it.Close()
	var kept []uint64
	for ; it.Valid(); it.Next() {
		key := it.Key()
		rest := key[len(prefix):]
		sep := bytesIndexByte(rest, '/')
		require.Greater(t, sep, 0)
		epochID := binary.BigEndian.Uint64(rest[sep+1 : sep+1+8])
		kept = append(kept, epochID)
	}
	require.ElementsMatch(t, []uint64{5, 10}, kept)
}

// Test pruneTargetBucketEpoch on st/spt-tbe shape:
// "st/spt-tbe/" + target + "/" + u32be(bucket) + "/" + u64be(epoch) + "/" + hash
func TestPruneTargetBucketEpoch(t *testing.T) {
	kv := newTestKVStore(t)
	prefix := []byte("st/spt-tbe/")

	target := "tgt1"
	for _, epoch := range []uint64{1, 5, 10} {
		for _, bucket := range []uint32{1, 2} {
			key := append([]byte{}, prefix...)
			key = append(key, target...)
			key = append(key, '/')
			key = binary.BigEndian.AppendUint32(key, bucket)
			key = append(key, '/')
			key = binary.BigEndian.AppendUint64(key, epoch)
			key = append(key, '/')
			key = append(key, "hashXYZ"...)
			kv.Set(key, []byte("v"))
		}
	}

	pruneTargetBucketEpoch(kv, prefix, 5) // keep epochs >= 5

	it := kv.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer it.Close()
	var keptEpochs []uint64
	for ; it.Valid(); it.Next() {
		key := it.Key()
		rest := key[len(prefix):]
		sep := bytesIndexByte(rest, '/')
		require.Greater(t, sep, 0)
		// after first '/': 4-byte bucket + '/' + 8-byte epoch
		epochStart := sep + 1 + 4 + 1
		keptEpochs = append(keptEpochs, binary.BigEndian.Uint64(rest[epochStart:epochStart+8]))
	}
	// 4 entries kept: epochs 5,5,10,10
	require.ElementsMatch(t, []uint64{5, 5, 10, 10}, keptEpochs)
}

// Test pruneStorageProofTranscripts decodes embedded epoch_id and prunes by JSON.
func TestPruneStorageProofTranscripts(t *testing.T) {
	kv := newTestKVStore(t)
	prefix := []byte("st/spt/")

	put := func(hash string, epoch uint64) {
		key := append([]byte{}, prefix...)
		key = append(key, hash...)
		v, _ := json.Marshal(struct {
			EpochID uint64 `json:"epoch_id"`
			Other   string `json:"other"`
		}{EpochID: epoch, Other: "x"})
		kv.Set(key, v)
	}
	put("h1", 1)
	put("h5", 5)
	put("h10", 10)
	// Malformed record — must be preserved (no data loss on parse error).
	kv.Set(append([]byte{}, append(prefix, "hbad"...)...), []byte("not-json"))

	pruneStorageProofTranscripts(sdk.Context{}, Keeper{logger: log.NewNopLogger()}, kv, prefix, 5)

	it := kv.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer it.Close()
	var keys []string
	for ; it.Valid(); it.Next() {
		keys = append(keys, string(it.Key()[len(prefix):]))
	}
	require.ElementsMatch(t, []string{"h5", "h10", "hbad"}, keys)
}
