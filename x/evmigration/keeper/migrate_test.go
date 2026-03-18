package keeper_test

import (
	"fmt"
	"testing"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"cosmossdk.io/x/feegrant"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	claimtypes "github.com/LumeraProtocol/lumera/x/claim/types"
	"github.com/LumeraProtocol/lumera/x/evmigration/keeper"
	evmigrationmocks "github.com/LumeraProtocol/lumera/x/evmigration/mocks"
	module "github.com/LumeraProtocol/lumera/x/evmigration/module"
	"github.com/LumeraProtocol/lumera/x/evmigration/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// mockFixture is a test fixture with gomock-based keeper mocks.
type mockFixture struct {
	ctx                sdk.Context
	keeper             keeper.Keeper
	accountKeeper      *evmigrationmocks.MockAccountKeeper
	bankKeeper         *evmigrationmocks.MockBankKeeper
	stakingKeeper      *evmigrationmocks.MockStakingKeeper
	distributionKeeper *evmigrationmocks.MockDistributionKeeper
	authzKeeper        *evmigrationmocks.MockAuthzKeeper
	feegrantKeeper     *evmigrationmocks.MockFeegrantKeeper
	supernodeKeeper    *evmigrationmocks.MockSupernodeKeeper
	actionKeeper       *evmigrationmocks.MockActionKeeper
	claimKeeper        *evmigrationmocks.MockClaimKeeper
}

func initMockFixture(t *testing.T) *mockFixture {
	t.Helper()

	ctrl := gomock.NewController(t)

	accountKeeper := evmigrationmocks.NewMockAccountKeeper(ctrl)
	bankKeeper := evmigrationmocks.NewMockBankKeeper(ctrl)
	stakingKeeper := evmigrationmocks.NewMockStakingKeeper(ctrl)
	distributionKeeper := evmigrationmocks.NewMockDistributionKeeper(ctrl)
	authzKeeper := evmigrationmocks.NewMockAuthzKeeper(ctrl)
	feegrantKeeper := evmigrationmocks.NewMockFeegrantKeeper(ctrl)
	supernodeKeeper := evmigrationmocks.NewMockSupernodeKeeper(ctrl)
	actionKeeper := evmigrationmocks.NewMockActionKeeper(ctrl)
	claimKeeper := evmigrationmocks.NewMockClaimKeeper(ctrl)

	encCfg := moduletestutil.MakeTestEncodingConfig(module.AppModule{})
	addrCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	storeService := runtime.NewKVStoreService(storeKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx

	authority := authtypes.NewModuleAddress(types.GovModuleName)

	k := keeper.NewKeeper(
		storeService,
		encCfg.Codec,
		addrCodec,
		authority,
		accountKeeper,
		bankKeeper,
		stakingKeeper,
		distributionKeeper,
		authzKeeper,
		feegrantKeeper,
		supernodeKeeper,
		actionKeeper,
		claimKeeper,
	)

	// Initialize params with migration enabled.
	params := types.NewParams(true, 0, 50, 2000)
	require.NoError(t, k.Params.Set(ctx, params))
	require.NoError(t, k.MigrationCounter.Set(ctx, 0))
	require.NoError(t, k.ValidatorMigrationCounter.Set(ctx, 0))

	return &mockFixture{
		ctx:                ctx,
		keeper:             k,
		accountKeeper:      accountKeeper,
		bankKeeper:         bankKeeper,
		stakingKeeper:      stakingKeeper,
		distributionKeeper: distributionKeeper,
		authzKeeper:        authzKeeper,
		feegrantKeeper:     feegrantKeeper,
		supernodeKeeper:    supernodeKeeper,
		actionKeeper:       actionKeeper,
		claimKeeper:        claimKeeper,
	}
}

func testAccAddr() sdk.AccAddress {
	return sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
}

func expectHistoricalRewardsLookup(
	mock *evmigrationmocks.MockDistributionKeeper,
	val sdk.ValAddress,
	period uint64,
	refCount uint32,
) {
	mock.EXPECT().IterateValidatorHistoricalRewards(gomock.Any(), gomock.Any()).
		Do(func(_ any, cb func(sdk.ValAddress, uint64, distrtypes.ValidatorHistoricalRewards) bool) {
			cb(val, period, distrtypes.ValidatorHistoricalRewards{ReferenceCount: refCount})
		})
}

func expectHistoricalRewardsIncrement(
	mock *evmigrationmocks.MockDistributionKeeper,
	val sdk.ValAddress,
	period uint64,
	refCount uint32,
) {
	expectHistoricalRewardsLookup(mock, val, period, refCount)
	mock.EXPECT().SetValidatorHistoricalRewards(gomock.Any(), val, period, gomock.Any()).Return(nil)
}

// expectHistoricalRewardsReset sets up mock expectations for
// resetHistoricalRewardsReferenceCount: iterate to find the period, then set refcount to 1.
func expectHistoricalRewardsReset(
	mock *evmigrationmocks.MockDistributionKeeper,
	val sdk.ValAddress,
	period uint64,
	refCount uint32,
) {
	expectHistoricalRewardsLookup(mock, val, period, refCount)
	mock.EXPECT().SetValidatorHistoricalRewards(gomock.Any(), val, period, gomock.Any()).Return(nil)
}

// --- MigrateAuth tests ---

// TestMigrateAuth_BaseAccount verifies that a plain BaseAccount is removed
// from the legacy address and a new account is created at the new address.
func TestMigrateAuth_BaseAccount(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	baseAcc := authtypes.NewBaseAccountWithAddress(legacy)

	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacy).Return(baseAcc)
	f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), baseAcc)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
	newAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	f.accountKeeper.EXPECT().NewAccountWithAddress(gomock.Any(), newAddr).Return(newAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), newAcc)

	vi, err := f.keeper.MigrateAuth(f.ctx, legacy, newAddr)
	require.NoError(t, err)
	require.Nil(t, vi)
}

// TestMigrateAuth_ContinuousVesting verifies that ContinuousVestingAccount
// parameters (start time, end time, original vesting) are captured in VestingInfo.
func TestMigrateAuth_ContinuousVesting(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	baseAcc := authtypes.NewBaseAccountWithAddress(legacy)
	origVesting := sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000))
	bva, err := vestingtypes.NewBaseVestingAccount(baseAcc, origVesting, 1000000)
	require.NoError(t, err)
	cva := vestingtypes.NewContinuousVestingAccountRaw(bva, 500000)

	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacy).Return(cva)
	f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), cva)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
	newAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	f.accountKeeper.EXPECT().NewAccountWithAddress(gomock.Any(), newAddr).Return(newAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), newAcc)

	vi, err := f.keeper.MigrateAuth(f.ctx, legacy, newAddr)
	require.NoError(t, err)
	require.NotNil(t, vi)
	require.Equal(t, origVesting, vi.OriginalVesting)
	require.Equal(t, int64(1000000), vi.EndTime)
	require.Equal(t, int64(500000), vi.StartTime)
}

// TestMigrateAuth_DelayedVesting verifies that DelayedVestingAccount parameters
// are captured in VestingInfo.
func TestMigrateAuth_DelayedVesting(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	baseAcc := authtypes.NewBaseAccountWithAddress(legacy)
	origVesting := sdk.NewCoins(sdk.NewInt64Coin("ulume", 500))
	bva, err := vestingtypes.NewBaseVestingAccount(baseAcc, origVesting, 2000000)
	require.NoError(t, err)
	dva := vestingtypes.NewDelayedVestingAccountRaw(bva)

	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacy).Return(dva)
	f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), dva)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
	newAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	f.accountKeeper.EXPECT().NewAccountWithAddress(gomock.Any(), newAddr).Return(newAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), newAcc)

	vi, err := f.keeper.MigrateAuth(f.ctx, legacy, newAddr)
	require.NoError(t, err)
	require.NotNil(t, vi)
	require.Equal(t, int64(2000000), vi.EndTime)
}

// TestMigrateAuth_PeriodicVesting verifies that PeriodicVestingAccount parameters
// including vesting periods are captured in VestingInfo.
func TestMigrateAuth_PeriodicVesting(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	baseAcc := authtypes.NewBaseAccountWithAddress(legacy)
	origVesting := sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000))
	bva, err := vestingtypes.NewBaseVestingAccount(baseAcc, origVesting, 3000000)
	require.NoError(t, err)
	periods := vestingtypes.Periods{
		{Length: 100000, Amount: sdk.NewCoins(sdk.NewInt64Coin("ulume", 500))},
		{Length: 200000, Amount: sdk.NewCoins(sdk.NewInt64Coin("ulume", 500))},
	}
	pva := vestingtypes.NewPeriodicVestingAccountRaw(bva, 1000000, periods)

	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacy).Return(pva)
	f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), pva)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
	newAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	f.accountKeeper.EXPECT().NewAccountWithAddress(gomock.Any(), newAddr).Return(newAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), newAcc)

	vi, err := f.keeper.MigrateAuth(f.ctx, legacy, newAddr)
	require.NoError(t, err)
	require.NotNil(t, vi)
	require.Len(t, vi.Periods, 2)
}

// TestMigrateAuth_PermanentLocked verifies that PermanentLockedAccount parameters
// are captured in VestingInfo.
func TestMigrateAuth_PermanentLocked(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	baseAcc := authtypes.NewBaseAccountWithAddress(legacy)
	origVesting := sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000))
	pla, err := vestingtypes.NewPermanentLockedAccount(baseAcc, origVesting)
	require.NoError(t, err)

	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacy).Return(pla)
	f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), pla)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
	newAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	f.accountKeeper.EXPECT().NewAccountWithAddress(gomock.Any(), newAddr).Return(newAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), newAcc)

	vi, err := f.keeper.MigrateAuth(f.ctx, legacy, newAddr)
	require.NoError(t, err)
	require.NotNil(t, vi)
	require.Equal(t, origVesting, vi.OriginalVesting)
}

// TestMigrateAuth_ModuleAccount verifies that module accounts are rejected.
func TestMigrateAuth_ModuleAccount(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	modAcc := authtypes.NewEmptyModuleAccount("bonded_tokens_pool")
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacy).Return(modAcc)

	vi, err := f.keeper.MigrateAuth(f.ctx, legacy, newAddr)
	require.ErrorIs(t, err, types.ErrCannotMigrateModuleAccount)
	require.Nil(t, vi)
}

// TestMigrateAuth_AccountNotFound verifies error when legacy account does not exist.
func TestMigrateAuth_AccountNotFound(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacy).Return(nil)

	vi, err := f.keeper.MigrateAuth(f.ctx, legacy, newAddr)
	require.ErrorIs(t, err, types.ErrLegacyAccountNotFound)
	require.Nil(t, vi)
}

// TestMigrateAuth_NewAddressAlreadyExists verifies that if the new address already
// has an account, it is reused instead of creating a new one.
func TestMigrateAuth_NewAddressAlreadyExists(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	baseAcc := authtypes.NewBaseAccountWithAddress(legacy)

	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacy).Return(baseAcc)
	f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), baseAcc)
	// New address already has an account — should not create a new one.
	existingAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(existingAcc)

	vi, err := f.keeper.MigrateAuth(f.ctx, legacy, newAddr)
	require.NoError(t, err)
	require.Nil(t, vi)
}

// --- FinalizeVestingAccount tests ---

// TestFinalizeVestingAccount_Continuous verifies that a ContinuousVestingAccount
// is correctly recreated at the new address from VestingInfo.
func TestFinalizeVestingAccount_Continuous(t *testing.T) {
	f := initMockFixture(t)
	newAddr := testAccAddr()
	baseAcc := authtypes.NewBaseAccountWithAddress(newAddr)

	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(baseAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), gomock.Any())

	vi := &keeper.VestingInfo{
		Type:            keeper.VestingTypeContinuous,
		OriginalVesting: sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000)),
		EndTime:         1000000,
		StartTime:       500000,
	}

	err := f.keeper.FinalizeVestingAccount(f.ctx, newAddr, vi)
	require.NoError(t, err)
}

// TestFinalizeVestingAccount_PreservesDelegatedBalances verifies delegated
// vesting/free balances are preserved for all vesting account types.
func TestFinalizeVestingAccount_PreservesDelegatedBalances(t *testing.T) {
	testCases := []struct {
		name string
		vi   *keeper.VestingInfo
	}{
		{
			name: "continuous",
			vi: &keeper.VestingInfo{
				Type:             keeper.VestingTypeContinuous,
				OriginalVesting:  sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000)),
				DelegatedFree:    sdk.NewCoins(sdk.NewInt64Coin("ulume", 11)),
				DelegatedVesting: sdk.NewCoins(sdk.NewInt64Coin("ulume", 22)),
				EndTime:          1000000,
				StartTime:        500000,
			},
		},
		{
			name: "delayed",
			vi: &keeper.VestingInfo{
				Type:             keeper.VestingTypeDelayed,
				OriginalVesting:  sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000)),
				DelegatedFree:    sdk.NewCoins(sdk.NewInt64Coin("ulume", 33)),
				DelegatedVesting: sdk.NewCoins(sdk.NewInt64Coin("ulume", 44)),
				EndTime:          1000000,
			},
		},
		{
			name: "periodic",
			vi: &keeper.VestingInfo{
				Type:             keeper.VestingTypePeriodic,
				OriginalVesting:  sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000)),
				DelegatedFree:    sdk.NewCoins(sdk.NewInt64Coin("ulume", 55)),
				DelegatedVesting: sdk.NewCoins(sdk.NewInt64Coin("ulume", 66)),
				EndTime:          3000000,
				StartTime:        1000000,
				Periods: vestingtypes.Periods{
					{Length: 100000, Amount: sdk.NewCoins(sdk.NewInt64Coin("ulume", 500))},
					{Length: 200000, Amount: sdk.NewCoins(sdk.NewInt64Coin("ulume", 500))},
				},
			},
		},
		{
			name: "permanent_locked",
			vi: &keeper.VestingInfo{
				Type:             keeper.VestingTypePermanentLocked,
				OriginalVesting:  sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000)),
				DelegatedFree:    sdk.NewCoins(sdk.NewInt64Coin("ulume", 77)),
				DelegatedVesting: sdk.NewCoins(sdk.NewInt64Coin("ulume", 88)),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f := initMockFixture(t)
			newAddr := testAccAddr()
			baseAcc := authtypes.NewBaseAccountWithAddress(newAddr)

			f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(baseAcc)
			f.accountKeeper.EXPECT().SetAccount(gomock.Any(), gomock.Any()).Do(func(_ any, acc sdk.AccountI) {
				switch va := acc.(type) {
				case *vestingtypes.ContinuousVestingAccount:
					require.Equal(t, tc.vi.DelegatedFree, va.DelegatedFree)
					require.Equal(t, tc.vi.DelegatedVesting, va.DelegatedVesting)
				case *vestingtypes.DelayedVestingAccount:
					require.Equal(t, tc.vi.DelegatedFree, va.DelegatedFree)
					require.Equal(t, tc.vi.DelegatedVesting, va.DelegatedVesting)
				case *vestingtypes.PeriodicVestingAccount:
					require.Equal(t, tc.vi.DelegatedFree, va.DelegatedFree)
					require.Equal(t, tc.vi.DelegatedVesting, va.DelegatedVesting)
				case *vestingtypes.PermanentLockedAccount:
					require.Equal(t, tc.vi.DelegatedFree, va.DelegatedFree)
					require.Equal(t, tc.vi.DelegatedVesting, va.DelegatedVesting)
				default:
					t.Fatalf("unexpected vesting account type: %T", acc)
				}
			})

			err := f.keeper.FinalizeVestingAccount(f.ctx, newAddr, tc.vi)
			require.NoError(t, err)
		})
	}
}

// TestFinalizeVestingAccount_AccountNotFound verifies error when the new account
// does not exist at finalization time.
func TestFinalizeVestingAccount_AccountNotFound(t *testing.T) {
	f := initMockFixture(t)
	newAddr := testAccAddr()

	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)

	vi := &keeper.VestingInfo{
		Type:            keeper.VestingTypeContinuous,
		OriginalVesting: sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000)),
		EndTime:         1000000,
		StartTime:       500000,
	}

	err := f.keeper.FinalizeVestingAccount(f.ctx, newAddr, vi)
	require.Error(t, err)
}

// --- MigrateBank tests ---

// TestMigrateBank_WithBalance verifies that all balances are transferred via SendCoins.
func TestMigrateBank_WithBalance(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	balances := sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000))

	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), legacy).Return(balances)
	f.bankKeeper.EXPECT().SendCoins(gomock.Any(), legacy, newAddr, balances).Return(nil)

	err := f.keeper.MigrateBank(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// TestMigrateBank_ZeroBalance verifies that SendCoins is not called when balance is zero.
func TestMigrateBank_ZeroBalance(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), legacy).Return(sdk.Coins{})

	// SendCoins should NOT be called when balance is zero.
	err := f.keeper.MigrateBank(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// TestMigrateBank_MultiDenom verifies that multi-denom balances are transferred correctly.
func TestMigrateBank_MultiDenom(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	balances := sdk.NewCoins(
		sdk.NewInt64Coin("ulume", 500),
		sdk.NewInt64Coin("uatom", 200),
	)

	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), legacy).Return(balances)
	f.bankKeeper.EXPECT().SendCoins(gomock.Any(), legacy, newAddr, balances).Return(nil)

	err := f.keeper.MigrateBank(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// --- MigrateDistribution tests ---

// TestMigrateDistribution_WithDelegations verifies that pending rewards are
// withdrawn for all delegations.
func TestMigrateDistribution_WithDelegations(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	valAddr := sdk.ValAddress(testAccAddr())

	delegations := []stakingtypes.Delegation{
		stakingtypes.NewDelegation(legacy.String(), valAddr.String(), math.LegacyNewDec(100)),
	}

	// redirectWithdrawAddrIfMigrated: withdraw addr returns self — no redirect needed.
	f.distributionKeeper.EXPECT().GetDelegatorWithdrawAddr(gomock.Any(), legacy).Return(legacy, nil)

	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacy, ^uint16(0)).Return(delegations, nil)
	f.distributionKeeper.EXPECT().GetDelegatorStartingInfo(gomock.Any(), valAddr, legacy).Return(
		distrtypes.DelegatorStartingInfo{PreviousPeriod: 4}, nil,
	)
	expectHistoricalRewardsLookup(f.distributionKeeper, valAddr, 4, 1)
	f.distributionKeeper.EXPECT().WithdrawDelegationRewards(gomock.Any(), legacy, valAddr).Return(sdk.Coins{}, nil)

	err := f.keeper.MigrateDistribution(f.ctx, legacy)
	require.NoError(t, err)
}

// TestMigrateDistribution_NoDelegations verifies no-op when there are no delegations.
func TestMigrateDistribution_NoDelegations(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()

	// redirectWithdrawAddrIfMigrated: withdraw addr returns self — no redirect needed.
	f.distributionKeeper.EXPECT().GetDelegatorWithdrawAddr(gomock.Any(), legacy).Return(legacy, nil)

	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)

	err := f.keeper.MigrateDistribution(f.ctx, legacy)
	require.NoError(t, err)
}

// --- MigrateAuthz tests ---

// TestMigrateAuthz_AsGranter verifies that grants where legacy is the granter
// are re-keyed to the new address.
func TestMigrateAuthz_AsGranter(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	grantee := testAccAddr()

	genericAuth := authz.NewGenericAuthorization("/cosmos.bank.v1beta1.MsgSend")
	grant, err := authz.NewGrant(f.ctx.BlockTime(), genericAuth, nil)
	require.NoError(t, err)

	f.authzKeeper.EXPECT().IterateGrants(gomock.Any(), gomock.Any()).
		Do(func(_ any, cb func(sdk.AccAddress, sdk.AccAddress, authz.Grant) bool) {
			cb(legacy, grantee, grant)
		})
	f.authzKeeper.EXPECT().DeleteGrant(gomock.Any(), grantee, legacy, "/cosmos.bank.v1beta1.MsgSend").Return(nil)
	f.authzKeeper.EXPECT().SaveGrant(gomock.Any(), grantee, newAddr, genericAuth, grant.Expiration).Return(nil)

	err = f.keeper.MigrateAuthz(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// TestMigrateAuthz_AsGrantee verifies that grants where legacy is the grantee
// are re-keyed to the new address.
func TestMigrateAuthz_AsGrantee(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	granter := testAccAddr()

	genericAuth := authz.NewGenericAuthorization("/cosmos.bank.v1beta1.MsgSend")
	grant, err := authz.NewGrant(f.ctx.BlockTime(), genericAuth, nil)
	require.NoError(t, err)

	f.authzKeeper.EXPECT().IterateGrants(gomock.Any(), gomock.Any()).
		Do(func(_ any, cb func(sdk.AccAddress, sdk.AccAddress, authz.Grant) bool) {
			cb(granter, legacy, grant)
		})
	f.authzKeeper.EXPECT().DeleteGrant(gomock.Any(), legacy, granter, "/cosmos.bank.v1beta1.MsgSend").Return(nil)
	f.authzKeeper.EXPECT().SaveGrant(gomock.Any(), newAddr, granter, genericAuth, grant.Expiration).Return(nil)

	err = f.keeper.MigrateAuthz(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// TestMigrateAuthz_NoGrants verifies no-op when there are no authz grants.
func TestMigrateAuthz_NoGrants(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	f.authzKeeper.EXPECT().IterateGrants(gomock.Any(), gomock.Any())

	err := f.keeper.MigrateAuthz(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// --- MigrateFeegrant tests ---

// TestMigrateFeegrant_AsGranter verifies that fee allowances where legacy is the
// granter are re-created at the new address.
func TestMigrateFeegrant_AsGranter(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	grantee := testAccAddr()

	allowance := &feegrant.BasicAllowance{SpendLimit: sdk.NewCoins(sdk.NewInt64Coin("ulume", 100))}
	grant, err := feegrant.NewGrant(legacy, grantee, allowance)
	require.NoError(t, err)

	f.feegrantKeeper.EXPECT().IterateAllFeeAllowances(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, cb func(feegrant.Grant) bool) error {
			cb(grant)
			return nil
		})
	f.feegrantKeeper.EXPECT().GrantAllowance(gomock.Any(), newAddr, grantee, allowance).Return(nil)

	err = f.keeper.MigrateFeegrant(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// TestMigrateFeegrant_NoAllowances verifies no-op when there are no fee allowances.
func TestMigrateFeegrant_NoAllowances(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	f.feegrantKeeper.EXPECT().IterateAllFeeAllowances(gomock.Any(), gomock.Any()).Return(nil)

	err := f.keeper.MigrateFeegrant(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// --- MigrateSupernode tests ---

// TestMigrateSupernode_Found verifies that the supernode account field is updated
// from legacy to new address and PrevSupernodeAccounts history is maintained.
func TestMigrateSupernode_Found(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	sn := sntypes.SuperNode{
		SupernodeAccount: legacy.String(),
		ValidatorAddress: sdk.ValAddress(legacy).String(),
		PrevSupernodeAccounts: []*sntypes.SupernodeAccountHistory{
			{Account: legacy.String(), Height: 1},
		},
	}

	f.supernodeKeeper.EXPECT().GetSuperNodeByAccount(gomock.Any(), legacy.String()).Return(sn, true, nil)
	f.supernodeKeeper.EXPECT().SetSuperNode(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, updated sntypes.SuperNode) error {
			require.Equal(t, newAddr.String(), updated.SupernodeAccount)
			// Existing legacy entry should be rewritten to new address.
			require.Len(t, updated.PrevSupernodeAccounts, 2)
			require.Equal(t, newAddr.String(), updated.PrevSupernodeAccounts[0].Account)
			require.Equal(t, int64(1), updated.PrevSupernodeAccounts[0].Height)
			// New migration entry appended.
			require.Equal(t, newAddr.String(), updated.PrevSupernodeAccounts[1].Account)
			return nil
		})

	err := f.keeper.MigrateSupernode(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// TestMigrateSupernode_NotFound verifies no-op when legacy is not a supernode.
func TestMigrateSupernode_NotFound(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	f.supernodeKeeper.EXPECT().GetSuperNodeByAccount(gomock.Any(), legacy.String()).Return(sntypes.SuperNode{}, false, nil)

	err := f.keeper.MigrateSupernode(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// --- MigrateActions tests ---

// TestMigrateActions_CreatorAndSuperNodes verifies that both the Creator field
// and SuperNodes array entries are updated from legacy to new address.
func TestMigrateActions_CreatorAndSuperNodes(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	otherAddr := testAccAddr()

	action := &actiontypes.Action{
		ActionID:   "action-1",
		Creator:    legacy.String(),
		SuperNodes: []string{legacy.String(), otherAddr.String()},
	}

	f.actionKeeper.EXPECT().IterateActions(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, cb func(*actiontypes.Action) bool) error {
			cb(action)
			return nil
		})
	f.actionKeeper.EXPECT().SetAction(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, updated *actiontypes.Action) error {
			require.Equal(t, newAddr.String(), updated.Creator)
			require.Equal(t, newAddr.String(), updated.SuperNodes[0])
			require.Equal(t, otherAddr.String(), updated.SuperNodes[1])
			return nil
		})

	err := f.keeper.MigrateActions(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// TestMigrateActions_NoMatch verifies no-op when no actions reference legacy address.
func TestMigrateActions_NoMatch(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	f.actionKeeper.EXPECT().IterateActions(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, cb func(*actiontypes.Action) bool) error {
			// No actions match legacy address.
			cb(&actiontypes.Action{
				ActionID:   "action-1",
				Creator:    testAccAddr().String(),
				SuperNodes: []string{testAccAddr().String()},
			})
			return nil
		})

	err := f.keeper.MigrateActions(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// --- MigrateClaim tests ---

// TestMigrateClaim_Found verifies that the claim record's DestAddress is updated.
func TestMigrateClaim_Found(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	record := claimtypes.ClaimRecord{
		OldAddress:  "pastel1legacyoldaddress",
		DestAddress: legacy.String(),
	}

	f.claimKeeper.EXPECT().IterateClaimRecords(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, cb func(claimtypes.ClaimRecord) (bool, error)) error {
			_, err := cb(record)
			return err
		})
	f.claimKeeper.EXPECT().GetClaimRecord(gomock.Any(), record.OldAddress).Return(record, true, nil)
	f.claimKeeper.EXPECT().SetClaimRecord(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, updated claimtypes.ClaimRecord) error {
			require.Equal(t, newAddr.String(), updated.DestAddress)
			return nil
		})

	err := f.keeper.MigrateClaim(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// TestMigrateClaim_NotFound verifies no-op when there is no claim record.
func TestMigrateClaim_NotFound(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	f.claimKeeper.EXPECT().IterateClaimRecords(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, cb func(claimtypes.ClaimRecord) (bool, error)) error {
			_, err := cb(claimtypes.ClaimRecord{
				OldAddress:  "pastel1otheroldaddress",
				DestAddress: testAccAddr().String(),
			})
			return err
		})

	err := f.keeper.MigrateClaim(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// --- MigrateStaking tests ---

// TestMigrateStaking_ActiveDelegations verifies the full staking migration flow:
// active delegation re-keying, distribution starting info, and withdraw address.
func TestMigrateStaking_ActiveDelegations(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	valAddr := sdk.ValAddress(testAccAddr())

	del := stakingtypes.NewDelegation(legacy.String(), valAddr.String(), math.LegacyNewDec(100))

	// migrateActiveDelegations
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacy, ^uint16(0)).Return([]stakingtypes.Delegation{del}, nil)
	f.distributionKeeper.EXPECT().DeleteDelegatorStartingInfo(gomock.Any(), valAddr, legacy).Return(nil)
	f.stakingKeeper.EXPECT().RemoveDelegation(gomock.Any(), del).Return(nil)
	f.stakingKeeper.EXPECT().SetDelegation(gomock.Any(), gomock.Any()).Return(nil)
	f.distributionKeeper.EXPECT().GetValidatorCurrentRewards(gomock.Any(), valAddr).Return(distrtypes.ValidatorCurrentRewards{Period: 5}, nil)
	f.distributionKeeper.EXPECT().GetDelegatorStartingInfo(gomock.Any(), valAddr, legacy).Return(distrtypes.DelegatorStartingInfo{}, nil)
	expectHistoricalRewardsIncrement(f.distributionKeeper, valAddr, 4, 1)
	f.distributionKeeper.EXPECT().SetDelegatorStartingInfo(gomock.Any(), valAddr, newAddr, gomock.Any()).Return(nil)

	// migrateUnbondingDelegations
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)

	// migrateRedelegations
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)

	// migrateWithdrawAddress
	f.distributionKeeper.EXPECT().GetDelegatorWithdrawAddr(gomock.Any(), legacy).Return(legacy, nil)
	f.distributionKeeper.EXPECT().SetDelegatorWithdrawAddr(gomock.Any(), newAddr, newAddr).Return(nil)

	err := f.keeper.MigrateStaking(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// TestMigrateStaking_NoDelegations verifies no-op when delegator has no delegations.
func TestMigrateStaking_NoDelegations(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	// migrateActiveDelegations — no delegations.
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)
	// migrateWithdrawAddress — no custom withdraw address.
	f.distributionKeeper.EXPECT().GetDelegatorWithdrawAddr(gomock.Any(), legacy).Return(nil, fmt.Errorf("not set"))

	err := f.keeper.MigrateStaking(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// TestMigrateStaking_ThirdPartyWithdrawAddress verifies that a third-party
// withdraw address is preserved (not replaced with newAddr).
func TestMigrateStaking_ThirdPartyWithdrawAddress(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	thirdParty := testAccAddr()

	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)
	f.distributionKeeper.EXPECT().GetDelegatorWithdrawAddr(gomock.Any(), legacy).Return(thirdParty, nil)
	f.distributionKeeper.EXPECT().SetDelegatorWithdrawAddr(gomock.Any(), newAddr, thirdParty).Return(nil)

	err := f.keeper.MigrateStaking(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// --- Unbonding delegation re-key tests ---

// TestMigrateStaking_WithUnbondingDelegation verifies that unbonding delegations
// are re-keyed from legacy to new address, including queue and UnbondingId indexes.
func TestMigrateStaking_WithUnbondingDelegation(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	valAddr := sdk.ValAddress(testAccAddr())

	del := stakingtypes.NewDelegation(legacy.String(), valAddr.String(), math.LegacyNewDec(100))
	completionTime := f.ctx.BlockTime().Add(21 * 24 * 3600 * 1e9) // 21 days
	ubd := stakingtypes.UnbondingDelegation{
		DelegatorAddress: legacy.String(),
		ValidatorAddress: valAddr.String(),
		Entries: []stakingtypes.UnbondingDelegationEntry{
			{
				CreationHeight: 10,
				CompletionTime: completionTime,
				InitialBalance: math.NewInt(50),
				Balance:        math.NewInt(50),
				UnbondingId:    42,
			},
		},
	}

	// migrateActiveDelegations
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacy, ^uint16(0)).Return([]stakingtypes.Delegation{del}, nil)
	f.distributionKeeper.EXPECT().DeleteDelegatorStartingInfo(gomock.Any(), valAddr, legacy).Return(nil)
	f.stakingKeeper.EXPECT().RemoveDelegation(gomock.Any(), del).Return(nil)
	f.stakingKeeper.EXPECT().SetDelegation(gomock.Any(), gomock.Any()).Return(nil)
	f.distributionKeeper.EXPECT().GetValidatorCurrentRewards(gomock.Any(), valAddr).Return(distrtypes.ValidatorCurrentRewards{Period: 5}, nil)
	f.distributionKeeper.EXPECT().GetDelegatorStartingInfo(gomock.Any(), valAddr, legacy).Return(distrtypes.DelegatorStartingInfo{}, nil)
	expectHistoricalRewardsIncrement(f.distributionKeeper, valAddr, 4, 1)
	f.distributionKeeper.EXPECT().SetDelegatorStartingInfo(gomock.Any(), valAddr, newAddr, gomock.Any()).Return(nil)

	// migrateUnbondingDelegations
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacy, ^uint16(0)).Return([]stakingtypes.UnbondingDelegation{ubd}, nil)
	f.stakingKeeper.EXPECT().RemoveUnbondingDelegation(gomock.Any(), ubd).Return(nil)
	f.stakingKeeper.EXPECT().SetUnbondingDelegation(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, newUbd stakingtypes.UnbondingDelegation) error {
			require.Equal(t, newAddr.String(), newUbd.DelegatorAddress)
			require.Equal(t, valAddr.String(), newUbd.ValidatorAddress)
			require.Len(t, newUbd.Entries, 1)
			return nil
		})
	f.stakingKeeper.EXPECT().InsertUBDQueue(gomock.Any(), gomock.Any(), completionTime).Return(nil)
	f.stakingKeeper.EXPECT().SetUnbondingDelegationByUnbondingID(gomock.Any(), gomock.Any(), uint64(42)).Return(nil)

	// migrateRedelegations
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)

	// migrateWithdrawAddress
	f.distributionKeeper.EXPECT().GetDelegatorWithdrawAddr(gomock.Any(), legacy).Return(nil, fmt.Errorf("not set"))

	err := f.keeper.MigrateStaking(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// TestMigrateStaking_WithRedelegation verifies that redelegations are re-keyed
// from legacy to new address, including queue and UnbondingId indexes.
func TestMigrateStaking_WithRedelegation(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	srcValAddr := sdk.ValAddress(testAccAddr())
	dstValAddr := sdk.ValAddress(testAccAddr())

	del := stakingtypes.NewDelegation(legacy.String(), srcValAddr.String(), math.LegacyNewDec(100))
	completionTime := f.ctx.BlockTime().Add(21 * 24 * 3600 * 1e9)
	red := stakingtypes.Redelegation{
		DelegatorAddress:    legacy.String(),
		ValidatorSrcAddress: srcValAddr.String(),
		ValidatorDstAddress: dstValAddr.String(),
		Entries: []stakingtypes.RedelegationEntry{
			{
				CreationHeight: 10,
				CompletionTime: completionTime,
				InitialBalance: math.NewInt(30),
				SharesDst:      math.LegacyNewDec(30),
				UnbondingId:    99,
			},
		},
	}

	// migrateActiveDelegations
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacy, ^uint16(0)).Return([]stakingtypes.Delegation{del}, nil)
	f.distributionKeeper.EXPECT().DeleteDelegatorStartingInfo(gomock.Any(), srcValAddr, legacy).Return(nil)
	f.stakingKeeper.EXPECT().RemoveDelegation(gomock.Any(), del).Return(nil)
	f.stakingKeeper.EXPECT().SetDelegation(gomock.Any(), gomock.Any()).Return(nil)
	f.distributionKeeper.EXPECT().GetValidatorCurrentRewards(gomock.Any(), srcValAddr).Return(distrtypes.ValidatorCurrentRewards{Period: 3}, nil)
	f.distributionKeeper.EXPECT().GetDelegatorStartingInfo(gomock.Any(), srcValAddr, legacy).Return(distrtypes.DelegatorStartingInfo{}, nil)
	expectHistoricalRewardsIncrement(f.distributionKeeper, srcValAddr, 2, 1)
	f.distributionKeeper.EXPECT().SetDelegatorStartingInfo(gomock.Any(), srcValAddr, newAddr, gomock.Any()).Return(nil)

	// migrateUnbondingDelegations
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)

	// migrateRedelegations
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacy, ^uint16(0)).Return([]stakingtypes.Redelegation{red}, nil)
	f.stakingKeeper.EXPECT().RemoveRedelegation(gomock.Any(), red).Return(nil)
	f.stakingKeeper.EXPECT().SetRedelegation(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, newRed stakingtypes.Redelegation) error {
			require.Equal(t, newAddr.String(), newRed.DelegatorAddress)
			require.Equal(t, srcValAddr.String(), newRed.ValidatorSrcAddress)
			require.Equal(t, dstValAddr.String(), newRed.ValidatorDstAddress)
			require.Len(t, newRed.Entries, 1)
			return nil
		})
	f.stakingKeeper.EXPECT().InsertRedelegationQueue(gomock.Any(), gomock.Any(), completionTime).Return(nil)
	f.stakingKeeper.EXPECT().SetRedelegationByUnbondingID(gomock.Any(), gomock.Any(), uint64(99)).Return(nil)

	// migrateWithdrawAddress
	f.distributionKeeper.EXPECT().GetDelegatorWithdrawAddr(gomock.Any(), legacy).Return(nil, fmt.Errorf("not set"))

	err := f.keeper.MigrateStaking(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// TestMigrateStaking_UnbondingWithoutActiveDelegation verifies that unbonding
// delegations are still migrated when the delegator no longer has an active
// delegation to the validator.
func TestMigrateStaking_UnbondingWithoutActiveDelegation(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	valAddr := sdk.ValAddress(testAccAddr())

	completionTime := f.ctx.BlockTime().Add(21 * 24 * 3600 * 1e9)
	ubd := stakingtypes.UnbondingDelegation{
		DelegatorAddress: legacy.String(),
		ValidatorAddress: valAddr.String(),
		Entries: []stakingtypes.UnbondingDelegationEntry{
			{
				CreationHeight: 11,
				CompletionTime: completionTime,
				InitialBalance: math.NewInt(40),
				Balance:        math.NewInt(40),
				UnbondingId:    77,
			},
		},
	}

	// migrateActiveDelegations
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)

	// migrateUnbondingDelegations
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacy, ^uint16(0)).Return([]stakingtypes.UnbondingDelegation{ubd}, nil)
	f.stakingKeeper.EXPECT().RemoveUnbondingDelegation(gomock.Any(), ubd).Return(nil)
	f.stakingKeeper.EXPECT().SetUnbondingDelegation(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, newUbd stakingtypes.UnbondingDelegation) error {
			require.Equal(t, newAddr.String(), newUbd.DelegatorAddress)
			require.Equal(t, valAddr.String(), newUbd.ValidatorAddress)
			require.Len(t, newUbd.Entries, 1)
			return nil
		})
	f.stakingKeeper.EXPECT().InsertUBDQueue(gomock.Any(), gomock.Any(), completionTime).Return(nil)
	f.stakingKeeper.EXPECT().SetUnbondingDelegationByUnbondingID(gomock.Any(), gomock.Any(), uint64(77)).Return(nil)

	// migrateRedelegations
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)

	// migrateWithdrawAddress
	f.distributionKeeper.EXPECT().GetDelegatorWithdrawAddr(gomock.Any(), legacy).Return(nil, fmt.Errorf("not set"))

	err := f.keeper.MigrateStaking(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// --- Validator-specific delegation re-key tests ---

// TestMigrateValidatorDelegations_WithUnbondingAndRedelegation verifies
// MigrateValidatorDelegations re-keys unbonding delegations and redelegations
// with UnbondingId indexes.
func TestMigrateValidatorDelegations_WithUnbondingAndRedelegation(t *testing.T) {
	f := initMockFixture(t)
	oldValAddr := sdk.ValAddress(testAccAddr())
	newValAddr := sdk.ValAddress(testAccAddr())
	delegator := testAccAddr()

	completionTime := f.ctx.BlockTime().Add(21 * 24 * 3600 * 1e9)

	// No active delegations.
	f.stakingKeeper.EXPECT().GetValidatorDelegations(gomock.Any(), oldValAddr).Return(nil, nil)

	// One unbonding delegation with an UnbondingId.
	ubd := stakingtypes.UnbondingDelegation{
		DelegatorAddress: delegator.String(),
		ValidatorAddress: oldValAddr.String(),
		Entries: []stakingtypes.UnbondingDelegationEntry{
			{
				CreationHeight: 5,
				CompletionTime: completionTime,
				InitialBalance: math.NewInt(100),
				Balance:        math.NewInt(100),
				UnbondingId:    77,
			},
		},
	}
	f.stakingKeeper.EXPECT().GetUnbondingDelegationsFromValidator(gomock.Any(), oldValAddr).Return(
		[]stakingtypes.UnbondingDelegation{ubd}, nil,
	)
	f.stakingKeeper.EXPECT().RemoveUnbondingDelegation(gomock.Any(), ubd).Return(nil)
	f.stakingKeeper.EXPECT().SetUnbondingDelegation(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, newUbd stakingtypes.UnbondingDelegation) error {
			require.Equal(t, newValAddr.String(), newUbd.ValidatorAddress)
			require.Equal(t, delegator.String(), newUbd.DelegatorAddress)
			return nil
		})
	f.stakingKeeper.EXPECT().InsertUBDQueue(gomock.Any(), gomock.Any(), completionTime).Return(nil)
	f.stakingKeeper.EXPECT().SetUnbondingDelegationByUnbondingID(gomock.Any(), gomock.Any(), uint64(77)).Return(nil)

	// Two redelegations with an UnbondingId: one where the migrated validator is
	// the source, and one where it is the destination.
	dstVal := sdk.ValAddress(testAccAddr())
	srcRed := stakingtypes.Redelegation{
		DelegatorAddress:    delegator.String(),
		ValidatorSrcAddress: oldValAddr.String(),
		ValidatorDstAddress: dstVal.String(),
		Entries: []stakingtypes.RedelegationEntry{
			{
				CreationHeight: 8,
				CompletionTime: completionTime,
				InitialBalance: math.NewInt(50),
				SharesDst:      math.LegacyNewDec(50),
				UnbondingId:    88,
			},
		},
	}
	srcVal := sdk.ValAddress(testAccAddr())
	dstRed := stakingtypes.Redelegation{
		DelegatorAddress:    delegator.String(),
		ValidatorSrcAddress: srcVal.String(),
		ValidatorDstAddress: oldValAddr.String(),
		Entries: []stakingtypes.RedelegationEntry{
			{
				CreationHeight: 9,
				CompletionTime: completionTime,
				InitialBalance: math.NewInt(75),
				SharesDst:      math.LegacyNewDec(75),
				UnbondingId:    89,
			},
		},
	}
	f.stakingKeeper.EXPECT().IterateRedelegations(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, fn func(int64, stakingtypes.Redelegation) bool) error {
			require.False(t, fn(0, srcRed))
			require.False(t, fn(1, dstRed))
			return nil
		},
	)
	f.stakingKeeper.EXPECT().RemoveRedelegation(gomock.Any(), srcRed).Return(nil)
	f.stakingKeeper.EXPECT().SetRedelegation(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, newRed stakingtypes.Redelegation) error {
			require.Equal(t, newValAddr.String(), newRed.ValidatorSrcAddress)
			require.Equal(t, dstVal.String(), newRed.ValidatorDstAddress)
			return nil
		},
	)
	f.stakingKeeper.EXPECT().InsertRedelegationQueue(gomock.Any(), gomock.Any(), completionTime).Return(nil)
	f.stakingKeeper.EXPECT().SetRedelegationByUnbondingID(gomock.Any(), gomock.Any(), uint64(88)).Return(nil)
	f.stakingKeeper.EXPECT().RemoveRedelegation(gomock.Any(), dstRed).Return(nil)
	f.stakingKeeper.EXPECT().SetRedelegation(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, newRed stakingtypes.Redelegation) error {
			require.Equal(t, srcVal.String(), newRed.ValidatorSrcAddress)
			require.Equal(t, newValAddr.String(), newRed.ValidatorDstAddress)
			return nil
		},
	)
	f.stakingKeeper.EXPECT().InsertRedelegationQueue(gomock.Any(), gomock.Any(), completionTime).Return(nil)
	f.stakingKeeper.EXPECT().SetRedelegationByUnbondingID(gomock.Any(), gomock.Any(), uint64(89)).Return(nil)

	err := f.keeper.MigrateValidatorDelegations(f.ctx, oldValAddr, newValAddr)
	require.NoError(t, err)
}

// --- Validator-supernode metrics tests ---

// TestMigrateValidatorSupernode_WithMetrics verifies that metrics state is
// re-keyed when the supernode has metrics.
func TestMigrateValidatorSupernode_WithMetrics(t *testing.T) {
	f := initMockFixture(t)
	oldValAddr := sdk.ValAddress(testAccAddr())
	newValAddr := sdk.ValAddress(testAccAddr())
	newAddr := sdk.AccAddress(newValAddr)

	sn := sntypes.SuperNode{
		ValidatorAddress: oldValAddr.String(),
		SupernodeAccount: sdk.AccAddress(oldValAddr).String(),
	}
	metrics := sntypes.SupernodeMetricsState{
		ValidatorAddress: oldValAddr.String(),
	}

	f.supernodeKeeper.EXPECT().QuerySuperNode(gomock.Any(), oldValAddr).Return(sn, true)
	f.supernodeKeeper.EXPECT().DeleteSuperNode(gomock.Any(), oldValAddr)
	f.supernodeKeeper.EXPECT().GetMetricsState(gomock.Any(), oldValAddr).Return(metrics, true)
	f.supernodeKeeper.EXPECT().SetMetricsState(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, updated sntypes.SupernodeMetricsState) error {
			require.Equal(t, newValAddr.String(), updated.ValidatorAddress)
			return nil
		})
	f.supernodeKeeper.EXPECT().DeleteMetricsState(gomock.Any(), oldValAddr)
	f.supernodeKeeper.EXPECT().SetSuperNode(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, updated sntypes.SuperNode) error {
			require.Equal(t, newAddr.String(), updated.SupernodeAccount)
			return nil
		})

	err := f.keeper.MigrateValidatorSupernode(f.ctx, oldValAddr, newValAddr, sdk.AccAddress(oldValAddr), newAddr)
	require.NoError(t, err)
}

// TestMigrateValidatorSupernode_MetricsWriteFails verifies that a failure
// writing metrics state propagates as an error.
func TestMigrateValidatorSupernode_MetricsWriteFails(t *testing.T) {
	f := initMockFixture(t)
	oldValAddr := sdk.ValAddress(testAccAddr())
	newValAddr := sdk.ValAddress(testAccAddr())
	newAddr := sdk.AccAddress(newValAddr)

	sn := sntypes.SuperNode{
		ValidatorAddress: oldValAddr.String(),
		SupernodeAccount: sdk.AccAddress(oldValAddr).String(),
	}
	metrics := sntypes.SupernodeMetricsState{
		ValidatorAddress: oldValAddr.String(),
	}

	f.supernodeKeeper.EXPECT().QuerySuperNode(gomock.Any(), oldValAddr).Return(sn, true)
	f.supernodeKeeper.EXPECT().DeleteSuperNode(gomock.Any(), oldValAddr)
	f.supernodeKeeper.EXPECT().GetMetricsState(gomock.Any(), oldValAddr).Return(metrics, true)
	f.supernodeKeeper.EXPECT().SetMetricsState(gomock.Any(), gomock.Any()).Return(
		fmt.Errorf("metrics store write failed"),
	)

	err := f.keeper.MigrateValidatorSupernode(f.ctx, oldValAddr, newValAddr, sdk.AccAddress(oldValAddr), newAddr)
	require.Error(t, err)
	require.Contains(t, err.Error(), "metrics store write failed")
}

// TestMigrateValidatorSupernode_NotFound verifies no-op when not a supernode.
func TestMigrateValidatorSupernode_NotFound(t *testing.T) {
	f := initMockFixture(t)
	oldValAddr := sdk.ValAddress(testAccAddr())
	newValAddr := sdk.ValAddress(testAccAddr())
	newAddr := sdk.AccAddress(newValAddr)

	f.supernodeKeeper.EXPECT().QuerySuperNode(gomock.Any(), oldValAddr).Return(sntypes.SuperNode{}, false)

	err := f.keeper.MigrateValidatorSupernode(f.ctx, oldValAddr, newValAddr, sdk.AccAddress(oldValAddr), newAddr)
	require.NoError(t, err)
}

// TestMigrateValidatorSupernode_EvidenceAddressMigrated verifies that
// Evidence.ValidatorAddress entries matching the old valoper are updated.
func TestMigrateValidatorSupernode_EvidenceAddressMigrated(t *testing.T) {
	f := initMockFixture(t)
	oldValAddr := sdk.ValAddress(testAccAddr())
	newValAddr := sdk.ValAddress(testAccAddr())
	newAddr := sdk.AccAddress(newValAddr)
	otherValAddr := sdk.ValAddress(testAccAddr()).String()

	sn := sntypes.SuperNode{
		ValidatorAddress: oldValAddr.String(),
		SupernodeAccount: sdk.AccAddress(oldValAddr).String(),
		Evidence: []*sntypes.Evidence{
			{ValidatorAddress: oldValAddr.String(), ReporterAddress: testAccAddr().String(), ActionId: "1"},
			{ValidatorAddress: otherValAddr, ReporterAddress: testAccAddr().String(), ActionId: "2"},
		},
	}

	f.supernodeKeeper.EXPECT().QuerySuperNode(gomock.Any(), oldValAddr).Return(sn, true)
	f.supernodeKeeper.EXPECT().DeleteSuperNode(gomock.Any(), oldValAddr)
	f.supernodeKeeper.EXPECT().GetMetricsState(gomock.Any(), oldValAddr).Return(sntypes.SupernodeMetricsState{}, false)
	f.supernodeKeeper.EXPECT().SetSuperNode(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, updated sntypes.SuperNode) error {
			require.Len(t, updated.Evidence, 2)
			// Evidence pointing to the migrated validator should be updated.
			require.Equal(t, newValAddr.String(), updated.Evidence[0].ValidatorAddress)
			// Evidence pointing to a different validator should be unchanged.
			require.Equal(t, otherValAddr, updated.Evidence[1].ValidatorAddress)
			return nil
		})

	err := f.keeper.MigrateValidatorSupernode(f.ctx, oldValAddr, newValAddr, sdk.AccAddress(oldValAddr), newAddr)
	require.NoError(t, err)
}

// TestMigrateValidatorSupernode_AccountHistoryMigrated verifies that
// PrevSupernodeAccounts entries matching the old account are updated.
func TestMigrateValidatorSupernode_AccountHistoryMigrated(t *testing.T) {
	f := initMockFixture(t)
	oldValAddr := sdk.ValAddress(testAccAddr())
	newValAddr := sdk.ValAddress(testAccAddr())
	newAddr := sdk.AccAddress(newValAddr)
	oldAccountStr := sdk.AccAddress(oldValAddr).String()
	otherAccount := testAccAddr().String()

	sn := sntypes.SuperNode{
		ValidatorAddress: oldValAddr.String(),
		SupernodeAccount: oldAccountStr,
		PrevSupernodeAccounts: []*sntypes.SupernodeAccountHistory{
			{Account: oldAccountStr, Height: 100},
			{Account: otherAccount, Height: 50},
		},
	}

	f.supernodeKeeper.EXPECT().QuerySuperNode(gomock.Any(), oldValAddr).Return(sn, true)
	f.supernodeKeeper.EXPECT().DeleteSuperNode(gomock.Any(), oldValAddr)
	f.supernodeKeeper.EXPECT().GetMetricsState(gomock.Any(), oldValAddr).Return(sntypes.SupernodeMetricsState{}, false)
	f.supernodeKeeper.EXPECT().SetSuperNode(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, updated sntypes.SuperNode) error {
			require.Len(t, updated.PrevSupernodeAccounts, 3)
			// Entry matching old account should be updated.
			require.Equal(t, newAddr.String(), updated.PrevSupernodeAccounts[0].Account)
			// Entry for a different account should be unchanged.
			require.Equal(t, otherAccount, updated.PrevSupernodeAccounts[1].Account)
			// New migration entry appended with new address and current height.
			require.Equal(t, newAddr.String(), updated.PrevSupernodeAccounts[2].Account)
			require.Equal(t, f.ctx.BlockHeight(), updated.PrevSupernodeAccounts[2].Height)
			return nil
		})

	err := f.keeper.MigrateValidatorSupernode(f.ctx, oldValAddr, newValAddr, sdk.AccAddress(oldValAddr), newAddr)
	require.NoError(t, err)
}

// TestMigrateValidatorSupernode_IndependentAccountPreserved verifies that when
// the supernode account is a different entity from the validator (already migrated
// independently or set to a separate EVM address), it is NOT overwritten with
// the validator's new address.
func TestMigrateValidatorSupernode_IndependentAccountPreserved(t *testing.T) {
	f := initMockFixture(t)
	oldValAddr := sdk.ValAddress(testAccAddr())
	newValAddr := sdk.ValAddress(testAccAddr())
	newAddr := sdk.AccAddress(newValAddr)
	// Supernode account is a separate address (e.g. already migrated to EVM key).
	independentSNAccount := testAccAddr().String()

	sn := sntypes.SuperNode{
		ValidatorAddress: oldValAddr.String(),
		SupernodeAccount: independentSNAccount,
		PrevSupernodeAccounts: []*sntypes.SupernodeAccountHistory{
			{Account: independentSNAccount, Height: 100},
		},
	}

	f.supernodeKeeper.EXPECT().QuerySuperNode(gomock.Any(), oldValAddr).Return(sn, true)
	f.supernodeKeeper.EXPECT().DeleteSuperNode(gomock.Any(), oldValAddr)
	f.supernodeKeeper.EXPECT().GetMetricsState(gomock.Any(), oldValAddr).Return(sntypes.SupernodeMetricsState{}, false)
	f.supernodeKeeper.EXPECT().SetSuperNode(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, updated sntypes.SuperNode) error {
			// Validator address should be re-keyed.
			require.Equal(t, sdk.ValAddress(newAddr).String(), updated.ValidatorAddress)
			// Supernode account should be preserved (not overwritten).
			require.Equal(t, independentSNAccount, updated.SupernodeAccount)
			// History should be unchanged — no entries added or modified since the
			// supernode account is independent from the validator.
			require.Len(t, updated.PrevSupernodeAccounts, 1)
			require.Equal(t, independentSNAccount, updated.PrevSupernodeAccounts[0].Account)
			return nil
		})

	err := f.keeper.MigrateValidatorSupernode(f.ctx, oldValAddr, newValAddr, sdk.AccAddress(oldValAddr), newAddr)
	require.NoError(t, err)
}

// --- FinalizeVestingAccount tests for all vesting types ---

// TestFinalizeVestingAccount_Delayed verifies that a DelayedVestingAccount
// is correctly recreated at the new address.
func TestFinalizeVestingAccount_Delayed(t *testing.T) {
	f := initMockFixture(t)
	newAddr := testAccAddr()
	baseAcc := authtypes.NewBaseAccountWithAddress(newAddr)

	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(baseAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), gomock.Any()).Do(func(_ any, acc sdk.AccountI) {
		_, ok := acc.(*vestingtypes.DelayedVestingAccount)
		require.True(t, ok, "should create a DelayedVestingAccount")
	})

	vi := &keeper.VestingInfo{
		Type:            keeper.VestingTypeDelayed,
		OriginalVesting: sdk.NewCoins(sdk.NewInt64Coin("ulume", 500)),
		EndTime:         2000000,
	}

	err := f.keeper.FinalizeVestingAccount(f.ctx, newAddr, vi)
	require.NoError(t, err)
}

// TestFinalizeVestingAccount_Periodic verifies that a PeriodicVestingAccount
// is correctly recreated with the original periods.
func TestFinalizeVestingAccount_Periodic(t *testing.T) {
	f := initMockFixture(t)
	newAddr := testAccAddr()
	baseAcc := authtypes.NewBaseAccountWithAddress(newAddr)

	periods := vestingtypes.Periods{
		{Length: 100000, Amount: sdk.NewCoins(sdk.NewInt64Coin("ulume", 500))},
		{Length: 200000, Amount: sdk.NewCoins(sdk.NewInt64Coin("ulume", 500))},
	}

	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(baseAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), gomock.Any()).Do(func(_ any, acc sdk.AccountI) {
		pva, ok := acc.(*vestingtypes.PeriodicVestingAccount)
		require.True(t, ok, "should create a PeriodicVestingAccount")
		require.Len(t, pva.VestingPeriods, 2)
	})

	vi := &keeper.VestingInfo{
		Type:            keeper.VestingTypePeriodic,
		OriginalVesting: sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000)),
		EndTime:         3000000,
		StartTime:       1000000,
		Periods:         periods,
	}

	err := f.keeper.FinalizeVestingAccount(f.ctx, newAddr, vi)
	require.NoError(t, err)
}

// TestFinalizeVestingAccount_PermanentLocked verifies that a PermanentLockedAccount
// is correctly recreated at the new address.
func TestFinalizeVestingAccount_PermanentLocked(t *testing.T) {
	f := initMockFixture(t)
	newAddr := testAccAddr()
	baseAcc := authtypes.NewBaseAccountWithAddress(newAddr)

	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(baseAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), gomock.Any()).Do(func(_ any, acc sdk.AccountI) {
		_, ok := acc.(*vestingtypes.PermanentLockedAccount)
		require.True(t, ok, "should create a PermanentLockedAccount")
	})

	vi := &keeper.VestingInfo{
		Type:            keeper.VestingTypePermanentLocked,
		OriginalVesting: sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000)),
	}

	err := f.keeper.FinalizeVestingAccount(f.ctx, newAddr, vi)
	require.NoError(t, err)
}

// TestFinalizeVestingAccount_NonBaseAccountFallback verifies that when the new
// account is not a *BaseAccount, a BaseAccount is extracted and used.
func TestFinalizeVestingAccount_NonBaseAccountFallback(t *testing.T) {
	f := initMockFixture(t)
	newAddr := testAccAddr()

	// Return a ContinuousVestingAccount (not a *BaseAccount) as the existing account.
	baseAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	origVesting := sdk.NewCoins(sdk.NewInt64Coin("ulume", 500))
	bva, err := vestingtypes.NewBaseVestingAccount(baseAcc, origVesting, 999999)
	require.NoError(t, err)
	existingCVA := vestingtypes.NewContinuousVestingAccountRaw(bva, 100000)

	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(existingCVA)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), gomock.Any()).Do(func(_ any, acc sdk.AccountI) {
		_, ok := acc.(*vestingtypes.DelayedVestingAccount)
		require.True(t, ok, "should create a DelayedVestingAccount even from non-base account")
	})

	vi := &keeper.VestingInfo{
		Type:            keeper.VestingTypeDelayed,
		OriginalVesting: sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000)),
		EndTime:         5000000,
	}

	err = f.keeper.FinalizeVestingAccount(f.ctx, newAddr, vi)
	require.NoError(t, err)
}

// --- Params endpoint validation tests ---

// TestQueryParams_NilRequest verifies that a nil request returns an error.
func TestQueryParams_NilRequest(t *testing.T) {
	f := initMockFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	resp, err := qs.Params(f.ctx, nil)
	require.Error(t, err)
	require.Nil(t, resp)
	require.Contains(t, err.Error(), "invalid request")
}

// TestQueryParams_Valid verifies that a valid request returns params.
func TestQueryParams_Valid(t *testing.T) {
	f := initMockFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	resp, err := qs.Params(f.ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, resp.Params.EnableMigration)
}

// TestUpdateParams_InvalidAuthority verifies that UpdateParams rejects
// requests from non-authority addresses.
func TestUpdateParams_InvalidAuthority(t *testing.T) {
	f := initMockFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	badAuthority := testAccAddr()
	req := &types.MsgUpdateParams{
		Authority: badAuthority.String(),
		Params:    types.DefaultParams(),
	}

	_, err := ms.UpdateParams(f.ctx, req)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidSigner)
}

// TestUpdateParams_ValidAuthority verifies that UpdateParams succeeds with
// the correct authority and valid params.
func TestUpdateParams_ValidAuthority(t *testing.T) {
	f := initMockFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	authority := authtypes.NewModuleAddress(types.GovModuleName)
	newParams := types.NewParams(false, 100, 25, 1000)
	req := &types.MsgUpdateParams{
		Authority: authority.String(),
		Params:    newParams,
	}

	resp, err := ms.UpdateParams(f.ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify params were updated.
	got, err := f.keeper.Params.Get(f.ctx)
	require.NoError(t, err)
	require.Equal(t, false, got.EnableMigration)
	require.Equal(t, int64(100), got.MigrationEndTime)
	require.Equal(t, uint64(25), got.MaxMigrationsPerBlock)
}
