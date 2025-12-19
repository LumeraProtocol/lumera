package keeper

import (
	"bytes"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/stretchr/testify/require"
)

func setupKeeperForInternalTest(t testing.TB) (Keeper, sdk.Context) {
	t.Helper()

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	authority := authtypes.NewModuleAddress(govtypes.ModuleName).String()

	k := NewKeeper(
		cdc,
		runtime.NewKVStoreService(storeKey),
		log.NewNopLogger(),
		authority,
		nil,
		nil,
		nil,
	)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{}, false, log.NewNopLogger())

	return k, ctx
}

func TestKeeper_GetSuperNodeByAccount(t *testing.T) {
	const (
		accountPrefix       = "lumera"
		validatorOperPrefix = "lumeravaloper"
	)

	val1 := sdk.ValAddress(bytes.Repeat([]byte{0x01}, 20))
	val2 := sdk.ValAddress(bytes.Repeat([]byte{0x02}, 20))
	accABytes := sdk.AccAddress(bytes.Repeat([]byte{0x0a}, 20))
	accBBytes := sdk.AccAddress(bytes.Repeat([]byte{0x0b}, 20))

	val1Bech32, err := sdk.Bech32ifyAddressBytes(validatorOperPrefix, val1)
	require.NoError(t, err)
	val2Bech32, err := sdk.Bech32ifyAddressBytes(validatorOperPrefix, val2)
	require.NoError(t, err)
	accABech32, err := sdk.Bech32ifyAddressBytes(accountPrefix, accABytes)
	require.NoError(t, err)
	accBBech32, err := sdk.Bech32ifyAddressBytes(accountPrefix, accBBytes)
	require.NoError(t, err)

	baseSN := func(valBech32, accBech32 string) types.SuperNode {
		return types.SuperNode{
			ValidatorAddress: valBech32,
			SupernodeAccount: accBech32,
			PrevIpAddresses: []*types.IPAddressHistory{
				{Address: "1.2.3.4", Height: 1},
			},
			States: []*types.SuperNodeStateRecord{
				{State: types.SuperNodeStateActive, Height: 1},
			},
		}
	}

	t.Run("success", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)
		sn := baseSN(val1Bech32, accABech32)

		require.NoError(t, k.SetSuperNode(ctx, sn))

		got, found, err := k.GetSuperNodeByAccount(ctx, accABech32)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, sn.ValidatorAddress, got.ValidatorAddress)
		require.Equal(t, sn.SupernodeAccount, got.SupernodeAccount)
	})

	t.Run("missing", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)

		_, found, err := k.GetSuperNodeByAccount(ctx, accABech32)
		require.NoError(t, err)
		require.False(t, found)
	})

	t.Run("stale index points to missing primary", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)

		storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
		indexStore := prefix.NewStore(storeAdapter, types.SuperNodeByAccountKey)
		indexStore.Set([]byte(accABech32), val1)

		_, found, err := k.GetSuperNodeByAccount(ctx, accABech32)
		require.NoError(t, err)
		require.False(t, found)
	})

	t.Run("stale index points to wrong account", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)
		sn := baseSN(val1Bech32, accBBech32)
		require.NoError(t, k.SetSuperNode(ctx, sn))

		storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
		indexStore := prefix.NewStore(storeAdapter, types.SuperNodeByAccountKey)
		indexStore.Set([]byte(accABech32), val1)

		_, found, err := k.GetSuperNodeByAccount(ctx, accABech32)
		require.NoError(t, err)
		require.False(t, found)
	})

	t.Run("account change updates index", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)
		sn := baseSN(val1Bech32, accABech32)
		require.NoError(t, k.SetSuperNode(ctx, sn))

		sn.SupernodeAccount = accBBech32
		require.NoError(t, k.SetSuperNode(ctx, sn))

		_, foundA, err := k.GetSuperNodeByAccount(ctx, accABech32)
		require.NoError(t, err)
		require.False(t, foundA)

		gotB, foundB, err := k.GetSuperNodeByAccount(ctx, accBBech32)
		require.NoError(t, err)
		require.True(t, foundB)
		require.Equal(t, val1Bech32, gotB.ValidatorAddress)
		require.Equal(t, accBBech32, gotB.SupernodeAccount)

		storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
		indexStore := prefix.NewStore(storeAdapter, types.SuperNodeByAccountKey)
		require.Nil(t, indexStore.Get([]byte(accABech32)))
	})

	t.Run("account collision rejected", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)
		require.NoError(t, k.SetSuperNode(ctx, baseSN(val1Bech32, accABech32)))

		err := k.SetSuperNode(ctx, baseSN(val2Bech32, accABech32))
		require.Error(t, err)
		require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
	})
}
