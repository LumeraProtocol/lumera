package keeper

import (
	"bytes"
	"strings"
	"testing"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

func migrationValidators() (sdk.ValAddress, sdk.ValAddress, string) {
	source := sdk.ValAddress(bytes.Repeat([]byte{0x31}, 20))
	destination := sdk.ValAddress(bytes.Repeat([]byte{0x32}, 20))
	account := sdk.AccAddress(bytes.Repeat([]byte{0x41}, 20)).String()
	return source, destination, account
}

func seedMigrationSuperNode(t *testing.T, k Keeper, ctx sdk.Context, validator sdk.ValAddress, account string) types.SuperNode {
	t.Helper()
	sn := rawTestSuperNode(validator, account)
	store := migrationRawStore(k, ctx)
	store.Set(types.GetSupernodeKey(validator), marshalRawSuperNode(t, k, sn))
	store.Set(append(bytes.Clone(types.SuperNodeByAccountKey), []byte(account)...), validator)
	return sn
}

func migrationRawStore(k Keeper, ctx sdk.Context) storetypes.KVStore {
	return runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
}

func TestIdentityMigrationPlanMovesOnlyContinuityState(t *testing.T) {
	k, ctx := setupKeeperForInternalTest(t)
	source, destination, account := migrationValidators()
	sourceSN := seedMigrationSuperNode(t, k, ctx, source, account)
	metrics := types.SupernodeMetricsState{
		ValidatorAddress: source.String(),
		Metrics:          &types.SupernodeMetrics{CascadeKademliaDbBytes: 987654.5, PeersCount: 17},
		ReportCount:      23,
		Height:           456,
	}
	require.NoError(t, k.SetMetricsState(ctx, metrics))
	dist := SNDistState{SmoothedBytes: 123.5, PrevRawBytes: 234.5, EligibilityStartHeight: 42, PeriodsActive: 9}
	k.SetSNDistState(ctx, source.String(), dist)

	store := migrationRawStore(k, ctx)
	sourcePrimaryRaw := bytes.Clone(store.Get(types.GetSupernodeKey(source)))
	accountIndexKey := append(bytes.Clone(types.SuperNodeByAccountKey), []byte(account)...)
	accountIndexRaw := bytes.Clone(store.Get(accountIndexKey))
	payoutKey := append(types.PayoutHistoryPrefixForValidator(source.String()), []byte("00000000000000000456")...)
	payoutRaw := []byte{0xde, 0xad, 0xbe, 0xef}
	store.Set(payoutKey, payoutRaw)

	plan, err := k.BuildIdentityMigrationPlan(ctx, source, destination)
	require.NoError(t, err)
	require.NoError(t, k.ApplyIdentityMigrationPlan(ctx, plan))

	// Primary/account/history are owned by PR196 and are validation-only here.
	require.Equal(t, sourcePrimaryRaw, store.Get(types.GetSupernodeKey(source)))
	require.Nil(t, store.Get(types.GetSupernodeKey(destination)))
	require.Equal(t, accountIndexRaw, store.Get(accountIndexKey))
	require.Equal(t, sourceSN.ValidatorAddress, source.String())
	require.Equal(t, payoutRaw, store.Get(payoutKey))
	require.Nil(t, store.Get(append(types.PayoutHistoryPrefixForValidator(destination.String()), []byte("00000000000000000456")...)))

	require.Nil(t, store.Get(types.GetMetricsStateKey(source)))
	movedMetrics, found := k.GetMetricsState(ctx, destination)
	require.True(t, found)
	metrics.ValidatorAddress = destination.String()
	require.Equal(t, metrics, movedMetrics)
	require.Nil(t, store.Get(types.SNDistStateKey(source.String())))
	movedDist, found := k.GetSNDistState(ctx, destination.String())
	require.True(t, found)
	require.Equal(t, dist, movedDist)
	require.Equal(t, applyEMA(dist.SmoothedBytes, applyGrowthCap(300, dist.PrevRawBytes, 1250), 4),
		applyEMA(movedDist.SmoothedBytes, applyGrowthCap(300, movedDist.PrevRawBytes, 1250), 4))
	require.Equal(t, computeRampUpWeight(dist.PeriodsActive, 12), computeRampUpWeight(movedDist.PeriodsActive, 12))
}

func TestBuildIdentityMigrationPlanValidatesPrimaryAndIndexes(t *testing.T) {
	t.Run("destination primary", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)
		source, destination, account := migrationValidators()
		seedMigrationSuperNode(t, k, ctx, source, account)
		seedMigrationSuperNode(t, k, ctx, destination, sdk.AccAddress(bytes.Repeat([]byte{0x42}, 20)).String())
		_, err := k.BuildIdentityMigrationPlan(ctx, source, destination)
		require.ErrorContains(t, err, "destination supernode primary")
	})

	t.Run("missing source index", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)
		source, destination, account := migrationValidators()
		sn := rawTestSuperNode(source, account)
		migrationRawStore(k, ctx).Set(types.GetSupernodeKey(source), marshalRawSuperNode(t, k, sn))
		_, err := k.BuildIdentityMigrationPlan(ctx, source, destination)
		require.ErrorContains(t, err, "exactly one")
	})

	t.Run("destination index alias", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)
		source, destination, account := migrationValidators()
		seedMigrationSuperNode(t, k, ctx, source, account)
		alias := sdk.AccAddress(bytes.Repeat([]byte{0x43}, 20)).String()
		migrationRawStore(k, ctx).Set(append(bytes.Clone(types.SuperNodeByAccountKey), []byte(alias)...), destination)
		_, err := k.BuildIdentityMigrationPlan(ctx, source, destination)
		require.ErrorContains(t, err, "destination validator has stale")
	})

	t.Run("source index alias", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)
		source, destination, account := migrationValidators()
		seedMigrationSuperNode(t, k, ctx, source, account)
		migrationRawStore(k, ctx).Set(append(bytes.Clone(types.SuperNodeByAccountKey), []byte(strings.ToUpper(account))...), source)
		_, err := k.BuildIdentityMigrationPlan(ctx, source, destination)
		require.Error(t, err)
	})
}

func TestApplyIdentityMigrationPlanRejectsStaleRowsBeforeWriting(t *testing.T) {
	for _, tc := range []struct {
		name  string
		stale func(store storetypes.KVStore, source, destination sdk.ValAddress)
	}{
		{
			name: "source metrics",
			stale: func(store storetypes.KVStore, source, _ sdk.ValAddress) {
				store.Set(types.GetMetricsStateKey(source), []byte{0xff})
			},
		},
		{
			name: "destination metrics",
			stale: func(store storetypes.KVStore, _, destination sdk.ValAddress) {
				store.Set(types.GetMetricsStateKey(destination), []byte("late collision"))
			},
		},
		{
			name: "source rdist",
			stale: func(store storetypes.KVStore, source, _ sdk.ValAddress) {
				store.Set(types.SNDistStateKey(source.String()), []byte(`{"periods_active":99}`))
			},
		},
		{
			name: "destination rdist",
			stale: func(store storetypes.KVStore, _, destination sdk.ValAddress) {
				store.Set(types.SNDistStateKey(destination.String()), []byte(`{"periods_active":1}`))
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx := setupKeeperForInternalTest(t)
			source, destination, account := migrationValidators()
			seedMigrationSuperNode(t, k, ctx, source, account)
			require.NoError(t, k.SetMetricsState(ctx, types.SupernodeMetricsState{ValidatorAddress: source.String(), ReportCount: 7}))
			k.SetSNDistState(ctx, source.String(), SNDistState{PeriodsActive: 3})
			plan, err := k.BuildIdentityMigrationPlan(ctx, source, destination)
			require.NoError(t, err)
			store := migrationRawStore(k, ctx)
			tc.stale(store, source, destination)
			before := snapshotSuperNodeStore(t, k, ctx)

			err = k.ApplyIdentityMigrationPlan(ctx, plan)
			require.ErrorContains(t, err, "stale")
			require.Equal(t, before, snapshotSuperNodeStore(t, k, ctx), "failed Apply must perform no writes")
		})
	}
}

func TestApplyIdentityMigrationPlanTwiceFailsWithoutMutation(t *testing.T) {
	k, ctx := setupKeeperForInternalTest(t)
	source, destination, account := migrationValidators()
	seedMigrationSuperNode(t, k, ctx, source, account)
	require.NoError(t, k.SetMetricsState(ctx, types.SupernodeMetricsState{ValidatorAddress: source.String(), ReportCount: 4}))
	plan, err := k.BuildIdentityMigrationPlan(ctx, source, destination)
	require.NoError(t, err)
	require.NoError(t, k.ApplyIdentityMigrationPlan(ctx, plan))
	before := snapshotSuperNodeStore(t, k, ctx)
	require.ErrorContains(t, k.ApplyIdentityMigrationPlan(ctx, plan), "stale")
	require.Equal(t, before, snapshotSuperNodeStore(t, k, ctx))
}

func TestIdentityMigrationPlanIsOpaqueAndOwnsBuffers(t *testing.T) {
	k, ctx := setupKeeperForInternalTest(t)
	source, destination, _ := migrationValidators()
	sourceExpected := sdk.ValAddress(bytes.Clone(source))
	destinationExpected := sdk.ValAddress(bytes.Clone(destination))
	require.NoError(t, k.SetMetricsState(ctx, types.SupernodeMetricsState{ValidatorAddress: source.String(), ReportCount: 5}))
	plan, err := k.BuildIdentityMigrationPlan(ctx, source, destination)
	require.NoError(t, err)

	// Every accessor returns deep copies; mutation cannot change shared plan data.
	preconditions := plan.Preconditions()
	writes := plan.Writes()
	prefixes := plan.PrefixPreconditions()
	preconditions[0].Key[0] ^= 0xff
	writes[0].Key[0] ^= 0xff
	prefixes[0].Prefix[0] ^= 0xff
	for i := range source {
		source[i] = 0x71
		destination[i] = 0x72
	}

	require.NoError(t, k.ApplyIdentityMigrationPlan(ctx, plan))
	require.Nil(t, migrationRawStore(k, ctx).Get(types.GetMetricsStateKey(sourceExpected)))
	state, found := k.GetMetricsState(ctx, destinationExpected)
	require.True(t, found)
	require.Equal(t, uint64(5), state.ReportCount)
}

func TestBuildIdentityMigrationPlanCanonicalRDistScan(t *testing.T) {
	t.Run("alternate source spelling", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)
		source, destination, _ := migrationValidators()
		store := migrationRawStore(k, ctx)
		store.Set(append(bytes.Clone(types.SNDistStatePrefix), []byte(strings.ToUpper(source.String()))...), []byte(`{}`))
		_, err := k.BuildIdentityMigrationPlan(ctx, source, destination)
		require.ErrorContains(t, err, "non-canonical")
	})

	t.Run("duplicate source alternate", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)
		source, destination, _ := migrationValidators()
		store := migrationRawStore(k, ctx)
		store.Set(types.SNDistStateKey(source.String()), []byte(`{}`))
		store.Set(append(bytes.Clone(types.SNDistStatePrefix), []byte(strings.ToUpper(source.String()))...), []byte(`{}`))
		_, err := k.BuildIdentityMigrationPlan(ctx, source, destination)
		require.Error(t, err)
	})

	t.Run("malformed valoper", func(t *testing.T) {
		k, ctx := setupKeeperForInternalTest(t)
		source, destination, _ := migrationValidators()
		migrationRawStore(k, ctx).Set(append(bytes.Clone(types.SNDistStatePrefix), []byte("not-a-valoper")...), []byte(`{}`))
		_, err := k.BuildIdentityMigrationPlan(ctx, source, destination)
		require.ErrorContains(t, err, "malformed rdist")
	})
}

func TestIdentityMigrationRDistScanExactCapAndCapPlusOne(t *testing.T) {
	seedRows := func(t *testing.T, count int) (Keeper, sdk.Context, sdk.ValAddress, sdk.ValAddress) {
		t.Helper()
		k, ctx := setupKeeperForInternalTest(t)
		source, destination, _ := migrationValidators()
		store := migrationRawStore(k, ctx)
		for i := 0; i < count; i++ {
			address := make([]byte, 20)
			address[0] = byte(i >> 8)
			address[1] = byte(i)
			address[2] = 0x7f
			validator := sdk.ValAddress(address)
			store.Set(types.SNDistStateKey(validator.String()), []byte(`{}`))
		}
		return k, ctx, source, destination
	}

	t.Run("cap", func(t *testing.T) {
		k, ctx, source, destination := seedRows(t, IdentityMigrationRDistScanLimit)
		_, err := k.BuildIdentityMigrationPlan(ctx, source, destination)
		require.NoError(t, err)
	})
	t.Run("cap plus one", func(t *testing.T) {
		k, ctx, source, destination := seedRows(t, IdentityMigrationRDistScanLimit+1)
		_, err := k.BuildIdentityMigrationPlan(ctx, source, destination)
		require.ErrorContains(t, err, "exceeds scan limit")
	})
}

func TestIdentityMigrationPlanWithoutPrimaryAndInvalidRequests(t *testing.T) {
	k, ctx := setupKeeperForInternalTest(t)
	source, destination, _ := migrationValidators()
	require.NoError(t, k.SetMetricsState(ctx, types.SupernodeMetricsState{ValidatorAddress: source.String(), ReportCount: 1}))
	plan, err := k.BuildIdentityMigrationPlan(ctx, source, destination)
	require.NoError(t, err)
	require.NoError(t, k.ApplyIdentityMigrationPlan(ctx, plan))

	_, err = k.BuildIdentityMigrationPlan(ctx, nil, destination)
	require.ErrorContains(t, err, "non-empty")
	_, err = k.BuildIdentityMigrationPlan(ctx, source, nil)
	require.ErrorContains(t, err, "non-empty")
	_, err = k.BuildIdentityMigrationPlan(ctx, source, bytes.Clone(source))
	require.ErrorContains(t, err, "must differ")
	require.ErrorContains(t, k.ApplyIdentityMigrationPlan(ctx, nil), "nil")
}
