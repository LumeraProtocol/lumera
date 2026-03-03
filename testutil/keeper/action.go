package keeper

import (
	"context"
	"testing"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"go.uber.org/mock/gomock"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	ibcclienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"
	ibctypes "github.com/cosmos/ibc-go/v10/modules/core/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	actionmodulev1 "github.com/LumeraProtocol/lumera/x/action/v1/module"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	audittypes "github.com/LumeraProtocol/lumera/x/audit/v1/types"
	supernodemocks "github.com/LumeraProtocol/lumera/x/supernode/v1/mocks"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

const (
	// test account amounts
	TestAccountAmount int64 = 1_000_000
)

// ActionBankKeeper extends the existing MockBankKeeper with the SpendableCoins method
type ActionBankKeeper struct {
	mock.Mock
	// sentCoins tracks the coins sent from accounts
	sentCoins map[string]sdk.Coins
	// moduleBalances tracks the balances of modules
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

func (m *ActionBankKeeper) GetModuleBalance(module string) sdk.Coins {
	if coins, ok := m.moduleBalances[module]; ok {
		return coins
	}
	return sdk.NewCoins()
}

func (m *ActionBankKeeper) GetAccountCoins(addr sdk.AccAddress) sdk.Coins {
	if coins, ok := m.sentCoins[addr.String()]; ok {
		return coins
	}
	return sdk.NewCoins()
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

type MockAuditKeeper struct {
	nextEvidenceID uint64
	CreateCalls    []MockAuditKeeperCreateEvidenceCall
}

type MockAuditKeeperCreateEvidenceCall struct {
	ReporterAddress string
	SubjectAddress  string
	ActionID        string
	EvidenceType    audittypes.EvidenceType
	MetadataJSON    string
}

func NewMockAuditKeeper() *MockAuditKeeper {
	return &MockAuditKeeper{nextEvidenceID: 1}
}

func (m *MockAuditKeeper) CreateEvidence(
	ctx context.Context,
	reporterAddress string,
	subjectAddress string,
	actionID string,
	evidenceType audittypes.EvidenceType,
	metadataJSON string,
) (uint64, error) {
	if m.nextEvidenceID == 0 {
		m.nextEvidenceID = 1
	}
	id := m.nextEvidenceID
	m.nextEvidenceID++
	m.CreateCalls = append(m.CreateCalls, MockAuditKeeperCreateEvidenceCall{
		ReporterAddress: reporterAddress,
		SubjectAddress:  subjectAddress,
		ActionID:        actionID,
		EvidenceType:    evidenceType,
		MetadataJSON:    metadataJSON,
	})
	return id, nil
}

type AccountPair struct {
	Address sdk.AccAddress
	PubKey  cryptotypes.PubKey
}

func ActionKeeper(t testing.TB, ctrl *gomock.Controller) (keeper.Keeper, sdk.Context) {
	return ActionKeeperWithAddress(t, ctrl, nil)
}

func ActionKeeperWithAddress(t testing.TB, ctrl *gomock.Controller, accounts []AccountPair) (keeper.Keeper, sdk.Context) {
	storeKey := storetypes.NewKVStoreKey(actiontypes.StoreKey)

	db := dbm.NewMemDB()
	encCfg := moduletestutil.MakeTestEncodingConfig(actionmodulev1.AppModule{})
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	authority := authtypes.NewModuleAddress(govtypes.ModuleName)

	// Create mock keepers
	bankKeeper := NewActionMockBankKeeper()

	authKeeper := NewMockAccountKeeper()

	stakingKeeper := new(MockStakingKeeper)

	supernodeKeeper := supernodemocks.NewMockSupernodeKeeper(ctrl)
	supernodeQueryServer := supernodemocks.NewMockQueryServer(ctrl)

	distributionKeeper := new(MockDistributionKeeper)
	auditKeeper := NewMockAuditKeeper()

	// Set up the context
	ctx := sdk.NewContext(stateStore, cmtproto.Header{}, false, log.NewNopLogger())
	if accounts != nil && len(accounts) > 0 {
		for _, acc := range accounts {
			account := authKeeper.NewAccountWithAddress(ctx, acc.Address)
			err := account.SetPubKey(acc.PubKey)
			require.NoError(t, err)
			authKeeper.SetAccount(ctx, account)
			bankKeeper.sentCoins[acc.Address.String()] = sdk.NewCoins(sdk.NewInt64Coin("ulume", TestAccountAmount))
		}
	}

	mockUpgradeKeeper := newMockUpgradeKeeper()

	storeService := runtime.NewKVStoreService(storeKey)
	k := keeper.NewKeeper(
		cdc,
		authKeeper.AddressCodec(),
		storeService,
		log.NewNopLogger(),
		authority,
		bankKeeper,
		authKeeper,
		stakingKeeper,
		distributionKeeper,
		supernodeKeeper,
		func() sntypes.QueryServer {
			return supernodeQueryServer
		},
		auditKeeper,
		func() *ibckeeper.Keeper {
			return ibckeeper.NewKeeper(encCfg.Codec, storeService, newMockIbcParams(), mockUpgradeKeeper, authority.String())
		},
	)

	// Initialize params
	params := actiontypes.DefaultParams()
	params.FoundationFeeShare = "0.1"
	params.SuperNodeFeeShare = "0.9"
	if err := k.SetParams(ctx, params); err != nil {
		panic(err)
	}

	return k, ctx
}

type MockUpgradeKeeper struct {
	ibcclienttypes.UpgradeKeeper

	initialized bool
}

func (m MockUpgradeKeeper) GetUpgradePlan(ctx context.Context) (upgradetypes.Plan, error) {
	return upgradetypes.Plan{}, nil
}

func newMockUpgradeKeeper() *MockUpgradeKeeper {
	return &MockUpgradeKeeper{initialized: true}
}

type mockIbcParams struct {
	ibctypes.ParamSubspace

	initialized bool
}

func newMockIbcParams() *mockIbcParams {
	return &mockIbcParams{initialized: true}
}

func (mockIbcParams) GetParamSet(ctx sdk.Context, ps paramtypes.ParamSet) {
}
