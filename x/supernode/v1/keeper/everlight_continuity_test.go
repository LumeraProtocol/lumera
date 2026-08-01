package keeper

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// everlightContinuityParams returns distribution params with every
// state-dependent lever switched ON, so a lost SNDistState cannot be masked.
//
// This matters: with ramp-up disabled and smoothing at 1, a migrated SN that
// lost its accumulator would still be paid the same amount, and the test would
// pass while the bug shipped. Non-zero ramp-up plus multi-period smoothing plus
// a real growth cap make PeriodsActive, SmoothedBytes and PrevRawBytes all
// observable in the payout.
func everlightContinuityParams() sntypes.Params {
	params := sntypes.DefaultParams()
	params.RewardDistribution.PaymentPeriodBlocks = 10
	params.RewardDistribution.MinCascadeBytesForPayment = 1000
	params.RewardDistribution.NewSnRampUpPeriods = 4
	params.RewardDistribution.MeasurementSmoothingPeriods = 4
	params.RewardDistribution.UsageGrowthCapBpsPerPeriod = 1000 // 10%
	return params
}

// everlightPeerState is the second, never-migrated supernode present in every
// scenario below.
//
// A single supernode always receives 100% of the pool no matter what its weight
// is, which would make payout equality trivially true and prove nothing. With a
// stable peer sharing the pool, the migrated SN's payout becomes a function of
// its *relative* effective weight — so any loss or reset of SNDistState moves
// the number.
var everlightPeerState = SNDistState{
	SmoothedBytes:          6000,
	PrevRawBytes:           6000,
	EligibilityStartHeight: 5,
	PeriodsActive:          9, // past ramp-up, so the peer is a stable reference
}

// runEverlightTick executes one distribution period and returns the payout the
// given account received.
func runEverlightTick(t *testing.T, k Keeper, ctx sdk.Context, bank *mockBankKeeper, account string) sdkmath.Int {
	t.Helper()

	before := len(bank.sent)
	ctx = ctx.WithBlockHeight(100)
	k.SetLastDistributionHeight(ctx, 80)
	require.NoError(t, k.distributePool(ctx))

	payout := sdkmath.ZeroInt()
	for _, s := range bank.sent[before:] {
		if s.to == account {
			payout = payout.Add(s.amount.AmountOf("ulume"))
		}
	}
	return payout
}

// everlightScenario builds a two-supernode chain: the subject SN under test and
// a stable peer. Returns the keeper, ctx, bank and the subject's bech32
// validator + account.
func everlightScenario(t *testing.T) (Keeper, sdk.Context, *mockBankKeeper, *mockSupernodeKeeper, string, string) {
	t.Helper()

	k, ctx, bank, snKeeper, auditKeeper := setupTestKeeper(t)
	require.NoError(t, k.SetParams(ctx, everlightContinuityParams()))

	subjectVal, subjectAcc := makeValAddr(1), makeAccAddr(1)
	peerVal, peerAcc := makeValAddr(3), makeAccAddr(3)
	addSupernode(snKeeper, auditKeeper, subjectVal, subjectAcc, sntypes.SuperNodeStateActive, everlightSubjectRawBytes)
	addSupernode(snKeeper, auditKeeper, peerVal, peerAcc, sntypes.SuperNodeStateActive, everlightPeerRawBytes)

	k.SetSNDistState(ctx, snKeeper.supernodes[1].ValidatorAddress, everlightPeerState)
	fundPool(bank, everlightPoolAmount)

	return k, ctx, bank, snKeeper, snKeeper.supernodes[0].ValidatorAddress, snKeeper.supernodes[0].SupernodeAccount
}

const (
	everlightSubjectRawBytes = 9500.0
	everlightPeerRawBytes    = 6200.0
	everlightPoolAmount      = 10000
)

var everlightSubjectState = SNDistState{
	SmoothedBytes:          8000,
	PrevRawBytes:           9000,
	EligibilityStartHeight: 12,
	PeriodsActive:          2, // inside ramp-up, so the value is observable
}

// TestEverlightNextTickContinuityAfterValidatorMigration proves invariant I4 and
// GOLDEN §7: after a validator-address migration, the NEXT Everlight
// distribution must pay exactly what it would have paid without the migration.
//
// This is the assertion the original gap analysis said was missing. Checking
// that `rdist/<oldVal>` was copied to `rdist/<newVal>` only proves the bytes
// moved; it does not prove the accumulator is still *consumed* correctly by
// distributePool, which reads SNDistState keyed by validator address and feeds
// PrevRawBytes into the growth cap, SmoothedBytes into the EMA, and
// PeriodsActive into the ramp-up weight. A migration that moved the row but
// left the consumer reading the old key — or that silently reset the state —
// would produce a different payout on the next tick.
//
// The test is a differential against a control chain that never migrated, with
// identical seeded state, so the assertion is payout equality rather than a
// hand-computed constant that would drift with the formula.
func TestEverlightNextTickContinuityAfterValidatorMigration(t *testing.T) {
	// --- Control: no migration. ---
	controlKeeper, controlCtx, controlBank, _, controlValBech, controlAccBech := everlightScenario(t)
	controlKeeper.SetSNDistState(controlCtx, controlValBech, everlightSubjectState)
	controlPayout := runEverlightTick(t, controlKeeper, controlCtx, controlBank, controlAccBech)
	require.True(t, controlPayout.IsPositive(),
		"control must actually pay out, otherwise payout equality is vacuous")
	require.True(t, controlPayout.LT(sdkmath.NewInt(everlightPoolAmount)),
		"control must share the pool with the peer, otherwise relative weight is untested")

	// --- Subject: identical state, then a validator identity migration. ---
	subjectKeeper, subjectCtx, subjectBank, subjectSN, sourceValBech, subjectAccBech := everlightScenario(t)
	subjectKeeper.SetSNDistState(subjectCtx, sourceValBech, everlightSubjectState)

	sourceVal, destinationVal := makeValAddr(1), makeValAddr(2)

	// Move the Everlight accumulator through the production plan API. The
	// SuperNode primary/index rewrite itself is PR196-owned; what must hold here
	// is that the mutable distribution state follows the validator identity.
	plan, err := subjectKeeper.BuildIdentityMigrationPlan(subjectCtx, sourceVal, destinationVal)
	require.NoError(t, err)
	require.NoError(t, subjectKeeper.ApplyIdentityMigrationPlan(subjectCtx, plan))

	destinationValBech, err := sdk.Bech32ifyAddressBytes("lumeravaloper", destinationVal)
	require.NoError(t, err)

	// The accumulator moved exactly once: gone from source, present at
	// destination, byte-identical.
	_, stillAtSource := subjectKeeper.GetSNDistState(subjectCtx, sourceValBech)
	require.False(t, stillAtSource, "SNDistState must not remain under the old validator")
	moved, found := subjectKeeper.GetSNDistState(subjectCtx, destinationValBech)
	require.True(t, found, "SNDistState must exist under the new validator")
	require.Equal(t, everlightSubjectState, moved, "SNDistState must move without mutation")

	// Complete the identity change on the SuperNode record itself, mirroring
	// what the evmigration validator flow commits, so the next tick enumerates
	// the node under its new validator address. The account index is unique per
	// account, so the source record must be removed before the destination is
	// written.
	migrated := subjectSN.supernodes[0]
	migrated.ValidatorAddress = destinationValBech
	subjectKeeper.DeleteSuperNode(subjectCtx, sourceVal)
	require.NoError(t, subjectKeeper.SetSuperNode(subjectCtx, migrated))
	require.NoError(t, subjectKeeper.SetMetricsState(subjectCtx, sntypes.SupernodeMetricsState{
		ValidatorAddress: destinationValBech,
		Metrics:          &sntypes.SupernodeMetrics{CascadeKademliaDbBytes: everlightSubjectRawBytes},
		Height:           subjectCtx.BlockHeight(),
	}))

	subjectPayout := runEverlightTick(t, subjectKeeper, subjectCtx, subjectBank, subjectAccBech)

	// THE assertion: the next tick pays identically across the migration.
	require.Equal(t, controlPayout, subjectPayout,
		"next Everlight distribution must be identical after validator migration (control=%s subject=%s)",
		controlPayout, subjectPayout)

	// And the accumulator advances under the new identity, not the old one.
	advanced, found := subjectKeeper.GetSNDistState(subjectCtx, destinationValBech)
	require.True(t, found)
	require.Equal(t, everlightSubjectState.PeriodsActive+1, advanced.PeriodsActive,
		"periods_active must continue advancing under the destination validator")
	_, resurrected := subjectKeeper.GetSNDistState(subjectCtx, sourceValBech)
	require.False(t, resurrected, "the tick must not re-create state under the old validator")
}

// TestEverlightDiscontinuityWouldChangeNextPayout is the negative control for
// the test above. It proves the payout-equality assertion has teeth by showing
// that a LOST accumulator — the exact pre-fix behavior, where validator
// migration left SNDistState behind under the old key — produces a materially
// different payout on the next tick.
//
// Without this, TestEverlightNextTickContinuityAfterValidatorMigration could
// pass for the wrong reason (e.g. if the params or topology made state
// irrelevant), and we would have no evidence the gap was ever real.
func TestEverlightDiscontinuityWouldChangeNextPayout(t *testing.T) {
	// With accumulator.
	withKeeper, withCtx, withBank, _, withValBech, withAccBech := everlightScenario(t)
	withKeeper.SetSNDistState(withCtx, withValBech, everlightSubjectState)
	withPayout := runEverlightTick(t, withKeeper, withCtx, withBank, withAccBech)

	// Without accumulator (simulating the discontinuity), same everything else.
	lostKeeper, lostCtx, lostBank, _, _, lostAccBech := everlightScenario(t)
	lostPayout := runEverlightTick(t, lostKeeper, lostCtx, lostBank, lostAccBech)

	require.NotEqual(t, withPayout, lostPayout,
		"losing SNDistState must change the next payout (with=%s lost=%s); if these are "+
			"equal the continuity assertion above proves nothing", withPayout, lostPayout)
}
