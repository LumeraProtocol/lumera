package integration_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/app"
	evmigrationkeeper "github.com/LumeraProtocol/lumera/x/evmigration/keeper"
	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// BenchmarkMigrateValidatorScaling measures how MsgMigrateValidator scales with a
// validator's own footprint — the number of delegation / unbonding-delegation /
// redelegation records re-keyed by the migration hot path.
//
// This mirrors the live testnet scenario (a validator with thousands of records)
// that motivated the scoped-iteration / double-fetch / O(1)-refcount optimizations
// in PR #184. It seeds a realistic mix (delegations dominate, plus a fraction of
// unbonding delegations and redelegations, ~ the testnet val1 ratio) via the real
// StakingKeeper, then times the migration itself with per-iteration setup excluded.
//
// The BENCHMARK itself is excluded from the normal pipeline by construction:
// `go test` does not run Benchmark* functions without an explicit -bench flag, and
// `make integration-tests` passes no -bench. Run it on demand (seeding thousands of
// delegations is slow, so pin one measured iteration per size with -benchtime=1x):
//
//	go test -tags='integration test' ./tests/integration/evmigration/ \
//	    -run='^$' -bench='^BenchmarkMigrateValidatorScaling$' -benchtime=1x -timeout=30m -v
//
// The reported "records/op" custom metric is the deterministic total (delegations +
// unbonding + redelegations) re-keyed, which — unlike wall-clock ns/op in this
// gas-meterless in-process harness — is the load-bearing scaling signal.
//
// Scale CORRECTNESS is separately guarded in the pipeline by
// TestMigrateValidatorAtScale (a plain Test that runs one realistic-scale
// migration and asserts full re-keying); it shares this file's harness.
func BenchmarkMigrateValidatorScaling(b *testing.B) {
	for _, total := range []int{1000, 2000, 4000, 6000} {
		b.Run(fmt.Sprintf("records=%d", total), func(b *testing.B) {
			mix := newValidatorRecordMix(total)
			for range b.N {
				b.StopTimer()
				h := newValScaleHarness(b)
				legacyPriv, legacyAddr, oldValAddr := h.seedValidator(b, mix)
				newPriv, newAddr := createNewEVMAddress(b)
				newValAddr := sdk.ValAddress(newAddr)
				msg := newValidatorMsg(b, legacyPriv, legacyAddr, newPriv, newAddr)
				b.StartTimer()

				_, err := h.msgServer.MigrateValidator(h.ctx, msg)

				b.StopTimer()
				require.NoError(b, err, "migration must succeed at scale (records=%d)", total)
				h.assertMigrated(b, oldValAddr, newValAddr, mix)
				b.ReportMetric(float64(mix.total()), "records/op")
			}
		})
	}
}

// TestMigrateValidatorAtScale is a pipeline correctness gate: it migrates a
// validator carrying a realistic testnet-scale footprint (~6000 delegation /
// unbonding / redelegation records in the observed val1 mix) and asserts every
// record is re-keyed onto the new operator address. It guards the scoped-iteration
// / double-fetch / O(1)-refcount hot-path optimizations (PR #184) against
// regressions that only surface at scale — e.g. a reintroduced full-chain scan or
// a pre-check bound that rejects large validators.
//
// Scaling MEASUREMENT (the ns/op curve across sizes) lives in the on-demand
// BenchmarkMigrateValidatorScaling. This test asserts only correctness at scale,
// which is deterministic and CI-stable — it makes no wall-clock assertion, so it
// never flakes on a slow or loaded runner.
func TestMigrateValidatorAtScale(t *testing.T) {
	const records = 6000
	mix := newValidatorRecordMix(records)

	h := newValScaleHarness(t)
	legacyPriv, legacyAddr, oldValAddr := h.seedValidator(t, mix)
	newPriv, newAddr := createNewEVMAddress(t)
	newValAddr := sdk.ValAddress(newAddr)
	msg := newValidatorMsg(t, legacyPriv, legacyAddr, newPriv, newAddr)

	_, err := h.msgServer.MigrateValidator(h.ctx, msg)
	require.NoError(t, err, "validator migration must succeed at scale (records=%d)", records)

	h.assertMigrated(t, oldValAddr, newValAddr, mix)
}

// validatorRecordMix splits a target record total into delegation / unbonding /
// redelegation counts approximating the observed testnet val1 ratio
// (2188 deleg + 979 unbond + 1982 redel ≈ 5149 → ~0.42 / 0.19 / 0.39). The self
// delegation is counted as one of the delegation records.
type validatorRecordMix struct {
	delegations   int
	unbonding     int
	redelegations int
}

func newValidatorRecordMix(total int) validatorRecordMix {
	if total < 3 {
		total = 3
	}
	unbonding := total * 19 / 100
	redelegations := total * 39 / 100
	delegations := total - unbonding - redelegations // remainder, includes the self delegation
	return validatorRecordMix{
		delegations:   delegations,
		unbonding:     unbonding,
		redelegations: redelegations,
	}
}

func (m validatorRecordMix) total() int {
	return m.delegations + m.unbonding + m.redelegations
}

type valScaleHarness struct {
	app       *app.App
	ctx       sdk.Context
	keeper    evmigrationkeeper.Keeper
	msgServer types.MsgServer
}

func newValScaleHarness(tb testing.TB) *valScaleHarness {
	tb.Helper()
	os.Setenv("SYSTEM_TESTS", "true")

	a := app.Setup(tb)
	ctx := a.BaseApp.NewContext(true).
		WithChainID(integrationTestChainID).
		WithBlockTime(time.Now().UTC())
	k := a.EvmigrationKeeper
	h := &valScaleHarness{
		app:       a,
		ctx:       ctx,
		keeper:    k,
		msgServer: evmigrationkeeper.NewMsgServerImpl(k),
	}
	// High MaxValidatorDelegations so large seed counts are not rejected by the
	// pre-check bound; the whole point of this benchmark is to exercise scale.
	require.NoError(tb, k.Params.Set(ctx, types.NewParams(true, 0, 50, 1_000_000, 20)))
	return h
}

// fundedAccount creates a registered secp256k1 account funded with coins.
func (h *valScaleHarness) fundedAccount(tb testing.TB, ulume int64) sdk.AccAddress {
	tb.Helper()
	priv := secp256k1.GenPrivKey()
	addr := sdk.AccAddress(priv.PubKey().Address())
	acc := h.app.AuthKeeper.NewAccountWithAddress(h.ctx, addr)
	baseAcc, ok := acc.(*authtypes.BaseAccount)
	require.True(tb, ok)
	require.NoError(tb, baseAcc.SetPubKey(priv.PubKey()))
	h.app.AuthKeeper.SetAccount(h.ctx, baseAcc)

	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", ulume))
	require.NoError(tb, h.app.BankKeeper.MintCoins(h.ctx, "mint", coins))
	require.NoError(tb, h.app.BankKeeper.SendCoinsFromModuleToAccount(h.ctx, "mint", addr, coins))
	return addr
}

// createUnbondedValidator sets up an Unbonded validator with initialized
// distribution state and a self delegation, returning the operator legacy key.
func (h *valScaleHarness) createUnbondedValidator(tb testing.TB, selfBond sdkmath.Int) (*secp256k1.PrivKey, sdk.AccAddress, sdk.ValAddress) {
	tb.Helper()
	legacyPriv := secp256k1.GenPrivKey()
	legacyAddr := sdk.AccAddress(legacyPriv.PubKey().Address())

	acc := h.app.AuthKeeper.NewAccountWithAddress(h.ctx, legacyAddr)
	baseAcc, ok := acc.(*authtypes.BaseAccount)
	require.True(tb, ok)
	require.NoError(tb, baseAcc.SetPubKey(legacyPriv.PubKey()))
	h.app.AuthKeeper.SetAccount(h.ctx, baseAcc)
	opCoins := sdk.NewCoins(sdk.NewInt64Coin("ulume", selfBond.Int64()*2))
	require.NoError(tb, h.app.BankKeeper.MintCoins(h.ctx, "mint", opCoins))
	require.NoError(tb, h.app.BankKeeper.SendCoinsFromModuleToAccount(h.ctx, "mint", legacyAddr, opCoins))

	valAddr := sdk.ValAddress(legacyAddr)
	consPub := ed25519.GenPrivKey().PubKey()
	pkAny, err := codectypes.NewAnyWithValue(consPub)
	require.NoError(tb, err)

	val := stakingtypes.Validator{
		OperatorAddress: valAddr.String(),
		ConsensusPubkey: pkAny,
		Jailed:          false,
		Status:          stakingtypes.Unbonded,
		Tokens:          sdkmath.ZeroInt(),
		DelegatorShares: sdkmath.LegacyZeroDec(),
		Description:     stakingtypes.Description{Moniker: "scale-validator"},
		Commission: stakingtypes.NewCommission(
			sdkmath.LegacyNewDecWithPrec(1, 1),
			sdkmath.LegacyNewDecWithPrec(2, 1),
			sdkmath.LegacyNewDecWithPrec(1, 2),
		),
		MinSelfDelegation: sdkmath.OneInt(),
	}
	require.NoError(tb, h.app.StakingKeeper.SetValidator(h.ctx, val))
	require.NoError(tb, h.app.StakingKeeper.SetValidatorByConsAddr(h.ctx, val))
	require.NoError(tb, h.app.StakingKeeper.SetNewValidatorByPowerIndex(h.ctx, val))

	require.NoError(tb, h.app.DistrKeeper.SetValidatorHistoricalRewards(h.ctx, valAddr, 0,
		distrtypes.NewValidatorHistoricalRewards(sdk.DecCoins{}, 1)))
	require.NoError(tb, h.app.DistrKeeper.SetValidatorCurrentRewards(h.ctx, valAddr,
		distrtypes.NewValidatorCurrentRewards(sdk.DecCoins{}, 1)))
	require.NoError(tb, h.app.DistrKeeper.SetValidatorAccumulatedCommission(h.ctx, valAddr,
		distrtypes.InitialValidatorAccumulatedCommission()))
	require.NoError(tb, h.app.DistrKeeper.SetValidatorOutstandingRewards(h.ctx, valAddr,
		distrtypes.ValidatorOutstandingRewards{Rewards: sdk.DecCoins{}}))

	val, err = h.app.StakingKeeper.GetValidator(h.ctx, valAddr)
	require.NoError(tb, err)
	_, err = h.app.StakingKeeper.Delegate(h.ctx, legacyAddr, selfBond, stakingtypes.Unbonded, val, true)
	require.NoError(tb, err)

	return legacyPriv, legacyAddr, valAddr
}

// delegate creates a fresh funded delegator and delegates to valAddr, returning
// the delegator's address.
func (h *valScaleHarness) delegate(tb testing.TB, valAddr sdk.ValAddress, amount sdkmath.Int) sdk.AccAddress {
	tb.Helper()
	delAddr := h.fundedAccount(tb, amount.Int64()*2)
	val, err := h.app.StakingKeeper.GetValidator(h.ctx, valAddr)
	require.NoError(tb, err)
	_, err = h.app.StakingKeeper.Delegate(h.ctx, delAddr, amount, stakingtypes.Unbonded, val, true)
	require.NoError(tb, err)
	return delAddr
}

// seedValidator builds a bonded validator whose re-keyable footprint matches mix:
// mix.delegations delegations (including the self delegation), mix.unbonding
// unbonding delegations, and mix.redelegations redelegations with the target as
// the redelegation SOURCE. Returns the operator legacy key and addresses.
func (h *valScaleHarness) seedValidator(tb testing.TB, mix validatorRecordMix) (*secp256k1.PrivKey, sdk.AccAddress, sdk.ValAddress) {
	tb.Helper()
	selfBond := sdkmath.NewInt(1_000_000)
	legacyPriv, legacyAddr, valAddr := h.createUnbondedValidator(tb, selfBond)
	h.bond(tb, valAddr)

	delAmt := sdkmath.NewInt(100_000)

	// Plain delegations (mix.delegations - 1 external; the self delegation is the
	// remaining one).
	for range mix.delegations - 1 {
		h.delegate(tb, valAddr, delAmt)
	}

	// Unbonding delegations: delegate then fully undelegate, leaving only the
	// unbonding-delegation record (no residual delegation to this validator).
	// advanceBlockTime spreads their completion times (see helper).
	for range mix.unbonding {
		h.advanceBlockTime()
		delAddr := h.delegate(tb, valAddr, delAmt)
		del, err := h.app.StakingKeeper.GetDelegation(h.ctx, delAddr, valAddr)
		require.NoError(tb, err)
		_, _, err = h.app.StakingKeeper.Undelegate(h.ctx, delAddr, valAddr, del.Shares)
		require.NoError(tb, err)
	}

	// Redelegations: delegate to the target, then redelegate target -> sink,
	// leaving a redelegation record with the target as the SOURCE validator
	// (and no residual delegation to the target).
	if mix.redelegations > 0 {
		_, _, sinkValAddr := h.createUnbondedValidator(tb, selfBond)
		h.bond(tb, sinkValAddr)
		for range mix.redelegations {
			h.advanceBlockTime()
			delAddr := h.delegate(tb, valAddr, delAmt)
			del, err := h.app.StakingKeeper.GetDelegation(h.ctx, delAddr, valAddr)
			require.NoError(tb, err)
			_, err = h.app.StakingKeeper.BeginRedelegation(h.ctx, delAddr, valAddr, sinkValAddr, del.Shares)
			require.NoError(tb, err)
		}
	}

	return legacyPriv, legacyAddr, valAddr
}

// advanceBlockTime moves the harness block time forward one second so that
// unbonding / redelegation records seeded on successive iterations receive
// DISTINCT completion times, landing in separate staking-queue buckets.
//
// This is load-bearing for realism, not cosmetics: the unbonding and
// redelegation queues are keyed by completion time, and each key holds a slice
// of all entries maturing at that instant. Seeding every record in one block
// gives them one shared completion time, piling all N entries into a single
// bucket; the migration then re-inserts them via InsertUBDQueue /
// InsertRedelegationQueue, each a read-append-write of the whole bucket — O(N²)
// serialization that a CPU profile pins on DVVTriplets marshal/unmarshal. A real
// validator accrues these records across thousands of blocks, so completion
// times are spread and buckets stay tiny; advancing time here reproduces that
// and lets the benchmark measure the true (linear) per-record migration cost.
func (h *valScaleHarness) advanceBlockTime() {
	h.ctx = h.ctx.WithBlockTime(h.ctx.BlockTime().Add(time.Second))
}

// bond promotes a validator to Bonded status and records its power.
func (h *valScaleHarness) bond(tb testing.TB, valAddr sdk.ValAddress) {
	tb.Helper()
	val, err := h.app.StakingKeeper.GetValidator(h.ctx, valAddr)
	require.NoError(tb, err)
	val.Status = stakingtypes.Bonded
	require.NoError(tb, h.app.StakingKeeper.SetValidator(h.ctx, val))
	require.NoError(tb, h.app.StakingKeeper.SetLastValidatorPower(h.ctx, valAddr, val.Tokens.Int64()))
}

// assertMigrated verifies the migration re-keyed every record type off the old
// operator address and onto the new one.
func (h *valScaleHarness) assertMigrated(tb testing.TB, oldValAddr, newValAddr sdk.ValAddress, mix validatorRecordMix) {
	tb.Helper()

	newDels, err := h.app.StakingKeeper.GetValidatorDelegations(h.ctx, newValAddr)
	require.NoError(tb, err)
	require.Len(tb, newDels, mix.delegations, "all delegations re-keyed to new valoper")

	oldDels, err := h.app.StakingKeeper.GetValidatorDelegations(h.ctx, oldValAddr)
	require.NoError(tb, err)
	require.Empty(tb, oldDels, "no delegations remain under old valoper")

	newUbds, err := h.app.StakingKeeper.GetUnbondingDelegationsFromValidator(h.ctx, newValAddr)
	require.NoError(tb, err)
	require.Len(tb, newUbds, mix.unbonding, "all unbonding delegations re-keyed")

	newReds, err := h.app.StakingKeeper.GetRedelegationsFromSrcValidator(h.ctx, newValAddr)
	require.NoError(tb, err)
	require.Len(tb, newReds, mix.redelegations, "all source-role redelegations re-keyed")

	// The migration record is keyed by the operator's account-address string
	// (sdk.AccAddress(valoper)); its presence confirms the migration finalized.
	legacyAcc := sdk.AccAddress(oldValAddr)
	rec, err := h.keeper.MigrationRecords.Get(h.ctx, legacyAcc.String())
	require.NoError(tb, err, "migration record stored for the operator")
	require.Equal(tb, legacyAcc.String(), rec.LegacyAddress)
}
