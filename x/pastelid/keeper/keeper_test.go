package keeper_test

import (
	"os"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/golang/mock/gomock"
	"github.com/pastelnetwork/pastel/x/pastelid/keeper"
	pastelidmocks "github.com/pastelnetwork/pastel/x/pastelid/mocks"
	"github.com/pastelnetwork/pastel/x/pastelid/types"
	"github.com/stretchr/testify/require"
)

func TestKeeper_GetAuthority(t *testing.T) {
	testCases := []struct {
		name        string
		authority   string
		expectPanic bool
	}{
		{
			name:        "Valid authority address",
			authority:   authtypes.NewModuleAddress(govtypes.ModuleName).String(),
			expectPanic: false,
		},
		{
			name:        "Invalid authority address",
			authority:   "invalid_address",
			expectPanic: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectPanic {
				require.Panics(t, func() {
					_, _ = setupKeeperForTest(t, tc.authority, log.NewNopLogger())
				}, "Expected panic with invalid authority address")
			} else {
				k, _ := setupKeeperForTest(t, tc.authority, log.NewNopLogger())
				result := k.GetAuthority()
				require.Equal(t, tc.authority, result, "GetAuthority should return the correct authority address")
			}
		})
	}
}

func TestKeeper_Logger(t *testing.T) {
	testCases := []struct {
		name      string
		logger    log.Logger
		authority string
	}{
		{
			name:      "Using NopLogger",
			logger:    log.NewNopLogger(),
			authority: authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		},
		{
			name:      "Using standard Logger",
			logger:    log.NewLogger(os.Stdout),
			authority: authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			k, _ := setupKeeperForTest(t, tc.authority, tc.logger)
			logger := k.Logger()
			require.NotNil(t, logger, "Logger should not be nil")
		})
	}
}

func setupKeeperForTest(t testing.TB, authority string, logger log.Logger) (
	keeper.Keeper, sdk.Context) {
	ctl := gomock.NewController(t)
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)

	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	bankKeeper := pastelidmocks.NewMockBankKeeper(ctl)
	accountKeeper := pastelidmocks.NewMockAccountKeeper(ctl)

	k := keeper.NewKeeper(
		cdc,
		runtime.NewKVStoreService(storeKey),
		logger,
		authority,
		bankKeeper,
		accountKeeper,
	)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{}, false, logger)

	if err := k.SetParams(ctx, types.DefaultParams()); err != nil {
		panic(err)
	}

	return k, ctx
}
