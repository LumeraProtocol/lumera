package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// MigrateValidator migrates a validator operator from legacy to new address.
// Performs everything MsgClaimLegacyAccount does PLUS validator-specific state re-keying.
func (ms msgServer) MigrateValidator(goCtx context.Context, msg *types.MsgMigrateValidator) (*types.MsgMigrateValidatorResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	legacyAddr, err := sdk.AccAddressFromBech32(msg.LegacyAddress)
	if err != nil {
		return nil, err
	}
	newAddr, err := sdk.AccAddressFromBech32(msg.NewAddress)
	if err != nil {
		return nil, err
	}

	// --- Pre-checks (shared) ---
	if err := ms.preChecks(ctx, legacyAddr, newAddr); err != nil {
		return nil, err
	}

	// --- Validator-specific pre-checks ---
	oldValAddr := sdk.ValAddress(legacyAddr)
	newValAddr := sdk.ValAddress(newAddr)

	val, err := ms.stakingKeeper.GetValidator(ctx, oldValAddr)
	if err != nil {
		return nil, types.ErrNotValidator
	}

	// Reject if validator is unbonding or unbonded.
	if val.Status == stakingtypes.Unbonding || val.Status == stakingtypes.Unbonded {
		return nil, types.ErrValidatorUnbonding
	}

	// Total delegation/unbonding/redelegation record count must not exceed
	// MaxValidatorDelegations to bound the gas cost of re-keying all records.
	params, err := ms.Params.Get(ctx)
	if err != nil {
		return nil, err
	}
	delegations, err := ms.stakingKeeper.GetValidatorDelegations(ctx, oldValAddr)
	if err != nil {
		return nil, err
	}
	ubds, err := ms.stakingKeeper.GetUnbondingDelegationsFromValidator(ctx, oldValAddr)
	if err != nil {
		return nil, err
	}
	reds, err := ms.stakingKeeper.GetRedelegationsFromSrcValidator(ctx, oldValAddr)
	if err != nil {
		return nil, err
	}
	totalRecords := uint64(len(delegations) + len(ubds) + len(reds))
	if totalRecords > params.MaxValidatorDelegations {
		return nil, types.ErrTooManyDelegators.Wrapf(
			"total records %d exceeds max %d", totalRecords, params.MaxValidatorDelegations,
		)
	}

	// Verify both embedded proofs before touching state.
	if err := VerifyLegacySignature(migrationPayloadKindValidator, legacyAddr, newAddr, msg.LegacyPubKey, msg.LegacySignature); err != nil {
		return nil, err
	}
	if err := VerifyNewSignature(migrationPayloadKindValidator, legacyAddr, newAddr, msg.NewPubKey, msg.NewSignature); err != nil {
		return nil, err
	}

	// --- Step V1: Withdraw all commission and delegation rewards ---
	// Must happen before re-keying so rewards accrue to the correct addresses.
	if _, err := ms.distributionKeeper.WithdrawValidatorCommission(ctx, oldValAddr); err != nil {
		// Commission may be zero — that returns an error we can safely ignore.
		_ = err
	}
	// Withdraw every delegator's pending rewards for this validator.
	for _, del := range delegations {
		delAddr, err := sdk.AccAddressFromBech32(del.DelegatorAddress)
		if err != nil {
			return nil, err
		}
		if _, err := ms.distributionKeeper.WithdrawDelegationRewards(ctx, delAddr, oldValAddr); err != nil {
			return nil, fmt.Errorf("withdraw rewards for delegator %s: %w", del.DelegatorAddress, err)
		}
	}

	// --- Step V2: Re-key validator record ---
	if err := ms.MigrateValidatorRecord(ctx, oldValAddr, newValAddr); err != nil {
		return nil, fmt.Errorf("migrate validator record: %w", err)
	}

	// --- Step V3: Re-key distribution state ---
	// Must happen before delegation re-keying because MigrateValidatorDelegations
	// calls GetValidatorCurrentRewards(ctx, newValAddr) to initialize starting info.
	if err := ms.MigrateValidatorDistribution(ctx, oldValAddr, newValAddr); err != nil {
		return nil, fmt.Errorf("migrate validator distribution: %w", err)
	}

	// --- Step V4: Re-key all delegations pointing to this validator ---
	if err := ms.MigrateValidatorDelegations(ctx, oldValAddr, newValAddr); err != nil {
		return nil, fmt.Errorf("migrate validator delegations: %w", err)
	}

	// --- Step V5: Re-key supernode record ---
	if err := ms.MigrateValidatorSupernode(ctx, oldValAddr, newValAddr, legacyAddr, newAddr); err != nil {
		return nil, fmt.Errorf("migrate validator supernode: %w", err)
	}

	// --- Step V6: Update action SuperNodes references ---
	if err := ms.MigrateValidatorActions(ctx, legacyAddr, newAddr); err != nil {
		return nil, fmt.Errorf("migrate validator actions: %w", err)
	}

	// --- Step V7: Account-level migration (shared with MsgClaimLegacyAccount) ---
	// Migrates auth account (vesting-aware), bank balances, authz grants,
	// feegrant allowances, and claim records.

	// Remove legacy auth account; for vesting accounts, extract schedule so it
	// can be re-applied at the new address after the bank transfer.
	vestingInfo, err := ms.MigrateAuth(ctx, legacyAddr, newAddr)
	if err != nil {
		return nil, fmt.Errorf("migrate auth: %w", err)
	}

	// Transfer all bank balances (spendable + locked) from legacy to new address.
	if err := ms.MigrateBank(ctx, legacyAddr, newAddr); err != nil {
		return nil, fmt.Errorf("migrate bank: %w", err)
	}

	// Re-create the vesting account at the new address with the original schedule.
	if vestingInfo != nil {
		if err := ms.FinalizeVestingAccount(ctx, newAddr, vestingInfo); err != nil {
			return nil, fmt.Errorf("finalize vesting: %w", err)
		}
	}

	// Re-key authz grants (both granter and grantee roles).
	if err := ms.MigrateAuthz(ctx, legacyAddr, newAddr); err != nil {
		return nil, fmt.Errorf("migrate authz: %w", err)
	}

	// Re-key feegrant allowances (both granter and grantee roles).
	if err := ms.MigrateFeegrant(ctx, legacyAddr, newAddr); err != nil {
		return nil, fmt.Errorf("migrate feegrant: %w", err)
	}

	// Update claim record destAddress from legacy to new address.
	if err := ms.MigrateClaim(ctx, legacyAddr, newAddr); err != nil {
		return nil, fmt.Errorf("migrate claim: %w", err)
	}

	// --- Step V8: Finalize — store record, increment counters, emit event ---
	if err := ms.finalizeMigration(ctx, legacyAddr, newAddr, true); err != nil {
		return nil, err
	}

	return &types.MsgMigrateValidatorResponse{}, nil
}
