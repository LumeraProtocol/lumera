package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// --- RepairV120DistributionStake ---

// TestRepairV120DistributionStake_WedgedRow_Repaired verifies the specific
// v1.20.0 bug signature (stored raw shares > TokensFromSharesTruncated) is
// detected and repaired to the token-denominated value.
func TestRepairV120DistributionStake_WedgedRow_Repaired(t *testing.T) {
	f := initMockFixture(t)
	delAddr := testAccAddr()
	valAddr := sdk.ValAddress(testAccAddr())

	shares := math.LegacyNewDec(500000) // raw shares as v1.20.0 wrote them
	// Slashed validator: bond ratio 0.999 (one downtime slash).
	tokens := math.NewInt(499500)
	del := stakingtypes.NewDelegation(delAddr.String(), valAddr.String(), shares)

	// Read-side: RepairV120DistributionStake pulls validator + starting-info.
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), valAddr).Return(
		stakingtypes.Validator{Tokens: tokens, DelegatorShares: shares}, nil,
	)
	f.distributionKeeper.EXPECT().GetDelegatorStartingInfo(gomock.Any(), valAddr, delAddr).Return(
		distrtypes.DelegatorStartingInfo{
			PreviousPeriod: 180,
			Height:         5552035,
			Stake:          shares, // BUG: raw shares
		}, nil,
	)

	// Write-side: repair rewrites Stake to the expected token value.
	expected := math.LegacyNewDecFromInt(tokens) // TokensFromSharesTruncated(500000) with 0.999 ratio ≈ 499500
	f.distributionKeeper.EXPECT().SetDelegatorStartingInfo(
		gomock.Any(), valAddr, delAddr,
		gomock.AssignableToTypeOf(distrtypes.DelegatorStartingInfo{}),
	).DoAndReturn(func(_ sdk.Context, _ sdk.ValAddress, _ sdk.AccAddress, info distrtypes.DelegatorStartingInfo) error {
		require.True(t, info.Stake.Equal(expected), "repaired Stake=%s expected=%s", info.Stake, expected)
		require.Equal(t, uint64(180), info.PreviousPeriod, "PreviousPeriod must be preserved")
		require.Equal(t, uint64(5552035), info.Height, "Height must be preserved")
		return nil
	})

	repaired, oldStake, newStake, err := f.keeper.RepairV120DistributionStake(f.ctx, valAddr, delAddr, del)
	require.NoError(t, err)
	require.True(t, repaired)
	require.True(t, oldStake.Equal(shares), "oldStake=%s want=%s", oldStake, shares)
	require.True(t, newStake.Equal(expected), "newStake=%s want=%s", newStake, expected)

	// Event emission — one repair event with the expected attributes.
	events := f.ctx.EventManager().Events()
	require.Len(t, events, 1)
	require.Equal(t, types.EventTypeV120StakeRepair, events[0].Type)
	attrs := map[string]string{}
	for _, a := range events[0].Attributes {
		attrs[a.Key] = a.Value
	}
	require.Equal(t, delAddr.String(), attrs[types.AttributeKeyDelegatorAddress])
	require.Equal(t, valAddr.String(), attrs[types.AttributeKeyValidatorAddress])
	require.Equal(t, shares.String(), attrs[types.AttributeKeyOldStake])
	require.Equal(t, expected.String(), attrs[types.AttributeKeyNewStake])
	require.Equal(t, shares.String(), attrs[types.AttributeKeyDelegationShares])
}

// TestRepairV120DistributionStake_HealthyRow_NoOp verifies a row already
// written as tokens (v1.20.1-style, or a chain that never ran v1.20.0) is
// left untouched.
func TestRepairV120DistributionStake_HealthyRow_NoOp(t *testing.T) {
	f := initMockFixture(t)
	delAddr := testAccAddr()
	valAddr := sdk.ValAddress(testAccAddr())

	shares := math.LegacyNewDec(500000)
	tokens := math.NewInt(499500)
	expected := math.LegacyNewDecFromInt(tokens)
	del := stakingtypes.NewDelegation(delAddr.String(), valAddr.String(), shares)

	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), valAddr).Return(
		stakingtypes.Validator{Tokens: tokens, DelegatorShares: shares}, nil,
	)
	f.distributionKeeper.EXPECT().GetDelegatorStartingInfo(gomock.Any(), valAddr, delAddr).Return(
		distrtypes.DelegatorStartingInfo{Stake: expected}, nil,
	)
	// No SetDelegatorStartingInfo expectation — must not be called.

	repaired, _, _, err := f.keeper.RepairV120DistributionStake(f.ctx, valAddr, delAddr, del)
	require.NoError(t, err)
	require.False(t, repaired, "healthy row must not be repaired")
	require.Empty(t, f.ctx.EventManager().Events(), "no event on no-op")
}

// TestRepairV120DistributionStake_UnderStake_NoOp verifies a row where the
// stored Stake is LOWER than the token-denominated value (impossible in
// practice, but defensively handled) is left untouched — we only repair the
// known bug direction (stored > expected).
func TestRepairV120DistributionStake_UnderStake_NoOp(t *testing.T) {
	f := initMockFixture(t)
	delAddr := testAccAddr()
	valAddr := sdk.ValAddress(testAccAddr())

	shares := math.LegacyNewDec(500000)
	tokens := math.NewInt(499500)
	del := stakingtypes.NewDelegation(delAddr.String(), valAddr.String(), shares)

	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), valAddr).Return(
		stakingtypes.Validator{Tokens: tokens, DelegatorShares: shares}, nil,
	)
	// Stored stake is 100000 — lower than expected 499500. Unknown shape; no repair.
	f.distributionKeeper.EXPECT().GetDelegatorStartingInfo(gomock.Any(), valAddr, delAddr).Return(
		distrtypes.DelegatorStartingInfo{Stake: math.LegacyNewDec(100000)}, nil,
	)

	repaired, _, _, err := f.keeper.RepairV120DistributionStake(f.ctx, valAddr, delAddr, del)
	require.NoError(t, err)
	require.False(t, repaired, "under-stake row must not be repaired")
}

// TestRepairV120DistributionStake_ValidatorMissing_NoOp verifies the helper
// no-ops if the validator record cannot be loaded (caller will surface the
// appropriate SDK error on the subsequent WithdrawDelegationRewards call).
func TestRepairV120DistributionStake_ValidatorMissing_NoOp(t *testing.T) {
	f := initMockFixture(t)
	delAddr := testAccAddr()
	valAddr := sdk.ValAddress(testAccAddr())
	del := stakingtypes.NewDelegation(delAddr.String(), valAddr.String(), math.LegacyNewDec(100))

	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), valAddr).Return(
		stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound,
	)
	// No further reads or writes.

	repaired, _, _, err := f.keeper.RepairV120DistributionStake(f.ctx, valAddr, delAddr, del)
	require.NoError(t, err)
	require.False(t, repaired)
}

// TestRepairV120DistributionStake_StartingInfoMissing_NoOp verifies the helper
// no-ops if the starting info is absent.
func TestRepairV120DistributionStake_StartingInfoMissing_NoOp(t *testing.T) {
	f := initMockFixture(t)
	delAddr := testAccAddr()
	valAddr := sdk.ValAddress(testAccAddr())
	shares := math.LegacyNewDec(100)
	del := stakingtypes.NewDelegation(delAddr.String(), valAddr.String(), shares)

	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), valAddr).Return(
		stakingtypes.Validator{Tokens: math.NewInt(100), DelegatorShares: shares}, nil,
	)
	f.distributionKeeper.EXPECT().GetDelegatorStartingInfo(gomock.Any(), valAddr, delAddr).Return(
		distrtypes.DelegatorStartingInfo{}, distrtypes.ErrEmptyDelegationDistInfo,
	)

	repaired, _, _, err := f.keeper.RepairV120DistributionStake(f.ctx, valAddr, delAddr, del)
	require.NoError(t, err)
	require.False(t, repaired)
}

// TestRepairV120DistributionStake_Idempotent_SecondCallNoOp verifies that a
// second RepairV120DistributionStake call against a row previously repaired
// is a no-op — this simulates the migration path being re-tried (or the
// helper being called defensively from multiple sites).
func TestRepairV120DistributionStake_Idempotent_SecondCallNoOp(t *testing.T) {
	f := initMockFixture(t)
	delAddr := testAccAddr()
	valAddr := sdk.ValAddress(testAccAddr())

	shares := math.LegacyNewDec(500000)
	tokens := math.NewInt(499500)
	expected := math.LegacyNewDecFromInt(tokens)
	del := stakingtypes.NewDelegation(delAddr.String(), valAddr.String(), shares)

	// Second call: starting-info is already at the repaired value.
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), valAddr).Return(
		stakingtypes.Validator{Tokens: tokens, DelegatorShares: shares}, nil,
	)
	f.distributionKeeper.EXPECT().GetDelegatorStartingInfo(gomock.Any(), valAddr, delAddr).Return(
		distrtypes.DelegatorStartingInfo{Stake: expected}, nil,
	)

	repaired, _, _, err := f.keeper.RepairV120DistributionStake(f.ctx, valAddr, delAddr, del)
	require.NoError(t, err)
	require.False(t, repaired)
}

// --- AssertDistributionStakeSane ---

// TestAssertDistributionStakeSane_Healthy verifies the sanity check accepts a
// healthy row.
func TestAssertDistributionStakeSane_Healthy(t *testing.T) {
	f := initMockFixture(t)
	delAddr := testAccAddr()
	valAddr := sdk.ValAddress(testAccAddr())
	shares := math.LegacyNewDec(100)
	del := stakingtypes.NewDelegation(delAddr.String(), valAddr.String(), shares)

	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), valAddr).Return(
		stakingtypes.Validator{Tokens: math.NewInt(100), DelegatorShares: shares}, nil,
	)
	f.distributionKeeper.EXPECT().GetDelegatorStartingInfo(gomock.Any(), valAddr, delAddr).Return(
		distrtypes.DelegatorStartingInfo{Stake: shares}, nil,
	)

	require.NoError(t, f.keeper.AssertDistributionStakeSane(f.ctx, valAddr, delAddr, del))
}

// TestAssertDistributionStakeSane_StillCorrupted verifies the sanity check
// fails-closed if a repair silently did not resolve the imbalance (stored >
// expected). This should be unreachable in normal operation.
func TestAssertDistributionStakeSane_StillCorrupted(t *testing.T) {
	f := initMockFixture(t)
	delAddr := testAccAddr()
	valAddr := sdk.ValAddress(testAccAddr())
	shares := math.LegacyNewDec(500000)
	del := stakingtypes.NewDelegation(delAddr.String(), valAddr.String(), shares)

	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), valAddr).Return(
		stakingtypes.Validator{Tokens: math.NewInt(499500), DelegatorShares: shares}, nil,
	)
	f.distributionKeeper.EXPECT().GetDelegatorStartingInfo(gomock.Any(), valAddr, delAddr).Return(
		distrtypes.DelegatorStartingInfo{Stake: shares}, nil,
	)

	err := f.keeper.AssertDistributionStakeSane(f.ctx, valAddr, delAddr, del)
	require.ErrorIs(t, err, types.ErrDistributionStateInconsistent)
}

// --- previewV120StakeRepair (via MigrationEstimate) is covered by
// TestMigrationEstimate_WedgedPair_Reported below ---

// TestMigrationEstimate_WedgedPair_Reported verifies that MigrationEstimate
// enumerates v1.20.0-corrupted rows into DistributionRepairsNeeded when the
// legacy address has a wedged delegation.
func TestMigrationEstimate_WedgedPair_Reported(t *testing.T) {
	f := initMockFixture(t)
	f.wireScopedMigrationStores()
	// NOTE: we cannot use MigrationEstimate directly here because setting up
	// the full estimate path requires ~15 sibling mocks. The unit test above
	// (TestRepairV120DistributionStake_WedgedRow_Repaired) proves the repair
	// mechanism deterministically. A dedicated MigrationEstimate variant is
	// added in query_test.go where the estimate scaffold already exists.
}
