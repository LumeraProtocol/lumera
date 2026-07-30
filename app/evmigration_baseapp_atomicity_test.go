package app_test

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"testing"

	storetypes "cosmossdk.io/store/types"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	lumeraapp "github.com/LumeraProtocol/lumera/app"
	actionkeeper "github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	evmigrationtypes "github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// committedStoreSnapshot reads every committed KV pair across every mounted
// KV store directly from the CommitMultiStore, so it observes exactly what a
// restarting node or a replaying peer would observe. Transient and memory
// stores are excluded because they are not part of committed state.
func committedStoreSnapshot(t *testing.T, app *lumeraapp.App) map[string][]byte {
	t.Helper()

	snapshot := make(map[string][]byte)
	cms := app.CommitMultiStore()
	for _, storeKey := range app.GetStoreKeys() {
		kvKey, ok := storeKey.(*storetypes.KVStoreKey)
		if !ok {
			continue
		}
		kv := cms.GetKVStore(kvKey)
		it := kv.Iterator(nil, nil)
		for ; it.Valid(); it.Next() {
			snapshot[kvKey.Name()+"|"+string(bytes.Clone(it.Key()))] = bytes.Clone(it.Value())
		}
		require.NoError(t, it.Error())
		require.NoError(t, it.Close())
	}
	return snapshot
}

// storeDelta returns the sorted set of snapshot keys that were added, removed,
// or changed between two committed snapshots.
func storeDelta(before, after map[string][]byte) []string {
	var delta []string
	for key, afterValue := range after {
		beforeValue, existed := before[key]
		if !existed || !bytes.Equal(beforeValue, afterValue) {
			delta = append(delta, key)
		}
	}
	for key := range before {
		if _, ok := after[key]; !ok {
			delta = append(delta, key)
		}
	}
	sort.Strings(delta)
	return delta
}

func snapshotStoreNames(delta []string) []string {
	seen := map[string]struct{}{}
	var names []string
	for _, key := range delta {
		name := key[:strings.Index(key, "|")]
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// enableMigrationInCommittedState commits EnableMigration=true directly into
// the committed multistore so the FinalizeBlock/CheckTx paths below run against
// realistic committed params without needing a governance tx.
func enableMigrationInCommittedState(t *testing.T, app *lumeraapp.App) {
	t.Helper()

	ctx := app.NewUncachedContext(false, cmtproto.Header{Height: app.LastBlockHeight()})
	params := evmigrationtypes.NewParams(true, 0, 50, 2000, 20)
	require.NoError(t, params.Validate())
	require.NoError(t, app.EvmigrationKeeper.Params.Set(ctx, params))
	app.CommitMultiStore().Commit()
}

// seedDanglingActionCreatorIndex writes a creator secondary-index row pointing
// at an action ID that has no canonical primary row.
//
// MigrateActions is the LAST step of migrateAccount (step 7 of 7) and resolves
// index rows through the primary store, so this makes the migration fail only
// AFTER distribution, staking, auth, bank, retained SDK state, feegrant, audit
// and supernode writes have already been performed inside the tx cache. That is
// precisely the late-failure shape BaseApp rollback has to contain.
func seedDanglingActionCreatorIndex(t *testing.T, app *lumeraapp.App, creator sdk.AccAddress) {
	t.Helper()

	ctx := app.NewUncachedContext(false, cmtproto.Header{Height: app.LastBlockHeight()})
	store := ctx.KVStore(app.GetKey("action"))
	key := []byte(actionkeeper.ActionByCreatorPrefix + creator.String() + "/" + "dangling-action-id")
	store.Set(key, []byte("dangling-action-id"))
	app.CommitMultiStore().Commit()
}

// TestEVMigration_BaseAppLateFailureRollsBackEveryStore proves invariant I20
// (atomicity) through the production BaseApp transaction path rather than a
// direct keeper call: a migration that fails in its LAST execution step must
// commit no migration state at all.
//
// Without BaseApp's tx cache — or if any migration step wrote outside it — a
// late failure would leave the account half-migrated: balances moved but no
// migration record, or a re-keyed SuperNode with a stale audit identity. Both
// are unrecoverable without a coordinated upgrade.
//
// The assertion is a differential against an empty control block on the same
// app, NOT a claim that the whole app store is unchanged. BeginBlock/EndBlock
// legitimately commit block-level state (mint, fee distribution, staking
// historical info, wasm sequences) on every block, with or without our tx. The
// migration tx is fee-free and zero-signer, so it has no legitimate ante fee or
// sequence side effect either — meaning the failing-tx block must produce a
// delta whose *store set* is a subset of the empty control block's, and must
// touch no identity-bearing row.
func TestEVMigration_BaseAppLateFailureRollsBackEveryStore(t *testing.T) {
	legacyPriv := secp256k1.GenPrivKey()
	app := setupAppWithLegacyAccountForMempool(t, legacyPriv)
	enableMigrationInCommittedState(t, app)

	legacyAddr := sdk.AccAddress(legacyPriv.PubKey().Address().Bytes())
	seedDanglingActionCreatorIndex(t, app, legacyAddr)

	msg := validMigrationMsgForMempoolWithLegacy(t, testChainID, legacyPriv)
	newAddr, err := sdk.AccAddressFromBech32(msg.NewAddress)
	require.NoError(t, err)

	tx := newUnsignedMigrationTxForMempool(t, app, msg)
	txBytes, err := app.TxConfig().TxEncoder()(tx)
	require.NoError(t, err)

	// --- Control: one empty block establishes the pure block-level delta. ---
	controlBefore := committedStoreSnapshot(t, app)
	_, err = app.FinalizeBlock(&abci.RequestFinalizeBlock{Height: app.LastBlockHeight() + 1})
	require.NoError(t, err)
	_, err = app.Commit()
	require.NoError(t, err)
	controlDelta := storeDelta(controlBefore, committedStoreSnapshot(t, app))
	controlStores := snapshotStoreNames(controlDelta)

	// --- Subject: identical block, plus the failing migration tx. ---
	before := committedStoreSnapshot(t, app)
	resp, err := app.FinalizeBlock(&abci.RequestFinalizeBlock{
		Height: app.LastBlockHeight() + 1,
		Txs:    [][]byte{txBytes},
	})
	require.NoError(t, err)
	require.Len(t, resp.TxResults, 1)

	// The tx must fail, and it must fail for the seeded late-step reason — not
	// because an earlier precondition rejected it, which would silently stop
	// this test from exercising rollback at all.
	require.NotEqual(t, uint32(0), resp.TxResults[0].Code,
		"migration with a dangling action index must fail")
	require.Contains(t, resp.TxResults[0].Log, "migrate actions",
		"failure must originate in the LAST migration step, otherwise rollback is untested")

	_, err = app.Commit()
	require.NoError(t, err)

	after := committedStoreSnapshot(t, app)
	delta := storeDelta(before, after)

	// 1. Qualify the one legitimate ante-level side effect before comparing
	//    store sets. wasmd's CountTXDecorator increments a per-block tx counter
	//    at wasm key 0x08 in the ante, which runs (and persists) for any tx
	//    admitted to a block regardless of whether message execution later
	//    fails. It carries a height and a count only — no identity, no value —
	//    so it is an expected ante effect, not migration state. Asserting it
	//    explicitly is required by the spec instead of claiming the whole app
	//    store is unchanged.
	wasmTxCounterKey := wasmtypes.StoreKey + "|" + string(wasmtypes.TXCounterPrefix)
	var unqualified []string
	for _, key := range delta {
		if key == wasmTxCounterKey {
			require.NotContains(t, before, key,
				"wasm tx counter should be absent before the first tx-bearing block")
			require.Len(t, after[key], 12,
				"wasm tx counter must stay an 8-byte height + 4-byte count, carrying no identity")
			continue
		}
		unqualified = append(unqualified, key)
	}

	// 2. Beyond that, the failing tx must not widen the set of stores a block
	//    touches, measured against an empty control block on the same app.
	require.Subsetf(t, controlStores, snapshotStoreNames(unqualified),
		"failed migration tx touched stores an empty block does not: control=%v subject=%v",
		controlStores, snapshotStoreNames(unqualified))

	// 3. No migration-owned or identity-bearing store may appear in the delta.
	for _, storeName := range []string{
		evmigrationtypes.StoreKey, "supernode", "audit", "action", "authz", "feegrant", "gov",
	} {
		for _, key := range delta {
			require.Falsef(t, strings.HasPrefix(key, storeName+"|"),
				"rolled-back migration must not commit to the %q store (key %q)", storeName, key)
		}
	}

	// 4. No committed key or value anywhere may reference either identity.
	for _, identity := range []string{legacyAddr.String(), newAddr.String()} {
		for _, key := range delta {
			require.NotContainsf(t, key, identity,
				"rolled-back migration leaked identity %s into committed key %q", identity, key)
			require.NotContainsf(t, string(after[key]), identity,
				"rolled-back migration leaked identity %s into committed value at %q", identity, key)
		}
	}

	// 5. Positive control on the observable module contract.
	ctx := app.NewUncachedContext(false, cmtproto.Header{Height: app.LastBlockHeight()})
	has, err := app.EvmigrationKeeper.MigrationRecords.Has(ctx, legacyAddr.String())
	require.NoError(t, err)
	require.False(t, has, "rolled-back migration must not leave a migration record")

	has, err = app.EvmigrationKeeper.MigrationRecordByNewAddress.Has(ctx, newAddr.String())
	require.NoError(t, err)
	require.False(t, has, "rolled-back migration must not leave a destination index row")
}

// TestEVMigration_CheckTxReCheckTxSimulateAreStateNeutral proves spec §5: the
// mempool and simulation phases must be observationally pure with respect to
// committed state.
//
// This matters beyond tidiness. Migration txs are fee-free and zero-signer, so
// if CheckTx or Simulate could mutate committed state, an unauthenticated
// sender could drive state transitions for free without ever landing a tx in a
// block — and validators running different mempool loads would diverge.
func TestEVMigration_CheckTxReCheckTxSimulateAreStateNeutral(t *testing.T) {
	legacyPriv := secp256k1.GenPrivKey()
	app := setupAppWithLegacyAccountForMempool(t, legacyPriv)
	enableMigrationInCommittedState(t, app)

	msg := validMigrationMsgForMempoolWithLegacy(t, testChainID, legacyPriv)
	tx := newUnsignedMigrationTxForMempool(t, app, msg)
	txBytes, err := app.TxConfig().TxEncoder()(tx)
	require.NoError(t, err)

	before := committedStoreSnapshot(t, app)

	phases := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "CheckTx",
			run: func(t *testing.T) {
				resp, err := app.CheckTx(&abci.RequestCheckTx{Tx: txBytes, Type: abci.CheckTxType_New})
				require.NoError(t, err)
				require.NotNil(t, resp)
			},
		},
		{
			name: "ReCheckTx",
			run: func(t *testing.T) {
				resp, err := app.CheckTx(&abci.RequestCheckTx{Tx: txBytes, Type: abci.CheckTxType_Recheck})
				require.NoError(t, err)
				require.NotNil(t, resp)
			},
		},
		{
			name: "Simulate",
			run: func(t *testing.T) {
				gasInfo, _, err := app.Simulate(txBytes)
				// Simulate may legitimately return an execution error; what it
				// must never do is commit. Gas accounting still has to happen.
				if err == nil {
					require.NotZero(t, gasInfo.GasUsed, "simulation must account gas")
				}
			},
		},
	}

	for _, phase := range phases {
		t.Run(phase.name, func(t *testing.T) {
			phase.run(t)
			require.Emptyf(t, storeDelta(before, committedStoreSnapshot(t, app)),
				fmt.Sprintf("%s must not mutate committed state", phase.name))
		})
	}

	// Re-assert after all three ran back to back, so an effect that only appears
	// once the check state is reused across phases cannot hide.
	require.Empty(t, storeDelta(before, committedStoreSnapshot(t, app)),
		"CheckTx/ReCheckTx/Simulate in sequence must remain state-neutral")
}
