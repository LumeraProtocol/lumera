package keeper

import (
	"context"
	"testing"

	"cosmossdk.io/math"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/action/keeper"
	"github.com/LumeraProtocol/lumera/x/action/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/types"
)

// Mock implementation for SupernodeKeeper
type ActionMockSupernodeKeeper struct {
	mock.Mock
}

// Implement SupernodeKeeper interface
func (m *ActionMockSupernodeKeeper) GetTopSuperNodesForBlock(ctx context.Context, req *sntypes.QueryGetTopSuperNodesForBlockRequest) (*sntypes.QueryGetTopSuperNodesForBlockResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*sntypes.QueryGetTopSuperNodesForBlockResponse), args.Error(1)
}

func (m *ActionMockSupernodeKeeper) IsSuperNodeActive(ctx sdk.Context, valAddr sdk.ValAddress) bool {
	args := m.Called(ctx, valAddr)
	return args.Bool(0)
}

func (m *ActionMockSupernodeKeeper) QuerySuperNode(ctx sdk.Context, valOperAddr sdk.ValAddress) (sntypes.SuperNode, bool) {
	args := m.Called(ctx, valOperAddr)
	return args.Get(0).(sntypes.SuperNode), args.Bool(1)
}

func (m *ActionMockSupernodeKeeper) SetSuperNode(ctx sdk.Context, superNode sntypes.SuperNode) error {
	args := m.Called(ctx, superNode)
	return args.Error(0)
}

// ActionBankKeeper extends the existing MockBankKeeper with the SpendableCoins method
type ActionBankKeeper struct {
	mock.Mock
	sentCoins      map[string]sdk.Coins
	moduleBalances map[string]sdk.Coins
}

func NewActionMockBankKeeper() *ActionBankKeeper {
	return &ActionBankKeeper{
		sentCoins:      make(map[string]sdk.Coins),
		moduleBalances: make(map[string]sdk.Coins),
	}
}

// SpendableCoins is a mock implementation of the BankKeeper interface
func (m *ActionBankKeeper) SpendableCoins(ctx context.Context, addr sdk.AccAddress) sdk.Coins {
	if coins, ok := m.sentCoins[addr.String()]; ok {
		return coins
	}
	return sdk.NewCoins()
}

// SendCoinsFromAccountToModule is a mock implementation of the BankKeeper interface
func (m *ActionBankKeeper) SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
	if m.moduleBalances[recipientModule].IsZero() {
		m.moduleBalances[recipientModule] = amt
	} else {
		m.moduleBalances[recipientModule] = m.moduleBalances[recipientModule].Add(amt...)
	}
	if _, ok := m.sentCoins[senderAddr.String()]; ok {
		m.sentCoins[senderAddr.String()] = m.sentCoins[senderAddr.String()].Sub(amt...)
	}
	return nil
}

// SendCoinsFromModuleToAccount is a mock implementation of the BankKeeper interface
func (m *ActionBankKeeper) SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	if m.moduleBalances[senderModule].IsZero() {
		m.sentCoins[recipientAddr.String()] = amt
	} else {
		m.sentCoins[recipientAddr.String()] = m.sentCoins[recipientAddr.String()].Add(amt...)
	}
	if _, ok := m.moduleBalances[senderModule]; ok {
		m.moduleBalances[senderModule] = m.moduleBalances[senderModule].Sub(amt...)
	}
	return nil
}

func (m *ActionBankKeeper) GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	if coins, ok := m.sentCoins[addr.String()]; ok {
		for _, coin := range coins {
			if coin.Denom == denom {
				return coin
			}
		}
	}
	return sdk.Coin{}
}

type MockDistributionKeeper struct {
	mock.Mock
}

func (m *MockDistributionKeeper) FundCommunityPool(ctx context.Context, amount sdk.Coins, sender sdk.AccAddress) error {
	return nil
}

type MockStakingKeeper struct {
	mock.Mock
}

func (m *MockStakingKeeper) GetValidator(ctx context.Context, addr sdk.ValAddress) (validator stakingtypes.Validator, err error) {
	args := m.Called(ctx, addr)
	return args.Get(0).(stakingtypes.Validator), args.Error(1)
}

func (m *MockStakingKeeper) Validator(ctx context.Context, addr sdk.ValAddress) (stakingtypes.ValidatorI, error) {
	args := m.Called(ctx, addr)
	return args.Get(0).(stakingtypes.ValidatorI), args.Error(1)
}

type AccountPair struct {
	Address sdk.AccAddress
	PubKey  cryptotypes.PubKey
}

func ActionKeeper(t testing.TB) (keeper.Keeper, sdk.Context) {
	return ActionKeeperWithAddress(t, nil)
}

func ActionKeeperWithAddress(t testing.TB, accounts []AccountPair) (keeper.Keeper, sdk.Context) {
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)

	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	authority := authtypes.NewModuleAddress(govtypes.ModuleName)

	// Create mock keepers
	bankKeeper := NewActionMockBankKeeper()

	accountKeeper := NewMockAccountKeeper()

	stakingKeeper := new(MockStakingKeeper)

	supernodeKeeper := new(ActionMockSupernodeKeeper)

	distributionKeeper := new(MockDistributionKeeper)

	// Setup supernode mock for GetTopSuperNodesForBlock (used in validateSupernodeAuthorization)
	supernodeKeeper.On("GetTopSuperNodesForBlock", mock.Anything, mock.Anything).Return(
		&sntypes.QueryGetTopSuperNodesForBlockResponse{
			Supernodes: []*sntypes.SuperNode{
				{ValidatorAddress: "cosmosvaloper1example"}, // Example supernode for tests
			},
		}, nil)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{}, false, log.NewNopLogger())
	if accounts != nil && len(accounts) > 0 {
		for _, acc := range accounts {
			account := accountKeeper.NewAccountWithAddress(ctx, acc.Address)
			err := account.SetPubKey(acc.PubKey)
			require.NoError(t, err)
			accountKeeper.SetAccount(ctx, account)
			bankKeeper.sentCoins[acc.Address.String()] = sdk.NewCoins(sdk.NewCoin("ulume", math.NewInt(1000000)))
		}
	}

	k := keeper.NewKeeper(
		cdc,
		runtime.NewKVStoreService(storeKey),
		log.NewNopLogger(),
		authority.String(),
		bankKeeper,
		accountKeeper,
		stakingKeeper,
		distributionKeeper,
		supernodeKeeper,
	)

	// Initialize params
	params := types.DefaultParams()
	params.FoundationFeeShare = "0.1"
	params.SuperNodeFeeShare = "0.9"
	if err := k.SetParams(ctx, params); err != nil {
		panic(err)
	}

	return k, ctx
}
