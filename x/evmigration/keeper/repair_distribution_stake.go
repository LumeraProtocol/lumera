package keeper

import (
	"fmt"

	"cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// RepairV120DistributionStake repairs a DelegatorStartingInfo row whose Stake
// field was written as raw shares by the v1.20.0 evmigration bug, before the
// v1.20.1 hotfix (commit 4ce27cf0) corrected the writer to use
// TokensFromSharesTruncated.
//
// Background: In v1.20.0, migrateActiveDelegations and MigrateValidatorDelegations
// wrote `Stake: del.Shares` into the distribution store. Distribution expects
// tokens (post-slash quantity) rather than shares. On an ever-slashed validator
// the two differ (shares > tokens), so the SDK's stake-sanity guard in
// CalculateDelegationRewards panics with
// "calculated final stake for delegator ... greater than current stake" the
// next time rewards are withdrawn or shares are modified. The delegation is
// then wedged: no user-space action can heal it because every share-modifying
// hook (delegate / undelegate / redelegate) first calls
// withdrawDelegationRewards, which panics on the same guard.
//
// This helper detects the specific bug signature (`stored.Stake > expected`)
// and rewrites Stake to the value v1.20.1's fixed writer would have produced.
// PreviousPeriod and Height are preserved so that any legitimate rewards
// accrued in the affected window are not double-counted or forfeited beyond
// what the guard already skipped.
//
// The caller passes the Delegation record it is already iterating over, so
// this helper does not re-load it from the staking keeper.
//
// Safety:
//   - Loads the validator to compute expected := val.TokensFromSharesTruncated(del.Shares).
//     If the validator record is not found, returns (false, nil) without writing —
//     the caller will surface the appropriate SDK error.
//   - Loads the current DelegatorStartingInfo. If none exists (delegator has no
//     rewards state for this validator), returns (false, nil) without writing.
//   - If stored.Stake ≤ expected, the row is healthy (or a v1.20.1-written row)
//     and returns (false, nil) — idempotent.
//   - If stored.Stake > expected, rewrites Stake := expected, emits an audit
//     event, and returns (true, nil).
//
// Determinism: reads and writes are deterministic; math.LegacyDec.GT is exact;
// TokensFromSharesTruncated is the same function v1.20.1 uses on the write
// path.
func (k Keeper) RepairV120DistributionStake(
	ctx sdk.Context,
	valAddr sdk.ValAddress,
	delAddr sdk.AccAddress,
	del stakingtypes.Delegation,
) (repaired bool, oldStake, newStake math.LegacyDec, err error) {
	// Load the validator to convert shares to tokens (post-slash quantity).
	val, valErr := k.stakingKeeper.GetValidator(ctx, valAddr)
	if valErr != nil {
		return false, math.LegacyDec{}, math.LegacyDec{}, nil
	}

	// Load the existing distribution starting info. If absent, nothing to
	// repair; the caller will hit ErrEmptyDelegationDistInfo from the SDK,
	// which is a legitimate condition that this helper does not create.
	startingInfo, siErr := k.distributionKeeper.GetDelegatorStartingInfo(ctx, valAddr, delAddr)
	if siErr != nil {
		return false, math.LegacyDec{}, math.LegacyDec{}, nil
	}

	expected := val.TokensFromSharesTruncated(del.Shares)

	// Healthy row: stored Stake is already tokens (or lower, which the SDK
	// tolerates via marginOfErr). No-op — do not rewrite, do not emit.
	if !startingInfo.Stake.GT(expected) {
		return false, startingInfo.Stake, startingInfo.Stake, nil
	}

	// Corrupted row: stored.Stake > expected. The only known cause is the
	// v1.20.0 bug that wrote raw shares. Repair by rewriting Stake to the
	// v1.20.1-correct value.
	oldStake = startingInfo.Stake
	startingInfo.Stake = expected
	newStake = expected

	if err := k.distributionKeeper.SetDelegatorStartingInfo(ctx, valAddr, delAddr, startingInfo); err != nil {
		return false, oldStake, newStake, fmt.Errorf("write repaired starting info for (%s, %s): %w", delAddr.String(), valAddr.String(), err)
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeV120StakeRepair,
		sdk.NewAttribute(types.AttributeKeyDelegatorAddress, delAddr.String()),
		sdk.NewAttribute(types.AttributeKeyValidatorAddress, valAddr.String()),
		sdk.NewAttribute(types.AttributeKeyOldStake, oldStake.String()),
		sdk.NewAttribute(types.AttributeKeyNewStake, newStake.String()),
		sdk.NewAttribute(types.AttributeKeyDelegationShares, del.Shares.String()),
	))

	return true, oldStake, newStake, nil
}

// previewV120StakeRepair is a read-only variant used by MigrationEstimate.
// Returns (repair, true) if the (val, del) row would be repaired by
// RepairV120DistributionStake, or (nil, false) if the row is healthy or
// unreadable. Never writes state; safe to call from a query handler.
func (k Keeper) previewV120StakeRepair(
	ctx sdk.Context,
	valAddr sdk.ValAddress,
	delAddr sdk.AccAddress,
	del stakingtypes.Delegation,
) (*types.DistributionStakeRepair, bool) {
	val, err := k.stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return nil, false
	}
	startingInfo, err := k.distributionKeeper.GetDelegatorStartingInfo(ctx, valAddr, delAddr)
	if err != nil {
		return nil, false
	}
	expected := val.TokensFromSharesTruncated(del.Shares)
	if !startingInfo.Stake.GT(expected) {
		return nil, false
	}
	return &types.DistributionStakeRepair{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		OldStake:         startingInfo.Stake.String(),
		NewStake:         expected.String(),
	}, true
}

// AssertDistributionStakeSane returns ErrDistributionStateInconsistent if the
// stored DelegatorStartingInfo.Stake still exceeds the expected token-denominated
// value after a repair attempt. Callers use this immediately after
// RepairV120DistributionStake to fail-closed if a repair silently did not
// resolve the imbalance — this should be unreachable in normal operation, but
// exists to prevent a stale panic string from propagating up as a chain-recovery-caught
// panic during migration.
func (k Keeper) AssertDistributionStakeSane(
	ctx sdk.Context,
	valAddr sdk.ValAddress,
	delAddr sdk.AccAddress,
	del stakingtypes.Delegation,
) error {
	val, err := k.stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return nil
	}
	startingInfo, err := k.distributionKeeper.GetDelegatorStartingInfo(ctx, valAddr, delAddr)
	if err != nil {
		return nil
	}
	expected := val.TokensFromSharesTruncated(del.Shares)
	if startingInfo.Stake.GT(expected) {
		return errors.Wrapf(
			types.ErrDistributionStateInconsistent,
			"delegator=%s validator=%s stored_stake=%s expected_max=%s",
			delAddr.String(), valAddr.String(), startingInfo.Stake.String(), expected.String(),
		)
	}
	return nil
}
