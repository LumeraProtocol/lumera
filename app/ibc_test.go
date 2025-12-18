package app

import (
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	sdkaddress "github.com/cosmos/cosmos-sdk/codec/address"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	icatypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/types"
)

func TestIsInterchainAccount(t *testing.T) {
	// Non-ICA account should be false
	baseAcc := authtypes.NewBaseAccountWithAddress(sdk.MustAccAddressFromBech32(testaccounts.TestAddress1))
	require.False(t, isInterchainAccount(baseAcc))

	// ICA account should be true
	ica := icatypes.NewInterchainAccount(baseAcc, "owner")
	require.True(t, isInterchainAccount(ica))
}

func TestIsInterchainAccountAddr(t *testing.T) {
	// build a minimal auth keeper with two accounts: one regular, one ICA
	ctx, ak := makeTestAccountKeeper(t)

	// add an ICA account
	icaBase := authtypes.NewBaseAccountWithAddress(sdk.MustAccAddressFromBech32(testaccounts.TestAddress2))
	_ = ak.NewAccount(ctx, icaBase)
	ica := icatypes.NewInterchainAccount(icaBase, "owner")
	ak.SetAccount(ctx, ica)
	// add a regular account
	regAcc := authtypes.NewBaseAccountWithAddress(sdk.MustAccAddressFromBech32(testaccounts.TestAddress1))
	_ = ak.NewAccount(ctx, regAcc)
	ak.SetAccount(ctx, regAcc)

	// regular account: false
	require.False(t, isInterchainAccountAddr(ctx, ak, sdk.MustAccAddressFromBech32(testaccounts.TestAddress1)))
	// ICA account: true
	require.True(t, isInterchainAccountAddr(ctx, ak, sdk.MustAccAddressFromBech32(testaccounts.TestAddress2)))
	// unknown address: false
	require.False(t, isInterchainAccountAddr(ctx, ak, sdk.AccAddress("unknown-addr-0000000000")))
}

func makeTestAccountKeeper(t *testing.T) (sdk.Context, authkeeper.AccountKeeper) {
	t.Helper()

	storeKey := storetypes.NewKVStoreKey(authtypes.StoreKey)
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	cms.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, cms.LoadLatestVersion())

	storeService := runtime.NewKVStoreService(storeKey)

	ir := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(ir)
	icatypes.RegisterInterfaces(ir)
	cdc := codec.NewProtoCodec(ir)

	ak := authkeeper.NewAccountKeeper(
		cdc,
		storeService,
		authtypes.ProtoBaseAccount,
		map[string][]string{},
		sdkaddress.NewBech32Codec("lumera"),
		"lumera",
		"",
	)

	ctx := sdk.NewContext(cms, cmtproto.Header{}, false, log.NewNopLogger())
	return ctx, ak
}
