package keeper

import (
	"context"
	"fmt"
	"strconv"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// ClaimLegacyAccount migrates on-chain state from a legacy (coin-type-118)
// address to a new (coin-type-60) address. Authentication is fully embedded in
// the message: the legacy key authorizes the migration and the destination key
// authorizes receiving the migrated state.
func (ms msgServer) ClaimLegacyAccount(goCtx context.Context, msg *types.MsgClaimLegacyAccount) (*types.MsgClaimLegacyAccountResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	legacyAddr, err := sdk.AccAddressFromBech32(msg.LegacyAddress)
	if err != nil {
		return nil, err
	}
	newAddr, err := sdk.AccAddressFromBech32(msg.NewAddress)
	if err != nil {
		return nil, err
	}

	// --- Pre-checks ---
	if err := ms.preChecks(ctx, legacyAddr, newAddr); err != nil {
		return nil, err
	}

	// Check: legacy address must NOT be a validator operator.
	oldValAddr := sdk.ValAddress(legacyAddr)
	if _, err := ms.stakingKeeper.GetValidator(ctx, oldValAddr); err == nil {
		return nil, types.ErrUseValidatorMigration
	}

	// Verify both embedded proofs before touching state.
	if err := VerifyLegacySignature(migrationPayloadKindClaim, legacyAddr, newAddr, msg.LegacyPubKey, msg.LegacySignature); err != nil {
		return nil, err
	}
	if err := VerifyNewSignature(migrationPayloadKindClaim, legacyAddr, newAddr, msg.NewPubKey, msg.NewSignature); err != nil {
		return nil, err
	}

	// --- Execute migration steps ---
	if err := ms.migrateAccount(ctx, legacyAddr, newAddr); err != nil {
		return nil, err
	}

	// --- Finalize ---
	if err := ms.finalizeMigration(ctx, legacyAddr, newAddr, false); err != nil {
		return nil, err
	}

	return &types.MsgClaimLegacyAccountResponse{}, nil
}

// preChecks performs the common pre-check sequence shared by both
// ClaimLegacyAccount and MigrateValidator.
func (ms msgServer) preChecks(ctx sdk.Context, legacyAddr, newAddr sdk.AccAddress) error {
	// 1. Migration enabled
	params, err := ms.Params.Get(ctx)
	if err != nil {
		return err
	}
	if !params.EnableMigration {
		return types.ErrMigrationDisabled
	}

	// 2. Migration window
	if params.MigrationEndTime > 0 {
		endTime := time.Unix(params.MigrationEndTime, 0)
		if ctx.BlockTime().After(endTime) {
			return types.ErrMigrationWindowClosed
		}
	}

	// 3. Block rate limit
	blockHeight := ctx.BlockHeight()
	blockCount, err := ms.BlockMigrationCounter.Get(ctx, blockHeight)
	if err != nil {
		blockCount = 0
	}
	if blockCount >= params.MaxMigrationsPerBlock {
		return types.ErrBlockRateLimitExceeded
	}

	// 4. Addresses must differ
	if legacyAddr.Equals(newAddr) {
		return types.ErrSameAddress
	}

	// 5. Legacy address not already migrated
	has, err := ms.MigrationRecords.Has(ctx, legacyAddr.String())
	if err != nil {
		return err
	}
	if has {
		return types.ErrAlreadyMigrated
	}

	// 6. New address must not be a previously-migrated legacy address
	has, err = ms.MigrationRecords.Has(ctx, newAddr.String())
	if err != nil {
		return err
	}
	if has {
		return types.ErrNewAddressWasMigrated
	}

	// 7. Legacy address must not be a module account
	legacyAcc := ms.accountKeeper.GetAccount(ctx, legacyAddr)
	if legacyAcc == nil {
		return types.ErrLegacyAccountNotFound
	}
	if _, ok := legacyAcc.(sdk.ModuleAccountI); ok {
		return types.ErrCannotMigrateModuleAccount
	}

	return nil
}

// migrateAccount performs the account-level migration steps shared by both
// ClaimLegacyAccount and MigrateValidator (Steps 1-8 from the plan).
func (ms msgServer) migrateAccount(ctx sdk.Context, legacyAddr, newAddr sdk.AccAddress) error {
	// Step 1: Withdraw distribution rewards.
	if err := ms.MigrateDistribution(ctx, legacyAddr); err != nil {
		return fmt.Errorf("migrate distribution: %w", err)
	}

	// Step 2: Re-key staking (delegations, unbonding, redelegations).
	if err := ms.MigrateStaking(ctx, legacyAddr, newAddr); err != nil {
		return fmt.Errorf("migrate staking: %w", err)
	}

	// Step 3a: Migrate auth account (vesting-aware: remove lock before bank transfer).
	vestingInfo, err := ms.MigrateAuth(ctx, legacyAddr, newAddr)
	if err != nil {
		return fmt.Errorf("migrate auth: %w", err)
	}

	// Step 3b: Transfer bank balances.
	if err := ms.MigrateBank(ctx, legacyAddr, newAddr); err != nil {
		return fmt.Errorf("migrate bank: %w", err)
	}

	// Step 3c: Finalize vesting account at new address (if applicable).
	if vestingInfo != nil {
		if err := ms.FinalizeVestingAccount(ctx, newAddr, vestingInfo); err != nil {
			return fmt.Errorf("finalize vesting: %w", err)
		}
	}

	// Step 4: Re-key authz grants.
	if err := ms.MigrateAuthz(ctx, legacyAddr, newAddr); err != nil {
		return fmt.Errorf("migrate authz: %w", err)
	}

	// Step 5: Re-key feegrant allowances.
	if err := ms.MigrateFeegrant(ctx, legacyAddr, newAddr); err != nil {
		return fmt.Errorf("migrate feegrant: %w", err)
	}

	// Step 6: Update supernode account field.
	if err := ms.MigrateSupernode(ctx, legacyAddr, newAddr); err != nil {
		return fmt.Errorf("migrate supernode: %w", err)
	}

	// Step 7: Update action creator/supernode references.
	if err := ms.MigrateActions(ctx, legacyAddr, newAddr); err != nil {
		return fmt.Errorf("migrate actions: %w", err)
	}

	// Step 8: Update claim destAddress.
	if err := ms.MigrateClaim(ctx, legacyAddr, newAddr); err != nil {
		return fmt.Errorf("migrate claim: %w", err)
	}

	return nil
}

// finalizeMigration stores the migration record, increments counters, and emits events.
func (ms msgServer) finalizeMigration(ctx sdk.Context, legacyAddr, newAddr sdk.AccAddress, isValidator bool) error {
	record := types.MigrationRecord{
		LegacyAddress:   legacyAddr.String(),
		NewAddress:      newAddr.String(),
		MigrationTime:   ctx.BlockTime().Unix(),
		MigrationHeight: ctx.BlockHeight(),
	}

	if err := ms.MigrationRecords.Set(ctx, legacyAddr.String(), record); err != nil {
		return err
	}

	// Increment global counter.
	count, err := ms.MigrationCounter.Get(ctx)
	if err != nil {
		count = 0
	}
	if err := ms.MigrationCounter.Set(ctx, count+1); err != nil {
		return err
	}

	// Increment block counter.
	blockCount, err := ms.BlockMigrationCounter.Get(ctx, ctx.BlockHeight())
	if err != nil {
		blockCount = 0
	}
	if err := ms.BlockMigrationCounter.Set(ctx, ctx.BlockHeight(), blockCount+1); err != nil {
		return err
	}

	// Increment validator counter if applicable.
	if isValidator {
		valCount, err := ms.ValidatorMigrationCounter.Get(ctx)
		if err != nil {
			valCount = 0
		}
		if err := ms.ValidatorMigrationCounter.Set(ctx, valCount+1); err != nil {
			return err
		}
	}

	// Emit event.
	eventType := types.EventTypeClaimLegacyAccount
	if isValidator {
		eventType = types.EventTypeMigrateValidator
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			eventType,
			sdk.NewAttribute(types.AttributeKeyLegacyAddress, legacyAddr.String()),
			sdk.NewAttribute(types.AttributeKeyNewAddress, newAddr.String()),
			sdk.NewAttribute(types.AttributeKeyMigrationTime, strconv.FormatInt(ctx.BlockTime().Unix(), 10)),
			sdk.NewAttribute(types.AttributeKeyBlockHeight, strconv.FormatInt(ctx.BlockHeight(), 10)),
		),
	)

	return nil
}
