package keeper

import (
	"context"
	"testing"
	"time"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	"cosmossdk.io/core/address"
	storetypes "cosmossdk.io/store/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LumeraProtocol/lumera/app"
	"github.com/LumeraProtocol/lumera/x/claim/keeper"
	"github.com/LumeraProtocol/lumera/x/claim/types"
)

type TestSuite struct {
	suite.Suite
	app *app.App
}

func (s *TestSuite) SetupTest() {
	s.app = app.Setup(s.T())

}

type MockBankKeeper struct {
	mintedCoins    sdk.Coins
	burnedCoins    sdk.Coins
	sentCoins      map[string]sdk.Coins
	moduleBalances map[string]sdk.Coins
}

func NewMockBankKeeper() *MockBankKeeper {
	return &MockBankKeeper{
		sentCoins:      make(map[string]sdk.Coins),
		moduleBalances: make(map[string]sdk.Coins),
	}
}

func (k *MockBankKeeper) MintCoins(ctx context.Context, moduleName string, amt sdk.Coins) error {
	k.mintedCoins = amt
	if k.moduleBalances[moduleName].IsZero() {
		k.moduleBalances[moduleName] = amt
	} else {
		k.moduleBalances[moduleName] = k.moduleBalances[moduleName].Add(amt...)
	}
	return nil
}

func (k *MockBankKeeper) BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error {
	k.burnedCoins = amt
	if !k.moduleBalances[moduleName].IsZero() {
		k.moduleBalances[moduleName] = k.moduleBalances[moduleName].Sub(amt...)
	}
	return nil
}

func (k *MockBankKeeper) SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	k.sentCoins[recipientAddr.String()] = amt
	if !k.moduleBalances[senderModule].IsZero() {
		k.moduleBalances[senderModule] = k.moduleBalances[senderModule].Sub(amt...)
	}
	return nil
}

type MockAccountKeeper struct {
	accounts       map[string]sdk.AccountI
	moduleAccounts map[string]sdk.ModuleAccountI
	addrCodec      address.Codec
}

func NewMockAccountKeeper() *MockAccountKeeper {
	return &MockAccountKeeper{
		accounts:       make(map[string]sdk.AccountI),
		moduleAccounts: make(map[string]sdk.ModuleAccountI),
		addrCodec:      addresscodec.NewBech32Codec("lumera"),
	}
}

func (k *MockAccountKeeper) AddressCodec() address.Codec {
	return k.addrCodec
}

func (k *MockAccountKeeper) GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI {
	return k.accounts[addr.String()]
}

func (k *MockAccountKeeper) SetAccount(ctx context.Context, acc sdk.AccountI) {
	k.accounts[acc.GetAddress().String()] = acc
}

func (k *MockAccountKeeper) NewAccount(ctx context.Context, acc sdk.AccountI) sdk.AccountI {
	return acc
}

func (k *MockAccountKeeper) GetModuleAccount(ctx context.Context, moduleName string) sdk.ModuleAccountI {
	return k.moduleAccounts[moduleName]
}

func (k *MockAccountKeeper) SetModuleAccount(ctx context.Context, macc sdk.ModuleAccountI) {
	k.moduleAccounts[macc.GetName()] = macc
}

func (k *MockBankKeeper) GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	// Empty mock implementation
	return sdk.NewInt64Coin(denom, 0)
}

func (k *MockAccountKeeper) NewAccountWithAddress(ctx context.Context, addr sdk.AccAddress) sdk.AccountI {
	return authtypes.NewBaseAccountWithAddress(addr)
}

func (k *MockBankKeeper) SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
	return nil
}

func ClaimKeeper(t testing.TB, claimsPath string) (keeper.Keeper, sdk.Context) {
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	tstoreKey := storetypes.NewTransientStoreKey("t_" + types.StoreKey)

	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	// Add this line to mount the transient store
	stateStore.MountStoreWithDB(tstoreKey, storetypes.StoreTypeTransient, nil)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)

	authority := authtypes.NewModuleAddress(govtypes.ModuleName)

	// Create module account
	moduleAcc := authtypes.NewEmptyModuleAccount(types.ModuleName, authtypes.Minter, authtypes.Burner)
	ak := NewMockAccountKeeper()
	ak.SetModuleAccount(context.Background(), moduleAcc)

	bk := NewMockBankKeeper()

	k := keeper.NewKeeper(
		cdc,
		runtime.NewKVStoreService(storeKey),
		runtime.NewTransientStoreService(tstoreKey),
		log.NewNopLogger(),
		authority.String(),
		bk,
		ak,
		claimsPath, // Path to the claims CSV file for testing
	)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{
		Height: 1,
		Time:   time.Now().UTC(),
	}, false, log.NewNopLogger())

	// Initialize params
	if err := k.SetParams(ctx, types.DefaultParams()); err != nil {
		panic(err)
	}

	return k, ctx
}

// Helper function to create a claim record for testing
func CreateClaimRecord(ctx sdk.Context, k keeper.Keeper, oldAddress string, balance sdk.Coins, claimed bool) error {
	record := types.ClaimRecord{
		OldAddress: oldAddress,
		Balance:    balance,
		Claimed:    claimed,
	}
	return k.SetClaimRecord(ctx, record)
}
