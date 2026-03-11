package keeper

import (
	"context"
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
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"

	"github.com/LumeraProtocol/lumera/x/everlight/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// --- Mock implementations ---

type mockBankKeeper struct {
	balances map[string]sdk.Coins // addr -> coins
	sent     []sendRecord
}

type sendRecord struct {
	from   string
	to     string
	amount sdk.Coins
}

func newMockBankKeeper() *mockBankKeeper {
	return &mockBankKeeper{
		balances: make(map[string]sdk.Coins),
	}
}

func (m *mockBankKeeper) GetBalance(_ context.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	coins := m.balances[addr.String()]
	return sdk.NewCoin(denom, coins.AmountOf(denom))
}

func (m *mockBankKeeper) GetAllBalances(_ context.Context, addr sdk.AccAddress) sdk.Coins {
	return m.balances[addr.String()]
}

func (m *mockBankKeeper) SendCoins(_ context.Context, from, to sdk.AccAddress, amt sdk.Coins) error {
	m.sent = append(m.sent, sendRecord{from: from.String(), to: to.String(), amount: amt})
	return nil
}

func (m *mockBankKeeper) SendCoinsFromModuleToAccount(_ context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	moduleAddr := authtypes.NewModuleAddress(senderModule)
	m.sent = append(m.sent, sendRecord{from: moduleAddr.String(), to: recipientAddr.String(), amount: amt})
	// Deduct from module balance.
	m.balances[moduleAddr.String()] = m.balances[moduleAddr.String()].Sub(amt...)
	return nil
}

func (m *mockBankKeeper) SendCoinsFromAccountToModule(_ context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
	return nil
}

func (m *mockBankKeeper) SendCoinsFromModuleToModule(_ context.Context, senderModule, recipientModule string, amt sdk.Coins) error {
	return nil
}

type mockAccountKeeper struct{}

func (m *mockAccountKeeper) GetModuleAddress(moduleName string) sdk.AccAddress {
	return authtypes.NewModuleAddress(moduleName)
}

func (m *mockAccountKeeper) GetModuleAccount(_ context.Context, moduleName string) sdk.ModuleAccountI {
	return nil
}

type mockSupernodeKeeper struct {
	supernodes []sntypes.SuperNode
	metrics    map[string]sntypes.SupernodeMetricsState
}

func newMockSupernodeKeeper() *mockSupernodeKeeper {
	return &mockSupernodeKeeper{
		metrics: make(map[string]sntypes.SupernodeMetricsState),
	}
}

func (m *mockSupernodeKeeper) GetAllSuperNodes(_ sdk.Context, stateFilters ...sntypes.SuperNodeState) ([]sntypes.SuperNode, error) {
	if len(stateFilters) == 0 {
		return m.supernodes, nil
	}
	filterSet := make(map[sntypes.SuperNodeState]bool)
	for _, f := range stateFilters {
		filterSet[f] = true
	}
	var result []sntypes.SuperNode
	for _, sn := range m.supernodes {
		if len(sn.States) > 0 {
			lastState := sn.States[len(sn.States)-1]
			if filterSet[lastState.State] {
				result = append(result, sn)
			}
		}
	}
	return result, nil
}

func (m *mockSupernodeKeeper) GetMetricsState(_ sdk.Context, valAddr sdk.ValAddress) (sntypes.SupernodeMetricsState, bool) {
	state, ok := m.metrics[valAddr.String()]
	return state, ok
}

// --- Test helpers ---

func setupTestKeeper(t *testing.T) (Keeper, sdk.Context, *mockBankKeeper, *mockSupernodeKeeper) {
	t.Helper()

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	authority := authtypes.NewModuleAddress(govtypes.ModuleName)

	bankKeeper := newMockBankKeeper()
	accountKeeper := &mockAccountKeeper{}
	snKeeper := newMockSupernodeKeeper()

	k := NewKeeper(
		cdc,
		runtime.NewKVStoreService(storeKey),
		log.NewNopLogger(),
		authority.String(),
		bankKeeper,
		accountKeeper,
		snKeeper,
	)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 1}, false, log.NewNopLogger())

	// Initialize params.
	require.NoError(t, k.SetParams(ctx, types.DefaultParams()))

	return k, ctx, bankKeeper, snKeeper
}

// makeValAddr creates a deterministic validator address for testing.
func makeValAddr(i int) sdk.ValAddress {
	addr := make([]byte, 20)
	addr[0] = byte(i)
	addr[19] = byte(i)
	return sdk.ValAddress(addr)
}

// makeAccAddr creates a deterministic account address for testing.
func makeAccAddr(i int) sdk.AccAddress {
	addr := make([]byte, 20)
	addr[0] = byte(i + 100)
	addr[19] = byte(i + 100)
	return sdk.AccAddress(addr)
}

func addSupernode(snKeeper *mockSupernodeKeeper, valAddr sdk.ValAddress, accAddr sdk.AccAddress, state sntypes.SuperNodeState, cascadeBytes float64) {
	sn := sntypes.SuperNode{
		ValidatorAddress: valAddr.String(),
		SupernodeAccount: accAddr.String(),
		States: []*sntypes.SuperNodeStateRecord{
			{State: state},
		},
	}
	snKeeper.supernodes = append(snKeeper.supernodes, sn)
	snKeeper.metrics[valAddr.String()] = sntypes.SupernodeMetricsState{
		ValidatorAddress: valAddr.String(),
		Metrics: &sntypes.SupernodeMetrics{
			CascadeKademliaDbBytes: cascadeBytes,
		},
	}
}

func fundPool(bankKeeper *mockBankKeeper, amount int64) {
	moduleAddr := authtypes.NewModuleAddress(types.ModuleName)
	bankKeeper.balances[moduleAddr.String()] = sdk.NewCoins(sdk.NewCoin("ulume", sdkmath.NewInt(amount)))
}

// --- Tests ---

// AT35: Pool distributes proportionally by cascade_kademlia_db_bytes at period boundary.
func TestDistributePoolProportionally(t *testing.T) {
	k, ctx, bankKeeper, snKeeper := setupTestKeeper(t)

	// Set params with a small period for testing.
	params := types.DefaultParams()
	params.PaymentPeriodBlocks = 10
	params.MinCascadeBytesForPayment = 1000
	params.NewSnRampUpPeriods = 0 // No ramp-up for this test.
	params.MeasurementSmoothingPeriods = 0
	params.UsageGrowthCapBpsPerPeriod = 10000 // 100% cap (effectively no cap).
	require.NoError(t, k.SetParams(ctx, params))

	// Create two supernodes: SN1 with 3000 bytes, SN2 with 7000 bytes.
	val1 := makeValAddr(1)
	acc1 := makeAccAddr(1)
	val2 := makeValAddr(2)
	acc2 := makeAccAddr(2)

	addSupernode(snKeeper, val1, acc1, sntypes.SuperNodeStateActive, 3000)
	addSupernode(snKeeper, val2, acc2, sntypes.SuperNodeStateActive, 7000)

	// Fund pool with 10000 ulume.
	fundPool(bankKeeper, 10000)

	// Set context height to trigger distribution.
	ctx = ctx.WithBlockHeight(100)
	k.SetLastDistributionHeight(ctx, 80)

	err := k.distributePool(ctx)
	require.NoError(t, err)

	// Verify proportional distribution: SN1 gets 30%, SN2 gets 70%.
	require.GreaterOrEqual(t, len(bankKeeper.sent), 2)

	// Find payouts.
	var payout1, payout2 sdkmath.Int
	for _, s := range bankKeeper.sent {
		if s.to == acc1.String() {
			payout1 = s.amount.AmountOf("ulume")
		}
		if s.to == acc2.String() {
			payout2 = s.amount.AmountOf("ulume")
		}
	}

	// SN1: 30% of 10000 = 3000.
	require.Equal(t, sdkmath.NewInt(3000), payout1, "SN1 should get 30%% of pool")
	// SN2: 70% of 10000 = 7000.
	require.Equal(t, sdkmath.NewInt(7000), payout2, "SN2 should get 70%% of pool")

	// Verify last distribution height was updated.
	require.Equal(t, int64(100), k.GetLastDistributionHeight(ctx))

	// Verify total distributed was updated.
	totalDist := k.GetTotalDistributed(ctx)
	require.Equal(t, sdkmath.NewInt(10000), totalDist.AmountOf("ulume"))
}

// AT36: SNs below min_cascade_bytes_for_payment excluded from distribution.
func TestMinCascadeBytesThreshold(t *testing.T) {
	k, ctx, bankKeeper, snKeeper := setupTestKeeper(t)

	params := types.DefaultParams()
	params.PaymentPeriodBlocks = 10
	params.MinCascadeBytesForPayment = 5000 // 5000 bytes minimum
	params.NewSnRampUpPeriods = 0
	params.MeasurementSmoothingPeriods = 0
	params.UsageGrowthCapBpsPerPeriod = 10000
	require.NoError(t, k.SetParams(ctx, params))

	// SN1: 3000 bytes (below threshold), SN2: 7000 bytes (above threshold).
	val1 := makeValAddr(1)
	acc1 := makeAccAddr(1)
	val2 := makeValAddr(2)
	acc2 := makeAccAddr(2)

	addSupernode(snKeeper, val1, acc1, sntypes.SuperNodeStateActive, 3000)
	addSupernode(snKeeper, val2, acc2, sntypes.SuperNodeStateActive, 7000)

	fundPool(bankKeeper, 10000)

	ctx = ctx.WithBlockHeight(100)
	k.SetLastDistributionHeight(ctx, 80)

	err := k.distributePool(ctx)
	require.NoError(t, err)

	// Only SN2 should receive payout (all of the pool).
	require.Len(t, bankKeeper.sent, 1)
	require.Equal(t, acc2.String(), bankKeeper.sent[0].to)
	require.Equal(t, sdkmath.NewInt(10000), bankKeeper.sent[0].amount.AmountOf("ulume"))
}

// AT37: New SN receives ramped-up (partial) payout weight during ramp-up period.
func TestNewSNRampUp(t *testing.T) {
	k, ctx, bankKeeper, snKeeper := setupTestKeeper(t)

	params := types.DefaultParams()
	params.PaymentPeriodBlocks = 10
	params.MinCascadeBytesForPayment = 1000
	params.NewSnRampUpPeriods = 4  // 4-period ramp-up
	params.MeasurementSmoothingPeriods = 0
	params.UsageGrowthCapBpsPerPeriod = 10000
	require.NoError(t, k.SetParams(ctx, params))

	// Two SNs with identical cascade bytes.
	val1 := makeValAddr(1) // Existing SN (4+ periods)
	acc1 := makeAccAddr(1)
	val2 := makeValAddr(2) // New SN (0 periods)
	acc2 := makeAccAddr(2)

	addSupernode(snKeeper, val1, acc1, sntypes.SuperNodeStateActive, 10000)
	addSupernode(snKeeper, val2, acc2, sntypes.SuperNodeStateActive, 10000)

	// Set SN1 as established (4 periods active).
	k.SetSNDistState(ctx, val1.String(), SNDistState{
		SmoothedBytes:          10000,
		PrevRawBytes:           10000,
		PeriodsActive:          4,
		EligibilityStartHeight: 1,
	})
	// SN2 has no prior state (new SN).

	fundPool(bankKeeper, 10000)

	ctx = ctx.WithBlockHeight(100)
	k.SetLastDistributionHeight(ctx, 80)

	err := k.distributePool(ctx)
	require.NoError(t, err)

	// SN1 has rampWeight = 1.0 (4/4), SN2 has rampWeight = 1/4 = 0.25.
	// Effective weights: SN1 = 10000*1.0 = 10000, SN2 = 10000*0.25 = 2500.
	// Total = 12500.
	// SN1 share = 10000/12500 = 0.8 -> 8000 ulume.
	// SN2 share = 2500/12500 = 0.2 -> 2000 ulume.
	var payout1, payout2 sdkmath.Int
	for _, s := range bankKeeper.sent {
		if s.to == acc1.String() {
			payout1 = s.amount.AmountOf("ulume")
		}
		if s.to == acc2.String() {
			payout2 = s.amount.AmountOf("ulume")
		}
	}

	require.Equal(t, sdkmath.NewInt(8000), payout1, "established SN should get 80%%")
	require.Equal(t, sdkmath.NewInt(2000), payout2, "new SN should get 20%% (ramped)")
}

// AT38: Usage growth cap limits reported cascade bytes increase per period.
func TestUsageGrowthCap(t *testing.T) {
	k, ctx, bankKeeper, snKeeper := setupTestKeeper(t)

	params := types.DefaultParams()
	params.PaymentPeriodBlocks = 10
	params.MinCascadeBytesForPayment = 1000
	params.NewSnRampUpPeriods = 0
	params.MeasurementSmoothingPeriods = 0 // No smoothing for clarity.
	params.UsageGrowthCapBpsPerPeriod = 1000 // 10% max growth per period.
	require.NoError(t, k.SetParams(ctx, params))

	// Single SN reporting 20000 bytes, but previous was 10000.
	val1 := makeValAddr(1)
	acc1 := makeAccAddr(1)
	addSupernode(snKeeper, val1, acc1, sntypes.SuperNodeStateActive, 20000)

	// Previous period had 10000 bytes.
	k.SetSNDistState(ctx, val1.String(), SNDistState{
		SmoothedBytes:          10000,
		PrevRawBytes:           10000,
		PeriodsActive:          5,
		EligibilityStartHeight: 1,
	})

	fundPool(bankKeeper, 10000)

	ctx = ctx.WithBlockHeight(100)
	k.SetLastDistributionHeight(ctx, 80)

	err := k.distributePool(ctx)
	require.NoError(t, err)

	// With 10% cap, max allowed = 10000 * 1.10 = 11000.
	// Since raw was 20000 > 11000, capped to 11000.
	// Smoothed (no smoothing) = 11000.
	// Verify the dist state was updated with the capped value.
	distState, found := k.GetSNDistState(ctx, val1.String())
	require.True(t, found)
	// Smoothed bytes should be 11000 (capped), not 20000.
	require.InDelta(t, 11000.0, distState.SmoothedBytes, 1.0)
	// PrevRawBytes should be the actual raw value (20000), not the capped one.
	require.InDelta(t, 20000.0, distState.PrevRawBytes, 1.0)
}

// AT44: Pool with zero balance produces no distribution and no panic.
func TestZeroPoolBalance(t *testing.T) {
	k, ctx, bankKeeper, snKeeper := setupTestKeeper(t)

	params := types.DefaultParams()
	params.PaymentPeriodBlocks = 10
	params.MinCascadeBytesForPayment = 1000
	params.NewSnRampUpPeriods = 0
	params.MeasurementSmoothingPeriods = 0
	params.UsageGrowthCapBpsPerPeriod = 10000
	require.NoError(t, k.SetParams(ctx, params))

	val1 := makeValAddr(1)
	acc1 := makeAccAddr(1)
	addSupernode(snKeeper, val1, acc1, sntypes.SuperNodeStateActive, 5000)

	// Pool balance is zero (no funding).

	ctx = ctx.WithBlockHeight(100)
	k.SetLastDistributionHeight(ctx, 80)

	// Should not panic and should return nil.
	err := k.distributePool(ctx)
	require.NoError(t, err)

	// No sends should have occurred.
	require.Empty(t, bankKeeper.sent)

	// Last distribution height should still be updated.
	require.Equal(t, int64(100), k.GetLastDistributionHeight(ctx))
}

// AT45: No eligible SNs produces no distribution and no panic.
func TestNoEligibleSNs(t *testing.T) {
	k, ctx, bankKeeper, _ := setupTestKeeper(t)

	params := types.DefaultParams()
	params.PaymentPeriodBlocks = 10
	params.MinCascadeBytesForPayment = 1000
	params.NewSnRampUpPeriods = 0
	params.MeasurementSmoothingPeriods = 0
	params.UsageGrowthCapBpsPerPeriod = 10000
	require.NoError(t, k.SetParams(ctx, params))

	// No supernodes added.
	fundPool(bankKeeper, 10000)

	ctx = ctx.WithBlockHeight(100)
	k.SetLastDistributionHeight(ctx, 80)

	// Should not panic and should return nil.
	err := k.distributePool(ctx)
	require.NoError(t, err)

	// No sends should have occurred.
	require.Empty(t, bankKeeper.sent)

	// Last distribution height should still be updated.
	require.Equal(t, int64(100), k.GetLastDistributionHeight(ctx))
}

// Test EndBlocker period check.
func TestEndBlockerPeriodCheck(t *testing.T) {
	k, ctx, bankKeeper, snKeeper := setupTestKeeper(t)

	params := types.DefaultParams()
	params.PaymentPeriodBlocks = 100
	params.MinCascadeBytesForPayment = 1000
	params.NewSnRampUpPeriods = 0
	params.MeasurementSmoothingPeriods = 0
	params.UsageGrowthCapBpsPerPeriod = 10000
	require.NoError(t, k.SetParams(ctx, params))

	val1 := makeValAddr(1)
	acc1 := makeAccAddr(1)
	addSupernode(snKeeper, val1, acc1, sntypes.SuperNodeStateActive, 5000)
	fundPool(bankKeeper, 10000)

	// Set last distribution at height 50. Current height 100 (only 50 blocks elapsed, need 100).
	k.SetLastDistributionHeight(ctx, 50)
	ctx = ctx.WithBlockHeight(100)

	err := k.EndBlocker(ctx)
	require.NoError(t, err)
	// No distribution should have occurred.
	require.Empty(t, bankKeeper.sent)

	// Now set height to 151 (101 blocks elapsed > 100).
	ctx = ctx.WithBlockHeight(151)
	err = k.EndBlocker(ctx)
	require.NoError(t, err)
	// Distribution should have occurred.
	require.NotEmpty(t, bankKeeper.sent)
}

// Test STORAGE_FULL SNs are also eligible.
func TestStorageFullSNsEligible(t *testing.T) {
	k, ctx, bankKeeper, snKeeper := setupTestKeeper(t)

	params := types.DefaultParams()
	params.PaymentPeriodBlocks = 10
	params.MinCascadeBytesForPayment = 1000
	params.NewSnRampUpPeriods = 0
	params.MeasurementSmoothingPeriods = 0
	params.UsageGrowthCapBpsPerPeriod = 10000
	require.NoError(t, k.SetParams(ctx, params))

	val1 := makeValAddr(1)
	acc1 := makeAccAddr(1)
	addSupernode(snKeeper, val1, acc1, sntypes.SuperNodeStateStorageFull, 5000)

	fundPool(bankKeeper, 10000)

	ctx = ctx.WithBlockHeight(100)
	k.SetLastDistributionHeight(ctx, 80)

	err := k.distributePool(ctx)
	require.NoError(t, err)

	require.Len(t, bankKeeper.sent, 1)
	require.Equal(t, acc1.String(), bankKeeper.sent[0].to)
	require.Equal(t, sdkmath.NewInt(10000), bankKeeper.sent[0].amount.AmountOf("ulume"))
}

// Test EMA smoothing.
func TestEMASmoothing(t *testing.T) {
	// alpha = 2/(4+1) = 0.4
	// prevSmoothed = 10000, newValue = 20000
	// EMA = 0.4 * 20000 + 0.6 * 10000 = 8000 + 6000 = 14000
	result := applyEMA(10000, 20000, 4)
	require.InDelta(t, 14000.0, result, 0.01)

	// First observation (prev = 0): should return new value.
	result = applyEMA(0, 5000, 4)
	require.InDelta(t, 5000.0, result, 0.01)

	// Zero smoothing periods: return new value directly.
	result = applyEMA(10000, 20000, 0)
	require.InDelta(t, 20000.0, result, 0.01)
}

// Test growth cap function.
func TestApplyGrowthCap(t *testing.T) {
	// 10% cap: prev = 10000, raw = 12000 -> capped to 11000.
	result := applyGrowthCap(12000, 10000, 1000)
	require.InDelta(t, 11000.0, result, 0.01)

	// 10% cap: prev = 10000, raw = 10500 -> not capped (within limit).
	result = applyGrowthCap(10500, 10000, 1000)
	require.InDelta(t, 10500.0, result, 0.01)

	// First observation (prev = 0): no cap.
	result = applyGrowthCap(50000, 0, 1000)
	require.InDelta(t, 50000.0, result, 0.01)
}

// Test ramp-up weight function.
func TestComputeRampUpWeight(t *testing.T) {
	// 0 out of 4 periods: weight = 1/4 = 0.25.
	require.InDelta(t, 0.25, computeRampUpWeight(0, 4), 0.001)
	// 1 out of 4 periods: weight = 2/4 = 0.5.
	require.InDelta(t, 0.50, computeRampUpWeight(1, 4), 0.001)
	// 3 out of 4 periods: weight = 4/4 = 1.0.
	require.InDelta(t, 1.0, computeRampUpWeight(3, 4), 0.001)
	// 4 out of 4 periods (past ramp-up): weight = 1.0.
	require.InDelta(t, 1.0, computeRampUpWeight(4, 4), 0.001)
	// 10 out of 4 periods: weight = 1.0.
	require.InDelta(t, 1.0, computeRampUpWeight(10, 4), 0.001)
	// Zero ramp-up periods: always 1.0.
	require.InDelta(t, 1.0, computeRampUpWeight(0, 0), 0.001)
}

// Test SNDistState persistence.
func TestSNDistStatePersistence(t *testing.T) {
	k, ctx, _, _ := setupTestKeeper(t)

	valAddr := makeValAddr(1).String()

	// Initially not found.
	_, found := k.GetSNDistState(ctx, valAddr)
	require.False(t, found)

	// Set state.
	state := SNDistState{
		SmoothedBytes:          12345.678,
		PrevRawBytes:           10000.0,
		EligibilityStartHeight: 42,
		PeriodsActive:          3,
	}
	k.SetSNDistState(ctx, valAddr, state)

	// Read back.
	got, found := k.GetSNDistState(ctx, valAddr)
	require.True(t, found)
	require.InDelta(t, state.SmoothedBytes, got.SmoothedBytes, 0.001)
	require.InDelta(t, state.PrevRawBytes, got.PrevRawBytes, 0.001)
	require.Equal(t, state.EligibilityStartHeight, got.EligibilityStartHeight)
	require.Equal(t, state.PeriodsActive, got.PeriodsActive)
}

// Test total distributed tracking.
func TestTotalDistributed(t *testing.T) {
	k, ctx, _, _ := setupTestKeeper(t)

	// Initially empty.
	total := k.GetTotalDistributed(ctx)
	require.True(t, total.IsZero())

	// Add some.
	k.AddTotalDistributed(ctx, sdk.NewCoins(sdk.NewCoin("ulume", sdkmath.NewInt(5000))))
	total = k.GetTotalDistributed(ctx)
	require.Equal(t, sdkmath.NewInt(5000), total.AmountOf("ulume"))

	// Add more.
	k.AddTotalDistributed(ctx, sdk.NewCoins(sdk.NewCoin("ulume", sdkmath.NewInt(3000))))
	total = k.GetTotalDistributed(ctx)
	require.Equal(t, sdkmath.NewInt(8000), total.AmountOf("ulume"))
}

// Test that SNs without metrics are skipped.
func TestSNWithoutMetricsSkipped(t *testing.T) {
	k, ctx, bankKeeper, snKeeper := setupTestKeeper(t)

	params := types.DefaultParams()
	params.PaymentPeriodBlocks = 10
	params.MinCascadeBytesForPayment = 1000
	params.NewSnRampUpPeriods = 0
	params.MeasurementSmoothingPeriods = 0
	params.UsageGrowthCapBpsPerPeriod = 10000
	require.NoError(t, k.SetParams(ctx, params))

	// Add SN without metrics (only add to supernodes list, not metrics map).
	val1 := makeValAddr(1)
	acc1 := makeAccAddr(1)
	sn := sntypes.SuperNode{
		ValidatorAddress: val1.String(),
		SupernodeAccount: acc1.String(),
		States: []*sntypes.SuperNodeStateRecord{
			{State: sntypes.SuperNodeStateActive},
		},
	}
	snKeeper.supernodes = append(snKeeper.supernodes, sn)
	// Intentionally NOT adding metrics.

	// Add SN2 with metrics.
	val2 := makeValAddr(2)
	acc2 := makeAccAddr(2)
	addSupernode(snKeeper, val2, acc2, sntypes.SuperNodeStateActive, 5000)

	fundPool(bankKeeper, 10000)

	ctx = ctx.WithBlockHeight(100)
	k.SetLastDistributionHeight(ctx, 80)

	err := k.distributePool(ctx)
	require.NoError(t, err)

	// Only SN2 should receive payout.
	require.Len(t, bankKeeper.sent, 1)
	require.Equal(t, acc2.String(), bankKeeper.sent[0].to)
}
