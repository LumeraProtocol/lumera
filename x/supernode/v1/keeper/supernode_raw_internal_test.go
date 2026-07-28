package keeper

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"cosmossdk.io/store/prefix"
	db "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

type terminalErrorIterator struct {
	err      error
	closeErr error
}

var _ db.Iterator = (*terminalErrorIterator)(nil)

func (*terminalErrorIterator) Domain() ([]byte, []byte) { return nil, nil }
func (*terminalErrorIterator) Valid() bool              { return false }
func (*terminalErrorIterator) Next()                    { panic("invalid iterator") }
func (*terminalErrorIterator) Key() []byte              { panic("invalid iterator") }
func (*terminalErrorIterator) Value() []byte            { panic("invalid iterator") }
func (it *terminalErrorIterator) Error() error          { return it.err }
func (it *terminalErrorIterator) Close() error          { return it.closeErr }

func rawTestSuperNode(val sdk.ValAddress, account string) types.SuperNode {
	return types.SuperNode{
		ValidatorAddress: val.String(),
		SupernodeAccount: account,
		States: []*types.SuperNodeStateRecord{{
			State:  types.SuperNodeStateActive,
			Height: 1,
		}},
	}
}

func rawSuperNodeStores(k Keeper, ctx sdk.Context) (prefix.Store, prefix.Store) {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.SuperNodeKey), prefix.NewStore(storeAdapter, types.SuperNodeByAccountKey)
}

func marshalRawSuperNode(t *testing.T, k Keeper, sn types.SuperNode) []byte {
	t.Helper()
	bz, err := k.cdc.Marshal(&sn)
	require.NoError(t, err)
	return bz
}

func TestKeeper_StrictGetSuperNodeByAccount(t *testing.T) {
	val1 := sdk.ValAddress(bytes.Repeat([]byte{0x01}, 20))
	val2 := sdk.ValAddress(bytes.Repeat([]byte{0x02}, 20))
	account := sdk.AccAddress(bytes.Repeat([]byte{0x0a}, 20)).String()
	otherAccount := sdk.AccAddress(bytes.Repeat([]byte{0x0b}, 20)).String()

	t.Run("valid index hit", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)
		sn := rawTestSuperNode(val1, account)
		primary, index := rawSuperNodeStores(k, ctx)
		primary.Set(val1, marshalRawSuperNode(t, k, sn))
		index.Set([]byte(account), val1)

		got, found, err := k.StrictGetSuperNodeByAccount(ctx, account)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, sn, got)
	})

	t.Run("alternate account encoding resolves canonical identity", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)
		storedAccount := strings.ToUpper(account)
		sn := rawTestSuperNode(val1, storedAccount)
		primary, index := rawSuperNodeStores(k, ctx)
		primary.Set(val1, marshalRawSuperNode(t, k, sn))
		index.Set([]byte(storedAccount), val1)

		got, found, err := k.StrictGetSuperNodeByAccount(ctx, account)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, sn, got)
	})

	t.Run("duplicate alternate account index encodings fail closed", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)
		primary, index := rawSuperNodeStores(k, ctx)
		primary.Set(val1, marshalRawSuperNode(t, k, rawTestSuperNode(val1, account)))
		index.Set([]byte(account), val1)
		index.Set([]byte(strings.ToUpper(account)), val1)

		_, found, err := k.StrictGetSuperNodeByAccount(ctx, account)
		require.ErrorContains(t, err, "multiple account index entries")
		require.False(t, found)
	})

	t.Run("alternate account index pointing to missing primary fails closed", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)
		primary, index := rawSuperNodeStores(k, ctx)
		primary.Set(val1, marshalRawSuperNode(t, k, rawTestSuperNode(val1, strings.ToUpper(account))))
		index.Set([]byte(strings.ToUpper(account)), val2)

		_, found, err := k.StrictGetSuperNodeByAccount(ctx, account)
		require.ErrorContains(t, err, "does not resolve to a primary record")
		require.False(t, found)
	})

	t.Run("malformed account index key fails closed", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)
		_, index := rawSuperNodeStores(k, ctx)
		index.Set([]byte("not-bech32"), val1)

		_, found, err := k.StrictGetSuperNodeByAccount(ctx, account)
		require.ErrorContains(t, err, "invalid supernode account-index key")
		require.False(t, found)
	})

	t.Run("true absence after complete scan", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)
		primary, index := rawSuperNodeStores(k, ctx)
		primary.Set(val1, marshalRawSuperNode(t, k, rawTestSuperNode(val1, otherAccount)))
		index.Set([]byte(otherAccount), val1)

		_, found, err := k.StrictGetSuperNodeByAccount(ctx, account)
		require.NoError(t, err)
		require.False(t, found)
	})

	t.Run("missing index with matching primary", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)
		primary, _ := rawSuperNodeStores(k, ctx)
		primary.Set(val1, marshalRawSuperNode(t, k, rawTestSuperNode(val1, account)))

		_, found, err := k.StrictGetSuperNodeByAccount(ctx, account)
		require.ErrorContains(t, err, "missing account index")
		require.False(t, found)
	})

	t.Run("duplicate primary claims", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)
		primary, index := rawSuperNodeStores(k, ctx)
		primary.Set(val1, marshalRawSuperNode(t, k, rawTestSuperNode(val1, account)))
		primary.Set(val2, marshalRawSuperNode(t, k, rawTestSuperNode(val2, account)))
		index.Set([]byte(account), val1)

		_, found, err := k.StrictGetSuperNodeByAccount(ctx, account)
		require.ErrorContains(t, err, "multiple primary records")
		require.False(t, found)
	})

	t.Run("stale index points to missing primary", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)
		_, index := rawSuperNodeStores(k, ctx)
		index.Set([]byte(account), val1)

		_, found, err := k.StrictGetSuperNodeByAccount(ctx, account)
		require.ErrorContains(t, err, "does not resolve to a primary record")
		require.False(t, found)
	})

	t.Run("index points to primary owned by another account", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)
		primary, index := rawSuperNodeStores(k, ctx)
		primary.Set(val1, marshalRawSuperNode(t, k, rawTestSuperNode(val1, otherAccount)))
		index.Set([]byte(account), val1)

		_, found, err := k.StrictGetSuperNodeByAccount(ctx, account)
		require.ErrorContains(t, err, "account mismatch")
		require.False(t, found)
	})

	t.Run("malformed primary fails closed", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)
		primary, _ := rawSuperNodeStores(k, ctx)
		primary.Set(val1, []byte{0xff})

		_, found, err := k.StrictGetSuperNodeByAccount(ctx, account)
		require.ErrorContains(t, err, "unmarshal supernode")
		require.False(t, found)
	})

	t.Run("empty primary account fails closed", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)
		primary, _ := rawSuperNodeStores(k, ctx)
		primary.Set(val1, marshalRawSuperNode(t, k, rawTestSuperNode(val1, "")))

		_, found, err := k.StrictGetSuperNodeByAccount(ctx, account)
		require.ErrorContains(t, err, "invalid embedded supernode account")
		require.False(t, found)
	})

	t.Run("invalid primary account fails closed", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)
		primary, _ := rawSuperNodeStores(k, ctx)
		primary.Set(val1, marshalRawSuperNode(t, k, rawTestSuperNode(val1, "not-bech32")))

		_, found, err := k.StrictGetSuperNodeByAccount(ctx, account)
		require.ErrorContains(t, err, "invalid embedded supernode account")
		require.False(t, found)
	})

	t.Run("primary key and embedded validator mismatch", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)
		primary, _ := rawSuperNodeStores(k, ctx)
		primary.Set(val1, marshalRawSuperNode(t, k, rawTestSuperNode(val2, otherAccount)))

		_, found, err := k.StrictGetSuperNodeByAccount(ctx, account)
		require.ErrorContains(t, err, "validator mismatch")
		require.False(t, found)
	})
}

func snapshotSuperNodeStore(t *testing.T, k Keeper, ctx sdk.Context) map[string][]byte {
	t.Helper()
	store := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	iterator := store.Iterator(nil, nil)
	defer func() { require.NoError(t, iterator.Close()) }()

	snapshot := make(map[string][]byte)
	for ; iterator.Valid(); iterator.Next() {
		snapshot[string(bytes.Clone(iterator.Key()))] = bytes.Clone(iterator.Value())
	}
	return snapshot
}

func TestKeeper_StrictGetSuperNodeByAccount_DoesNotMutateState(t *testing.T) {
	k, ctx := setupKeeperForInternalTest(t)
	val := sdk.ValAddress(bytes.Repeat([]byte{0x03}, 20))
	account := sdk.AccAddress(bytes.Repeat([]byte{0x0c}, 20)).String()
	primary, index := rawSuperNodeStores(k, ctx)
	primary.Set(val, marshalRawSuperNode(t, k, rawTestSuperNode(val, account)))
	index.Set([]byte(account), val)
	before := snapshotSuperNodeStore(t, k, ctx)

	_, found, err := k.StrictGetSuperNodeByAccount(ctx, account)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, before, snapshotSuperNodeStore(t, k, ctx))
}

func TestKeeper_ScanStrictSuperNodes_TerminalIteratorErrorFailsClosed(t *testing.T) {
	k, _ := setupKeeperForInternalTest(t)
	wantErr := errors.New("terminal iterator failure")

	_, _, _, _, err := k.scanStrictSuperNodes(
		&terminalErrorIterator{err: wantErr},
		sdk.AccAddress(bytes.Repeat([]byte{0x0d}, 20)),
		nil,
		false,
	)
	require.ErrorIs(t, err, wantErr)
}

func TestKeeper_ScanStrictSuperNodes_DoesNotSwallowLookalikeTerminalError(t *testing.T) {
	k, _ := setupKeeperForInternalTest(t)
	wantErr := errors.New("invalid cacheMergeIterator")

	_, _, _, _, err := k.scanStrictSuperNodes(
		&terminalErrorIterator{err: wantErr},
		sdk.AccAddress(bytes.Repeat([]byte{0x0e}, 20)),
		nil,
		false,
	)
	require.ErrorIs(t, err, wantErr)
}

func TestKeeper_ScanStrictSuperNodes_CloseErrorFailsClosed(t *testing.T) {
	k, _ := setupKeeperForInternalTest(t)
	wantErr := errors.New("close iterator failure")

	_, _, _, _, err := k.scanStrictSuperNodes(
		&terminalErrorIterator{closeErr: wantErr},
		sdk.AccAddress(bytes.Repeat([]byte{0x0f}, 20)),
		nil,
		false,
	)
	require.ErrorIs(t, err, wantErr)
}

func TestScanStrictSuperNodeAccountIndexes_TerminalIteratorErrorFailsClosed(t *testing.T) {
	wantErr := errors.New("account-index terminal failure")

	_, _, err := scanStrictSuperNodeAccountIndexes(
		&terminalErrorIterator{err: wantErr},
		sdk.AccAddress(bytes.Repeat([]byte{0x10}, 20)),
	)
	require.ErrorIs(t, err, wantErr)
}

func TestScanStrictSuperNodeAccountIndexes_CloseErrorFailsClosed(t *testing.T) {
	wantErr := errors.New("account-index close failure")

	_, _, err := scanStrictSuperNodeAccountIndexes(
		&terminalErrorIterator{closeErr: wantErr},
		sdk.AccAddress(bytes.Repeat([]byte{0x11}, 20)),
	)
	require.ErrorIs(t, err, wantErr)
}
