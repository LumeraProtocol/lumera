package app_test

import (
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
	"cosmossdk.io/store/rootmulti"
	storetypes "cosmossdk.io/store/types"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	vmmocks "github.com/cosmos/evm/x/vm/types/mocks"

	"github.com/cosmos/evm/x/vm/statedb"
)

// newStateDBWithStore creates a StateDB backed by an in-memory multi-store so
// that Snapshot / AddPrecompileFn / RevertToSnapshot work through the public
// API (they need CacheContext which requires a real MultiStore).
func newStateDBWithStore(t *testing.T) (*statedb.StateDB, sdk.Context) {
	t.Helper()

	db := dbm.NewMemDB()
	ms := rootmulti.NewStore(db, log.NewNopLogger(), nil)

	// Mount at least one KV store so CacheContext succeeds.
	key := storetypes.NewKVStoreKey("test")
	ms.MountStoreWithDB(key, storetypes.StoreTypeIAVL, nil)
	require.NoError(t, ms.LoadLatestVersion())

	ctx := sdk.NewContext(ms, cmtproto.Header{}, false, log.NewNopLogger()).WithEventManager(sdk.NewEventManager())

	keeper := vmmocks.NewEVMKeeper()
	keeper.KVStoreKeys()[key.Name()] = key

	sdb := statedb.New(ctx, keeper, statedb.NewEmptyTxConfig())

	// Initialize the cache context (triggers cache() internally) so that
	// FlushToCacheCtx / AddPrecompileFn / MultiStoreSnapshot work.
	_, err := sdb.GetCacheContext()
	require.NoError(t, err)

	return sdb, ctx
}

// TestRevertToSnapshot_ProcessedEventsInvariant is adapted from cosmos/evm
// v0.6.0 x/vm/statedb/balance_events_test.go. It verifies that after a
// snapshot revert, the processedEventsCount tracked internally by StateDB
// is correctly rolled back so that it never exceeds the current event count.
//
// The upstream test accesses unexported StateDB fields directly (it lives in
// the statedb package). This adaptation exercises the same code path through
// the public API: Snapshot → AddPrecompileFn → FlushToCacheCtx → Revert.
func TestRevertToSnapshot_ProcessedEventsInvariant(t *testing.T) {
	testCases := []struct {
		name           string
		numPrecompiles int
		revertToIndex  int
		expectedEvents int
	}{
		{"revert to 5 precompile calls", 10, 5, 5},
		{"revert to 2 precompile calls", 10, 2, 2},
		{"revert to 0 precompile calls", 10, 0, 0},
		{"revert to 8 precompile calls", 10, 8, 8},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sdb, _ := newStateDBWithStore(t)

			// Snapshot 0: before any precompile calls.
			snapshots := []int{sdb.Snapshot()}

			for i := 0; i < tc.numPrecompiles; i++ {
				// FlushToCacheCtx commits pending journal entries to the
				// cache context and updates processedEventsCount internally.
				require.NoError(t, sdb.FlushToCacheCtx())

				// MultiStoreSnapshot creates a store-level snapshot for the
				// precompile journal entry (mirrors EVM precompile dispatch).
				msSnap := sdb.MultiStoreSnapshot()
				require.NoError(t, sdb.AddPrecompileFn(msSnap))

				// Emit an event in the cache context (simulates a precompile
				// emitting a Cosmos event during execution).
				cacheCtx, err := sdb.GetCacheContext()
				require.NoError(t, err)
				cacheCtx.EventManager().EmitEvent(
					sdk.NewEvent("precompile_test", sdk.NewAttribute("idx", string(rune('0'+i)))),
				)

				// FlushToCacheCtx again so processedEventsCount picks up the
				// event we just emitted.
				require.NoError(t, sdb.FlushToCacheCtx())

				// Snapshot after each precompile call.
				snapshots = append(snapshots, sdb.Snapshot())
			}

			// Revert to the target snapshot.
			sdb.RevertToSnapshot(snapshots[tc.revertToIndex])

			// After revert, the cache context event manager should contain
			// only the events up to the reverted snapshot.
			cacheCtx, err := sdb.GetCacheContext()
			require.NoError(t, err)
			currentEvents := len(cacheCtx.EventManager().Events())
			require.Equal(t, tc.expectedEvents, currentEvents,
				"event count mismatch after revert to snapshot %d", tc.revertToIndex)
		})
	}
}
