package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	kmultisig "github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"

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

func TestQueryMigrationRecordByNewAddress_Found(t *testing.T) {
	f := initMockFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	legacyAddr := testAccAddr()
	newAddr := testAccAddr()
	record := types.MigrationRecord{
		LegacyAddress:   legacyAddr.String(),
		NewAddress:      newAddr.String(),
		MigrationTime:   100,
		MigrationHeight: 10,
	}
	require.NoError(t, f.keeper.MigrationRecords.Set(f.ctx, legacyAddr.String(), record))
	require.NoError(t, f.keeper.MigrationRecordByNewAddress.Set(f.ctx, newAddr.String(), legacyAddr.String()))

	resp, err := qs.MigrationRecordByNewAddress(f.ctx, &types.QueryMigrationRecordByNewAddressRequest{
		NewAddress: newAddr.String(),
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Record)
	require.Equal(t, legacyAddr.String(), resp.Record.LegacyAddress)
	require.Equal(t, newAddr.String(), resp.Record.NewAddress)
}

func TestQueryMigrationRecordByNewAddress_NotFound(t *testing.T) {
	f := initMockFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	resp, err := qs.MigrationRecordByNewAddress(f.ctx, &types.QueryMigrationRecordByNewAddressRequest{
		NewAddress: testAccAddr().String(),
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

	// Account with secp256k1 pubkey and balance → counted as legacy.
	legacyPK := secp256k1.GenPrivKey().PubKey()
	legacyAddr := sdk.AccAddress(legacyPK.Address())
	legacyAcc := authtypes.NewBaseAccountWithAddress(legacyAddr)
	require.NoError(t, legacyAcc.SetPubKey(legacyPK))

	// Validator account with secp256k1 pubkey → counted as legacy validator.
	validatorPK := secp256k1.GenPrivKey().PubKey()
	validatorAddr := sdk.AccAddress(validatorPK.Address())
	validatorAcc := authtypes.NewBaseAccountWithAddress(validatorAddr)
	require.NoError(t, validatorAcc.SetPubKey(validatorPK))

	// Already-migrated account → excluded from legacy count.
	migratedPK := secp256k1.GenPrivKey().PubKey()
	migratedAddr := sdk.AccAddress(migratedPK.Address())
	migratedAcc := authtypes.NewBaseAccountWithAddress(migratedAddr)
	require.NoError(t, migratedAcc.SetPubKey(migratedPK))
	migratedNewAddr := testAccAddr()
	require.NoError(t, f.keeper.MigrationRecords.Set(f.ctx, migratedAddr.String(), types.MigrationRecord{
		LegacyAddress: migratedAddr.String(),
		NewAddress:    migratedNewAddr.String(),
	}))

	// Nil-pubkey account with balance → counted as legacy (funded but never signed).
	nilPKAddr := testAccAddr()
	nilPKAcc := authtypes.NewBaseAccountWithAddress(nilPKAddr)

	// Migration destination account (nil pubkey, has balance) → excluded.
	migDestAddr := migratedNewAddr
	migDestAcc := authtypes.NewBaseAccountWithAddress(migDestAddr)
	require.NoError(t, f.keeper.MigrationRecordByNewAddress.Set(f.ctx, migDestAddr.String(), migratedAddr.String()))

	f.accountKeeper.EXPECT().IterateAccounts(gomock.Any(), gomock.Any()).
		Do(func(_ any, cb func(sdk.AccountI) bool) {
			cb(legacyAcc)
			cb(validatorAcc)
			cb(migratedAcc)
			cb(nilPKAcc)
			cb(migDestAcc)
		})

	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), legacyAddr).Return(
		sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000)),
	)
	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), validatorAddr).Return(sdk.Coins{})
	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), nilPKAddr).Return(
		sdk.NewCoins(sdk.NewInt64Coin("ulume", 500)),
	)
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacyAddr, uint16(1)).Return(
		[]stakingtypes.Delegation{
			stakingtypes.NewDelegation(legacyAddr.String(), testAccAddr().String(), math.LegacyNewDec(100)),
		}, nil,
	)
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), validatorAddr, uint16(1)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), nilPKAddr, uint16(1)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), sdk.ValAddress(legacyAddr)).Return(
		stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound,
	)
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), sdk.ValAddress(validatorAddr)).Return(
		stakingtypes.Validator{OperatorAddress: sdk.ValAddress(validatorAddr).String()}, nil,
	)
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), sdk.ValAddress(nilPKAddr)).Return(
		stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound,
	)

	resp, err := qs.MigrationStats(f.ctx, &types.QueryMigrationStatsRequest{})
	require.NoError(t, err)
	require.Equal(t, uint64(5), resp.TotalMigrated)
	require.Equal(t, uint64(2), resp.TotalValidatorsMigrated)
	// 3 legacy: legacyAcc + validatorAcc + nilPKAcc (migrated excluded, migDest excluded).
	require.Equal(t, uint64(3), resp.TotalLegacy)
	require.Equal(t, uint64(1), resp.TotalLegacyStaked)
	require.Equal(t, uint64(1), resp.TotalValidatorsLegacy)

	require.Equal(t, uint64(2), resp.TotalLegacyWithPubkey)
	require.Equal(t, uint64(1), resp.TotalLegacyWithoutPubkey)
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

	// Has balance.
	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), addr).Return(
		sdk.NewCoins(sdk.NewCoin("ulume", math.NewInt(5000000000))),
	)
	// No supernode.
	f.supernodeKeeper.EXPECT().QuerySuperNode(gomock.Any(), sdk.ValAddress(addr)).Return(
		sntypes.SuperNode{}, false,
	)
	// No account stored → multisig preflight skipped.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), addr).Return(nil)

	resp, err := qs.MigrationEstimate(f.ctx, &types.QueryMigrationEstimateRequest{
		LegacyAddress: addr.String(),
	})
	require.NoError(t, err)
	require.False(t, resp.IsValidator)
	require.True(t, resp.WouldSucceed)
	require.Equal(t, uint64(2), resp.DelegationCount)
	require.Equal(t, uint64(2), resp.ActionCount)
	require.Equal(t, uint64(4), resp.TotalTouched)
	require.Equal(t, "5000000000ulume", resp.BalanceSummary)
	require.False(t, resp.HasSupernode)
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
	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), addr).Return(sdk.Coins{})
	f.supernodeKeeper.EXPECT().QuerySuperNode(gomock.Any(), sdk.ValAddress(addr)).Return(
		sntypes.SuperNode{}, false,
	)
	// No account stored → multisig preflight skipped.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), addr).Return(nil)

	resp, err := qs.MigrationEstimate(f.ctx, &types.QueryMigrationEstimateRequest{
		LegacyAddress: addr.String(),
	})
	require.NoError(t, err)
	require.False(t, resp.WouldSucceed)
	require.Equal(t, "already migrated", resp.RejectionReason)
	require.Empty(t, resp.BalanceSummary)
	require.False(t, resp.HasSupernode)
}

func TestQueryMigrationEstimate_ValidatorUsesScopedRedelegationIndexesForLimit(t *testing.T) {
	f := initMockFixture(t)
	f.wireScopedMigrationStores()
	qs := keeper.NewQueryServerImpl(f.keeper)

	params := types.NewParams(true, 0, 50, 2, 20)
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	addr := testAccAddr()
	valAddr := sdk.ValAddress(addr)
	completionTime := f.ctx.BlockTime().Add(21 * 24 * 3600 * 1e9)
	delegator := testAccAddr()

	f.writeRedelegation(stakingtypes.Redelegation{
		DelegatorAddress:    delegator.String(),
		ValidatorSrcAddress: valAddr.String(),
		ValidatorDstAddress: sdk.ValAddress(testAccAddr()).String(),
		Entries: []stakingtypes.RedelegationEntry{{
			CreationHeight: 10,
			CompletionTime: completionTime,
			InitialBalance: math.NewInt(100),
			SharesDst:      math.LegacyNewDec(100),
			UnbondingId:    301,
		}},
	})
	f.writeRedelegation(stakingtypes.Redelegation{
		DelegatorAddress:    delegator.String(),
		ValidatorSrcAddress: sdk.ValAddress(testAccAddr()).String(),
		ValidatorDstAddress: valAddr.String(),
		Entries: []stakingtypes.RedelegationEntry{{
			CreationHeight: 11,
			CompletionTime: completionTime,
			InitialBalance: math.NewInt(200),
			SharesDst:      math.LegacyNewDec(200),
			UnbondingId:    302,
		}},
	})
	f.writeRedelegation(stakingtypes.Redelegation{
		DelegatorAddress:    testAccAddr().String(),
		ValidatorSrcAddress: sdk.ValAddress(testAccAddr()).String(),
		ValidatorDstAddress: sdk.ValAddress(testAccAddr()).String(),
		Entries: []stakingtypes.RedelegationEntry{{
			CreationHeight: 12,
			CompletionTime: completionTime,
			InitialBalance: math.NewInt(300),
			SharesDst:      math.LegacyNewDec(300),
			UnbondingId:    303,
		}},
	})

	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), valAddr).Return(
		stakingtypes.Validator{OperatorAddress: valAddr.String(), Status: stakingtypes.Bonded}, nil,
	)
	f.stakingKeeper.EXPECT().GetValidatorDelegations(gomock.Any(), valAddr).Return(
		[]stakingtypes.Delegation{
			stakingtypes.NewDelegation(delegator.String(), valAddr.String(), math.LegacyNewDec(50)),
		}, nil,
	)
	f.stakingKeeper.EXPECT().GetUnbondingDelegationsFromValidator(gomock.Any(), valAddr).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), addr, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), addr, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), addr, ^uint16(0)).Return(nil, nil)
	f.authzKeeper.EXPECT().IterateGrants(gomock.Any(), gomock.Any())
	f.feegrantKeeper.EXPECT().IterateAllFeeAllowances(gomock.Any(), gomock.Any()).Return(nil)
	f.actionKeeper.EXPECT().IterateActions(gomock.Any(), gomock.Any()).Return(nil)
	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), addr).Return(sdk.Coins{})
	f.supernodeKeeper.EXPECT().QuerySuperNode(gomock.Any(), valAddr).Return(sntypes.SuperNode{}, false)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), addr).Return(nil)

	resp, err := qs.MigrationEstimate(f.ctx, &types.QueryMigrationEstimateRequest{
		LegacyAddress: addr.String(),
	})
	require.NoError(t, err)
	require.True(t, resp.IsValidator)
	require.Equal(t, uint64(1), resp.ValDelegationCount)
	require.Equal(t, uint64(2), resp.ValRedelegationCount)
	require.Equal(t, uint64(3), resp.TotalTouched)
	require.False(t, resp.WouldSucceed)
	require.Equal(t, "too many delegators", resp.RejectionReason)
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
		f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), addr).Return(
			sdk.NewCoins(sdk.NewInt64Coin("ulume", 1)),
		).Times(2)
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
	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), addr).Return(
		sdk.NewCoins(sdk.NewInt64Coin("ulume", 1)),
	)
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
	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), addr).Return(
		sdk.NewCoins(sdk.NewInt64Coin("ulume", 1)),
	)
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

// TestQueryLegacyAccounts_NilPubkeyIncluded verifies that accounts with nil pubkey
// but non-zero balance are counted as legacy (funded but never signed).
func TestQueryLegacyAccounts_NilPubkeyIncluded(t *testing.T) {
	f := initMockFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	nilPKAddr := testAccAddr()
	nilPKAcc := authtypes.NewBaseAccountWithAddress(nilPKAddr)

	f.accountKeeper.EXPECT().IterateAccounts(gomock.Any(), gomock.Any()).
		Do(func(_ any, cb func(sdk.AccountI) bool) {
			cb(nilPKAcc)
		})
	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), nilPKAddr).Return(
		sdk.NewCoins(sdk.NewInt64Coin("ulume", 5000)),
	)
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), nilPKAddr, uint16(1)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), sdk.ValAddress(nilPKAddr)).Return(
		stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound,
	)

	resp, err := qs.LegacyAccounts(f.ctx, &types.QueryLegacyAccountsRequest{
		Pagination: &query.PageRequest{Limit: 10},
	})
	require.NoError(t, err)
	require.Len(t, resp.Accounts, 1)
	require.Equal(t, nilPKAddr.String(), resp.Accounts[0].Address)
}

// TestQueryLegacyAccounts_MigrationDestExcluded verifies that migration destination
// accounts (nil pubkey, non-zero balance) are NOT counted as legacy.
func TestQueryLegacyAccounts_MigrationDestExcluded(t *testing.T) {
	f := initMockFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	// Migration destination with balance → should be excluded.
	destAddr := testAccAddr()
	destAcc := authtypes.NewBaseAccountWithAddress(destAddr)
	legacyAddr := testAccAddr()
	require.NoError(t, f.keeper.MigrationRecordByNewAddress.Set(f.ctx, destAddr.String(), legacyAddr.String()))

	f.accountKeeper.EXPECT().IterateAccounts(gomock.Any(), gomock.Any()).
		Do(func(_ any, cb func(sdk.AccountI) bool) {
			cb(destAcc)
		})

	resp, err := qs.LegacyAccounts(f.ctx, &types.QueryLegacyAccountsRequest{
		Pagination: &query.PageRequest{Limit: 10},
	})
	require.NoError(t, err)
	require.Empty(t, resp.Accounts)
	require.Equal(t, uint64(0), resp.Pagination.Total)
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

// --- Multisig query tests (Tasks 12, 13, 14) ---

// TestLegacyAccounts_Multisig verifies that a multisig account (flat secp256k1
// sub-keys) appears in the legacy list with correct is_multisig/threshold/num_signers.
func TestLegacyAccounts_Multisig(t *testing.T) {
	f := initMockFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	pubs := make([]cryptotypes.PubKey, 3)
	for i := 0; i < 3; i++ {
		pubs[i] = secp256k1.GenPrivKey().PubKey()
	}
	multiPK := kmultisig.NewLegacyAminoPubKey(2, pubs)
	addr := sdk.AccAddress(multiPK.Address())
	acc := authtypes.NewBaseAccountWithAddress(addr)
	require.NoError(t, acc.SetPubKey(multiPK))

	f.accountKeeper.EXPECT().IterateAccounts(gomock.Any(), gomock.Any()).
		Do(func(_ any, cb func(sdk.AccountI) bool) {
			cb(acc)
		})

	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), addr).Return(
		sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000)),
	)
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), addr, uint16(1)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), sdk.ValAddress(addr)).Return(
		stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound,
	)

	resp, err := qs.LegacyAccounts(f.ctx, &types.QueryLegacyAccountsRequest{
		Pagination: &query.PageRequest{Limit: 10},
	})
	require.NoError(t, err)
	require.Len(t, resp.Accounts, 1)
	found := resp.Accounts[0]
	require.Equal(t, addr.String(), found.Address)
	require.True(t, found.IsMultisig)
	require.Equal(t, uint32(2), found.Threshold)
	require.Equal(t, uint32(3), found.NumSigners)
}

// TestMigrationEstimate_Multisig_Supported verifies that a supported 2-of-3
// secp256k1 multisig returns WouldSucceed=true with correct metadata.
func TestMigrationEstimate_Multisig_Supported(t *testing.T) {
	f := initMockFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	pubs := make([]cryptotypes.PubKey, 3)
	for i := 0; i < 3; i++ {
		pubs[i] = secp256k1.GenPrivKey().PubKey()
	}
	multiPK := kmultisig.NewLegacyAminoPubKey(2, pubs)
	addr := sdk.AccAddress(multiPK.Address())
	acc := authtypes.NewBaseAccountWithAddress(addr)
	require.NoError(t, acc.SetPubKey(multiPK))

	valAddr := sdk.ValAddress(addr)
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), valAddr).Return(
		stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound,
	)
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), addr, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), addr, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), addr, ^uint16(0)).Return(nil, nil)
	f.authzKeeper.EXPECT().IterateGrants(gomock.Any(), gomock.Any())
	f.feegrantKeeper.EXPECT().IterateAllFeeAllowances(gomock.Any(), gomock.Any()).Return(nil)
	f.actionKeeper.EXPECT().IterateActions(gomock.Any(), gomock.Any()).Return(nil)
	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), addr).Return(sdk.Coins{})
	f.supernodeKeeper.EXPECT().QuerySuperNode(gomock.Any(), valAddr).Return(sntypes.SuperNode{}, false)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), addr).Return(acc)

	resp, err := qs.MigrationEstimate(f.ctx, &types.QueryMigrationEstimateRequest{
		LegacyAddress: addr.String(),
	})
	require.NoError(t, err)
	require.True(t, resp.IsMultisig)
	require.Equal(t, uint32(2), resp.Threshold)
	require.Equal(t, uint32(3), resp.NumSigners)
	require.True(t, resp.WouldSucceed)
}

// TestMigrationEstimate_Multisig_TooManySubKeys verifies that a multisig with
// N=21 sub-keys (> default cap 20) returns WouldSucceed=false.
func TestMigrationEstimate_Multisig_TooManySubKeys(t *testing.T) {
	f := initMockFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	pubs := make([]cryptotypes.PubKey, 21)
	for i := 0; i < 21; i++ {
		pubs[i] = secp256k1.GenPrivKey().PubKey()
	}
	multiPK := kmultisig.NewLegacyAminoPubKey(1, pubs)
	addr := sdk.AccAddress(multiPK.Address())
	acc := authtypes.NewBaseAccountWithAddress(addr)
	require.NoError(t, acc.SetPubKey(multiPK))

	valAddr := sdk.ValAddress(addr)
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), valAddr).Return(
		stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound,
	)
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), addr, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), addr, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), addr, ^uint16(0)).Return(nil, nil)
	f.authzKeeper.EXPECT().IterateGrants(gomock.Any(), gomock.Any())
	f.feegrantKeeper.EXPECT().IterateAllFeeAllowances(gomock.Any(), gomock.Any()).Return(nil)
	f.actionKeeper.EXPECT().IterateActions(gomock.Any(), gomock.Any()).Return(nil)
	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), addr).Return(sdk.Coins{})
	f.supernodeKeeper.EXPECT().QuerySuperNode(gomock.Any(), valAddr).Return(sntypes.SuperNode{}, false)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), addr).Return(acc)

	resp, err := qs.MigrationEstimate(f.ctx, &types.QueryMigrationEstimateRequest{
		LegacyAddress: addr.String(),
	})
	require.NoError(t, err)
	require.True(t, resp.IsMultisig)
	require.False(t, resp.WouldSucceed)
	require.Contains(t, resp.RejectionReason, "max is 20")
}

// TestMigrationEstimate_Multisig_NonSecp256k1SubKey verifies that a multisig
// containing an ed25519 sub-key returns WouldSucceed=false with "non-secp256k1".
func TestMigrationEstimate_Multisig_NonSecp256k1SubKey(t *testing.T) {
	f := initMockFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	sec := secp256k1.GenPrivKey().PubKey()
	ed := ed25519.GenPrivKey().PubKey()
	multiPK := kmultisig.NewLegacyAminoPubKey(1, []cryptotypes.PubKey{sec, ed})
	addr := sdk.AccAddress(multiPK.Address())
	acc := authtypes.NewBaseAccountWithAddress(addr)
	require.NoError(t, acc.SetPubKey(multiPK))

	valAddr := sdk.ValAddress(addr)
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), valAddr).Return(
		stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound,
	)
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), addr, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), addr, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), addr, ^uint16(0)).Return(nil, nil)
	f.authzKeeper.EXPECT().IterateGrants(gomock.Any(), gomock.Any())
	f.feegrantKeeper.EXPECT().IterateAllFeeAllowances(gomock.Any(), gomock.Any()).Return(nil)
	f.actionKeeper.EXPECT().IterateActions(gomock.Any(), gomock.Any()).Return(nil)
	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), addr).Return(sdk.Coins{})
	f.supernodeKeeper.EXPECT().QuerySuperNode(gomock.Any(), valAddr).Return(sntypes.SuperNode{}, false)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), addr).Return(acc)

	resp, err := qs.MigrationEstimate(f.ctx, &types.QueryMigrationEstimateRequest{
		LegacyAddress: addr.String(),
	})
	require.NoError(t, err)
	require.True(t, resp.IsMultisig)
	require.False(t, resp.WouldSucceed)
	require.Contains(t, resp.RejectionReason, "non-secp256k1")
}

// TestMigrationEstimate_Multisig_DuplicateSubKey verifies that a legacy
// multisig with a duplicated sub-key (SDK construction permits this, unlike
// evmigration's MultisigProof.validateBasic which rejects it at consensus)
// is flagged at preflight with WouldSucceed=false. Without this check,
// co-signers would run a full K-of-N ceremony only to have submit-proof
// fail with ErrInvalidMigrationPubKey.
func TestMigrationEstimate_Multisig_DuplicateSubKey(t *testing.T) {
	f := initMockFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	shared := secp256k1.GenPrivKey().PubKey()
	distinct := secp256k1.GenPrivKey().PubKey()
	// Positions 0 and 2 carry the same sub-key; position 1 is distinct.
	multiPK := kmultisig.NewLegacyAminoPubKey(2, []cryptotypes.PubKey{shared, distinct, shared})
	addr := sdk.AccAddress(multiPK.Address())
	acc := authtypes.NewBaseAccountWithAddress(addr)
	require.NoError(t, acc.SetPubKey(multiPK))

	valAddr := sdk.ValAddress(addr)
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), valAddr).Return(
		stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound,
	)
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), addr, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), addr, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), addr, ^uint16(0)).Return(nil, nil)
	f.authzKeeper.EXPECT().IterateGrants(gomock.Any(), gomock.Any())
	f.feegrantKeeper.EXPECT().IterateAllFeeAllowances(gomock.Any(), gomock.Any()).Return(nil)
	f.actionKeeper.EXPECT().IterateActions(gomock.Any(), gomock.Any()).Return(nil)
	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), addr).Return(sdk.Coins{})
	f.supernodeKeeper.EXPECT().QuerySuperNode(gomock.Any(), valAddr).Return(sntypes.SuperNode{}, false)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), addr).Return(acc)

	resp, err := qs.MigrationEstimate(f.ctx, &types.QueryMigrationEstimateRequest{
		LegacyAddress: addr.String(),
	})
	require.NoError(t, err)
	require.True(t, resp.IsMultisig)
	require.False(t, resp.WouldSucceed)
	require.Contains(t, resp.RejectionReason, "duplicates sub_pub_keys[0]")
}
