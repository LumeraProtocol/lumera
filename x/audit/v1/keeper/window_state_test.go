package keeper

import (
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/address"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	supernodemocks "github.com/LumeraProtocol/lumera/x/supernode/v1/mocks"
)

func newAuditKeeperForWindowTests(t testing.TB) (Keeper, sdk.Context) {
	t.Helper()

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	storeService := runtime.NewKVStoreService(storeKey)

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	supernodeKeeper := supernodemocks.NewMockSupernodeKeeper(ctrl)
	authority := authtypes.NewModuleAddress(govtypes.ModuleName)
	addrCodec := address.NewBech32Codec("lumera")

	k := NewKeeper(
		cdc,
		addrCodec,
		storeService,
		log.NewNopLogger(),
		authority,
		supernodeKeeper,
	)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{}, false, log.NewNopLogger())
	return k, ctx
}

func TestWindowState_AdvancesAndAppliesPendingWindowBlocks(t *testing.T) {
	k, ctx := newAuditKeeperForWindowTests(t)

	params := types.DefaultParams()
	params.ReportingWindowBlocks = 10

	ctx = ctx.WithBlockHeight(100)
	ws, err := k.getCurrentWindowState(ctx, params)
	require.NoError(t, err)
	require.Equal(t, uint64(0), ws.WindowID)
	require.Equal(t, int64(100), ws.StartHeight)
	require.Equal(t, int64(109), ws.EndHeight)
	require.Equal(t, uint64(10), ws.WindowBlocks)

	// Schedule a change; it should not affect the current window.
	ctx = ctx.WithBlockHeight(105)
	require.NoError(t, k.scheduleReportingWindowBlocksChangeAtNextBoundary(ctx, params, 7))
	ws, err = k.getCurrentWindowState(ctx, params)
	require.NoError(t, err)
	require.Equal(t, uint64(0), ws.WindowID)
	require.Equal(t, int64(100), ws.StartHeight)
	require.Equal(t, int64(109), ws.EndHeight)

	// Crossing the boundary advances the window and applies the pending size.
	ctx = ctx.WithBlockHeight(110)
	ws, err = k.getCurrentWindowState(ctx, params)
	require.NoError(t, err)
	require.Equal(t, uint64(1), ws.WindowID)
	require.Equal(t, int64(110), ws.StartHeight)
	require.Equal(t, int64(116), ws.EndHeight)
	require.Equal(t, uint64(7), ws.WindowBlocks)

	// Next boundary uses the current window size (no pending).
	ctx = ctx.WithBlockHeight(117)
	ws, err = k.getCurrentWindowState(ctx, params)
	require.NoError(t, err)
	require.Equal(t, uint64(2), ws.WindowID)
	require.Equal(t, int64(117), ws.StartHeight)
	require.Equal(t, int64(123), ws.EndHeight)
	require.Equal(t, uint64(7), ws.WindowBlocks)
}

func TestWindowState_PendingOverwrite(t *testing.T) {
	k, ctx := newAuditKeeperForWindowTests(t)

	params := types.DefaultParams()
	params.ReportingWindowBlocks = 10

	ctx = ctx.WithBlockHeight(50)
	_, err := k.getCurrentWindowState(ctx, params)
	require.NoError(t, err)

	ctx = ctx.WithBlockHeight(55)
	require.NoError(t, k.scheduleReportingWindowBlocksChangeAtNextBoundary(ctx, params, 9))
	require.NoError(t, k.scheduleReportingWindowBlocksChangeAtNextBoundary(ctx, params, 8))

	ctx = ctx.WithBlockHeight(60)
	ws, err := k.getCurrentWindowState(ctx, params)
	require.NoError(t, err)
	require.Equal(t, uint64(1), ws.WindowID)
	require.Equal(t, uint64(8), ws.WindowBlocks)
	require.Equal(t, int64(60), ws.StartHeight)
	require.Equal(t, int64(67), ws.EndHeight)
}

