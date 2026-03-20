package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"

	"github.com/LumeraProtocol/lumera/x/evmigration/keeper"
	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// --- MigrationRecord query tests ---

// TestQueryMigrationRecord_Found verifies the query returns a stored migration record.
func TestQueryMigrationRecord_Found(t *testing.T) {
	f := initMockFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	legacyAddr := testAccAddr()
	record := types.MigrationRecord{
		LegacyAddress:   legacyAddr.String(),
		NewAddress:      testAccAddr().String(),
		MigrationTime:   100,
		MigrationHeight: 10,
	}
	require.NoError(t, f.keeper.MigrationRecords.Set(f.ctx, legacyAddr.String(), record))

	resp, err := qs.MigrationRecord(f.ctx, &types.QueryMigrationRecordRequest{
		LegacyAddress: legacyAddr.String(),
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Record)
	require.Equal(t, legacyAddr.String(), resp.Record.LegacyAddress)
}

// TestQueryMigrationRecord_NotFound verifies the query returns an empty response
// when the legacy address has no migration record.
func TestQueryMigrationRecord_NotFound(t *testing.T) {
	f := initMockFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	resp, err := qs.MigrationRecord(f.ctx, &types.QueryMigrationRecordRequest{
		LegacyAddress: testAccAddr().String(),
	})
	require.NoError(t, err)
	require.Nil(t, resp.Record)
}

// --- MigrationRecords query tests ---

// TestQueryMigrationRecords_Paginated verifies paginated listing of all migration records.
func TestQueryMigrationRecords_Paginated(t *testing.T) {
	f := initMockFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	// Store 3 records.
	for i := 0; i < 3; i++ {
		addr := testAccAddr()
		require.NoError(t, f.keeper.MigrationRecords.Set(f.ctx, addr.String(), types.MigrationRecord{
			LegacyAddress: addr.String(),
			NewAddress:    testAccAddr().String(),
		}))
	}

	resp, err := qs.MigrationRecords(f.ctx, &types.QueryMigrationRecordsRequest{
		Pagination: &query.PageRequest{Limit: 2},
	})
	require.NoError(t, err)
	require.Len(t, resp.Records, 2)
	require.NotNil(t, resp.Pagination)
	require.NotEmpty(t, resp.Pagination.NextKey)
}

// --- MigrationStats query tests ---

// TestQueryMigrationStats verifies that counters and computed stats are returned.
func TestQueryMigrationStats(t *testing.T) {
	f := initMockFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	// Set some counters.
	require.NoError(t, f.keeper.MigrationCounter.Set(f.ctx, 5))
	require.NoError(t, f.keeper.ValidatorMigrationCounter.Set(f.ctx, 2))

	// Mock IterateAccounts — no legacy accounts.
	f.accountKeeper.EXPECT().IterateAccounts(gomock.Any(), gomock.Any()).Times(2)
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), gomock.Any()).AnyTimes()

	resp, err := qs.MigrationStats(f.ctx, &types.QueryMigrationStatsRequest{})
	require.NoError(t, err)
	require.Equal(t, uint64(5), resp.TotalMigrated)
	require.Equal(t, uint64(2), resp.TotalValidatorsMigrated)
}

// --- MigrationEstimate query tests ---

// TestQueryMigrationEstimate_NonValidator verifies estimate for a non-validator address
// with delegations.
func TestQueryMigrationEstimate_NonValidator(t *testing.T) {
	f := initMockFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	addr := testAccAddr()
	valAddr := sdk.ValAddress(addr)

	// Not a validator.
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), valAddr).Return(
		stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound,
	)

	// Has 2 delegations.
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), addr, ^uint16(0)).Return(
		[]stakingtypes.Delegation{
			stakingtypes.NewDelegation(addr.String(), testAccAddr().String(), math.LegacyNewDec(100)),
			stakingtypes.NewDelegation(addr.String(), testAccAddr().String(), math.LegacyNewDec(200)),
		}, nil,
	)
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), addr, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), addr, ^uint16(0)).Return(nil, nil)

	// No authz or feegrant.
	f.authzKeeper.EXPECT().IterateGrants(gomock.Any(), gomock.Any())
	f.feegrantKeeper.EXPECT().IterateAllFeeAllowances(gomock.Any(), gomock.Any()).Return(nil)
	f.actionKeeper.EXPECT().IterateActions(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, cb func(*actiontypes.Action) bool) error {
			cb(&actiontypes.Action{ActionID: "1", Creator: addr.String()})
			cb(&actiontypes.Action{ActionID: "2", SuperNodes: []string{addr.String()}})
			cb(&actiontypes.Action{ActionID: "3", Creator: testAccAddr().String()})
			return nil
		},
	)

	resp, err := qs.MigrationEstimate(f.ctx, &types.QueryMigrationEstimateRequest{
		LegacyAddress: addr.String(),
	})
	require.NoError(t, err)
	require.False(t, resp.IsValidator)
	require.True(t, resp.WouldSucceed)
	require.Equal(t, uint64(2), resp.DelegationCount)
	require.Equal(t, uint64(2), resp.ActionCount)
	require.Equal(t, uint64(4), resp.TotalTouched)
}

// TestQueryMigrationEstimate_AlreadyMigrated verifies that already-migrated addresses
// are reported as would_succeed=false.
func TestQueryMigrationEstimate_AlreadyMigrated(t *testing.T) {
	f := initMockFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	addr := testAccAddr()
	valAddr := sdk.ValAddress(addr)

	// Store migration record.
	require.NoError(t, f.keeper.MigrationRecords.Set(f.ctx, addr.String(), types.MigrationRecord{
		LegacyAddress: addr.String(),
		NewAddress:    testAccAddr().String(),
	}))

	// Not a validator.
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), valAddr).Return(
		stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound,
	)
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), addr, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), addr, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), addr, ^uint16(0)).Return(nil, nil)
	f.authzKeeper.EXPECT().IterateGrants(gomock.Any(), gomock.Any())
	f.feegrantKeeper.EXPECT().IterateAllFeeAllowances(gomock.Any(), gomock.Any()).Return(nil)
	f.actionKeeper.EXPECT().IterateActions(gomock.Any(), gomock.Any()).Return(nil)

	resp, err := qs.MigrationEstimate(f.ctx, &types.QueryMigrationEstimateRequest{
		LegacyAddress: addr.String(),
	})
	require.NoError(t, err)
	require.False(t, resp.WouldSucceed)
	require.Equal(t, "already migrated", resp.RejectionReason)
}

// --- LegacyAccounts query tests ---

// TestQueryLegacyAccounts_WithSecp256k1 verifies that accounts with secp256k1
// public keys are listed as legacy accounts.
func TestQueryLegacyAccounts_WithSecp256k1(t *testing.T) {
	f := initMockFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	legacyPrivKey := secp256k1.GenPrivKey()
	legacyPubKey := legacyPrivKey.PubKey()
	legacyAddr := sdk.AccAddress(legacyPubKey.Address())

	legacyAcc := authtypes.NewBaseAccountWithAddress(legacyAddr)
	require.NoError(t, legacyAcc.SetPubKey(legacyPubKey))

	f.accountKeeper.EXPECT().IterateAccounts(gomock.Any(), gomock.Any()).
		Do(func(_ any, cb func(sdk.AccountI) bool) {
			cb(legacyAcc)
		})

	// Balance and delegation checks for the legacy account.
	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), legacyAddr).Return(
		sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000)),
	)
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacyAddr, uint16(1)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), sdk.ValAddress(legacyAddr)).Return(
		stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound,
	)

	resp, err := qs.LegacyAccounts(f.ctx, &types.QueryLegacyAccountsRequest{
		Pagination: &query.PageRequest{Limit: 10},
	})
	require.NoError(t, err)
	require.Len(t, resp.Accounts, 1)
	require.Equal(t, legacyAddr.String(), resp.Accounts[0].Address)
	require.Contains(t, resp.Accounts[0].BalanceSummary, "ulume")
	require.Equal(t, uint64(1), resp.Pagination.Total)
}

// TestQueryLegacyAccounts_Pagination verifies multi-page offset pagination:
// page 1 returns the first slice with NextKey, page 2 returns the rest without NextKey.
func TestQueryLegacyAccounts_Pagination(t *testing.T) {
	f := initMockFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	// Create 3 legacy accounts.
	var accs []sdk.AccountI
	var addrs []sdk.AccAddress
	for i := 0; i < 3; i++ {
		pk := secp256k1.GenPrivKey().PubKey()
		addr := sdk.AccAddress(pk.Address())
		acc := authtypes.NewBaseAccountWithAddress(addr)
		require.NoError(t, acc.SetPubKey(pk))
		accs = append(accs, acc)
		addrs = append(addrs, addr)
	}

	// Mock: iterate yields all 3 accounts (called twice — once per page request).
	f.accountKeeper.EXPECT().IterateAccounts(gomock.Any(), gomock.Any()).
		Do(func(_ any, cb func(sdk.AccountI) bool) {
			for _, a := range accs {
				cb(a)
			}
		}).Times(2)

	// Each account triggers balance, delegation, and validator checks (x2 calls).
	for _, addr := range addrs {
		f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), addr).Return(sdk.Coins{}).Times(2)
		f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), addr, uint16(1)).Return(nil, nil).Times(2)
		f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), sdk.ValAddress(addr)).Return(
			stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound,
		).Times(2)
	}

	// Page 1: limit=2, offset=0.
	resp, err := qs.LegacyAccounts(f.ctx, &types.QueryLegacyAccountsRequest{
		Pagination: &query.PageRequest{Limit: 2, Offset: 0},
	})
	require.NoError(t, err)
	require.Len(t, resp.Accounts, 2, "page 1 should have 2 accounts")
	require.Equal(t, uint64(3), resp.Pagination.Total)
	require.NotEmpty(t, resp.Pagination.NextKey, "should have NextKey when more pages exist")

	// Page 2: limit=2, offset=2.
	resp2, err := qs.LegacyAccounts(f.ctx, &types.QueryLegacyAccountsRequest{
		Pagination: &query.PageRequest{Limit: 2, Offset: 2},
	})
	require.NoError(t, err)
	require.Len(t, resp2.Accounts, 1, "page 2 should have remaining 1 account")
	require.Equal(t, uint64(3), resp2.Pagination.Total)
	require.Empty(t, resp2.Pagination.NextKey, "no NextKey on last page")
}

// TestQueryLegacyAccounts_Empty verifies empty response when no legacy accounts exist.
func TestQueryLegacyAccounts_Empty(t *testing.T) {
	f := initMockFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	// No accounts in the iteration.
	f.accountKeeper.EXPECT().IterateAccounts(gomock.Any(), gomock.Any())

	resp, err := qs.LegacyAccounts(f.ctx, &types.QueryLegacyAccountsRequest{
		Pagination: &query.PageRequest{Limit: 10},
	})
	require.NoError(t, err)
	require.Empty(t, resp.Accounts)
	require.Equal(t, uint64(0), resp.Pagination.Total)
	require.Empty(t, resp.Pagination.NextKey)
}

// TestQueryLegacyAccounts_OffsetBeyondTotal verifies that an offset beyond the
// total number of accounts returns an empty slice (not a panic).
func TestQueryLegacyAccounts_OffsetBeyondTotal(t *testing.T) {
	f := initMockFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	// 1 legacy account.
	pk := secp256k1.GenPrivKey().PubKey()
	addr := sdk.AccAddress(pk.Address())
	acc := authtypes.NewBaseAccountWithAddress(addr)
	require.NoError(t, acc.SetPubKey(pk))

	f.accountKeeper.EXPECT().IterateAccounts(gomock.Any(), gomock.Any()).
		Do(func(_ any, cb func(sdk.AccountI) bool) {
			cb(acc)
		})
	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), addr).Return(sdk.Coins{})
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), addr, uint16(1)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), sdk.ValAddress(addr)).Return(
		stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound,
	)

	resp, err := qs.LegacyAccounts(f.ctx, &types.QueryLegacyAccountsRequest{
		Pagination: &query.PageRequest{Limit: 10, Offset: 100},
	})
	require.NoError(t, err)
	require.Empty(t, resp.Accounts)
	require.Equal(t, uint64(1), resp.Pagination.Total)
}

// TestQueryLegacyAccounts_DefaultLimit verifies that omitting limit uses the
// default (100) and does not panic.
func TestQueryLegacyAccounts_DefaultLimit(t *testing.T) {
	f := initMockFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	pk := secp256k1.GenPrivKey().PubKey()
	addr := sdk.AccAddress(pk.Address())
	acc := authtypes.NewBaseAccountWithAddress(addr)
	require.NoError(t, acc.SetPubKey(pk))

	f.accountKeeper.EXPECT().IterateAccounts(gomock.Any(), gomock.Any()).
		Do(func(_ any, cb func(sdk.AccountI) bool) {
			cb(acc)
		})
	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), addr).Return(sdk.Coins{})
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), addr, uint16(1)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), sdk.ValAddress(addr)).Return(
		stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound,
	)

	// nil pagination → default limit of 100.
	resp, err := qs.LegacyAccounts(f.ctx, &types.QueryLegacyAccountsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Accounts, 1)
	require.Equal(t, uint64(1), resp.Pagination.Total)
}

// --- MigratedAccounts query tests ---

// TestQueryMigratedAccounts verifies paginated listing of migrated account records.
func TestQueryMigratedAccounts(t *testing.T) {
	f := initMockFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	// Store 2 records.
	for i := 0; i < 2; i++ {
		addr := testAccAddr()
		require.NoError(t, f.keeper.MigrationRecords.Set(f.ctx, addr.String(), types.MigrationRecord{
			LegacyAddress: addr.String(),
			NewAddress:    testAccAddr().String(),
		}))
	}

	resp, err := qs.MigratedAccounts(f.ctx, &types.QueryMigratedAccountsRequest{
		Pagination: &query.PageRequest{Limit: 10},
	})
	require.NoError(t, err)
	require.Len(t, resp.Records, 2)
}
