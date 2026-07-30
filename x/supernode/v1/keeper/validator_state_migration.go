package keeper

import (
	"bytes"
	"encoding/json"
	"fmt"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// IdentityMigrationRDistScanLimit bounds the textual rdist namespace scan.
// Build accepts exactly this many rows and rejects the cap+1 row.
const IdentityMigrationRDistScanLimit = 10_000

// BuildIdentityMigrationPlan validates SuperNode ownership integrity and
// snapshots the continuity state owned by this module. Primary records and
// account indexes are validation-only: PR196 owns moving those state families.
// This plan writes only latest metrics and Everlight SNDistState.
func (k Keeper) BuildIdentityMigrationPlan(
	ctx sdk.Context,
	sourceValidator sdk.ValAddress,
	destinationValidator sdk.ValAddress,
) (types.IdentityMigrationPlan, error) {
	if len(sourceValidator) == 0 || len(destinationValidator) == 0 {
		return nil, fmt.Errorf("source and destination validator addresses must be non-empty")
	}
	if sourceValidator.Equals(destinationValidator) {
		return nil, fmt.Errorf("source and destination validator addresses must differ")
	}

	// Own the caller's slice-backed addresses before deriving any plan data.
	sourceValidator = sdk.ValAddress(bytes.Clone(sourceValidator))
	destinationValidator = sdk.ValAddress(bytes.Clone(destinationValidator))
	store := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))

	sourcePrimaryKey := types.GetSupernodeKey(sourceValidator)
	sourceSN, sourceSNFound, err := k.strictReadMigrationSuperNode(store.Get(sourcePrimaryKey), sourceValidator)
	if err != nil {
		return nil, fmt.Errorf("source supernode primary state: %w", err)
	}
	destinationPrimaryKey := types.GetSupernodeKey(destinationValidator)
	_, destinationSNFound, err := k.strictReadMigrationSuperNode(store.Get(destinationPrimaryKey), destinationValidator)
	if err != nil {
		return nil, fmt.Errorf("destination supernode primary state: %w", err)
	}
	if destinationSNFound {
		return nil, fmt.Errorf("destination supernode primary state already exists for %s", destinationValidator)
	}

	if err := k.validateMigrationAccountIndexes(ctx, sourceValidator, destinationValidator, sourceSN, sourceSNFound); err != nil {
		return nil, err
	}

	sourceMetricsKey := types.GetMetricsStateKey(sourceValidator)
	sourceMetricsRaw := bytes.Clone(store.Get(sourceMetricsKey))
	sourceMetrics, sourceMetricsFound, err := k.strictReadMigrationMetrics(sourceMetricsRaw, sourceValidator)
	if err != nil {
		return nil, fmt.Errorf("source metrics state: %w", err)
	}
	destinationMetricsKey := types.GetMetricsStateKey(destinationValidator)
	destinationMetricsRaw := bytes.Clone(store.Get(destinationMetricsKey))
	_, destinationMetricsFound, err := k.strictReadMigrationMetrics(destinationMetricsRaw, destinationValidator)
	if err != nil {
		return nil, fmt.Errorf("destination metrics state: %w", err)
	}
	if destinationMetricsFound {
		return nil, fmt.Errorf("destination metrics state already exists for %s", destinationValidator)
	}

	rdistRows, sourceDistRaw, sourceDistFound, destinationDistFound, err := scanMigrationRDistState(store, sourceValidator, destinationValidator)
	if err != nil {
		return nil, err
	}
	if destinationDistFound {
		return nil, fmt.Errorf("destination distribution state already exists for %s", destinationValidator)
	}

	var movedMetricsRaw []byte
	if sourceMetricsFound {
		sourceMetrics.ValidatorAddress = destinationValidator.String()
		movedMetricsRaw, err = k.cdc.Marshal(&sourceMetrics)
		if err != nil {
			return nil, fmt.Errorf("marshal destination metrics state: %w", err)
		}
	}
	if !sourceDistFound {
		sourceDistRaw = nil
	}
	return types.NewIdentityMigrationPlan(
		sourceValidator, destinationValidator,
		sourceMetricsRaw, destinationMetricsRaw, movedMetricsRaw,
		rdistRows, sourceDistRaw,
	), nil
}

// ApplyIdentityMigrationPlan first revalidates every frozen source/destination
// and bounded-prefix precondition, then performs the captured writes. Thus a
// stale/reused plan fails before any mutation and cannot overwrite a late
// destination collision.
func (k Keeper) ApplyIdentityMigrationPlan(ctx sdk.Context, plan types.IdentityMigrationPlan) error {
	if plan == nil {
		return fmt.Errorf("identity migration plan is nil")
	}
	store := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	for _, expected := range plan.Preconditions() {
		actual := store.Get(expected.Key)
		if !sameMigrationValue(actual, expected.Value) {
			return fmt.Errorf("identity migration plan is stale at key %X", expected.Key)
		}
	}
	for _, expected := range plan.PrefixPreconditions() {
		actual, err := snapshotMigrationPrefix(store, expected.Prefix, IdentityMigrationRDistScanLimit)
		if err != nil {
			return err
		}
		if !equalMigrationRows(actual, expected.Rows) {
			return fmt.Errorf("identity migration plan is stale under prefix %q", expected.Prefix)
		}
	}

	// All reads and comparisons complete before the first write.
	for _, write := range plan.Writes() {
		if write.Value == nil {
			store.Delete(write.Key)
		} else {
			store.Set(write.Key, write.Value)
		}
	}
	return nil
}

func (k Keeper) validateMigrationAccountIndexes(
	ctx sdk.Context,
	sourceValidator, destinationValidator sdk.ValAddress,
	sourceSN types.SuperNode,
	sourceSNFound bool,
) error {
	store := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	iterator := store.Iterator(types.SuperNodeByAccountKey, storetypes.PrefixEndBytes(types.SuperNodeByAccountKey))
	defer func() { _ = iterator.Close() }()

	sourceIndexCount := 0
	for ; iterator.Valid(); iterator.Next() {
		key := iterator.Key()
		if !bytes.HasPrefix(key, types.SuperNodeByAccountKey) {
			return fmt.Errorf("supernode account-index iterator returned key outside prefix: %X", key)
		}
		accountText := key[len(types.SuperNodeByAccountKey):]
		if _, err := sdk.AccAddressFromBech32(string(accountText)); err != nil {
			return fmt.Errorf("invalid supernode account-index key %q: %w", accountText, err)
		}
		validator := iterator.Value()
		if err := sdk.VerifyAddressFormat(validator); err != nil {
			return fmt.Errorf("invalid validator in supernode account index %q: %w", accountText, err)
		}
		if bytes.Equal(validator, destinationValidator) {
			return fmt.Errorf("destination validator has stale supernode account index %q", accountText)
		}
		if bytes.Equal(validator, sourceValidator) {
			sourceIndexCount++
			if !sourceSNFound {
				return fmt.Errorf("source validator has stale supernode account index %q", accountText)
			}
			indexedAccount, err := sdk.AccAddressFromBech32(string(accountText))
			if err != nil {
				return err
			}
			primaryAccount, err := sdk.AccAddressFromBech32(sourceSN.SupernodeAccount)
			if err != nil {
				return fmt.Errorf("source supernode account %q is invalid: %w", sourceSN.SupernodeAccount, err)
			}
			if !indexedAccount.Equals(primaryAccount) || string(accountText) != sourceSN.SupernodeAccount {
				return fmt.Errorf("source supernode account index does not canonically match primary account")
			}
		}
	}
	if err := strictIteratorTerminalError(iterator); err != nil {
		return fmt.Errorf("iterate supernode account-index records: %w", err)
	}
	if sourceSNFound {
		if _, err := sdk.AccAddressFromBech32(sourceSN.SupernodeAccount); err != nil {
			return fmt.Errorf("source supernode account %q is invalid: %w", sourceSN.SupernodeAccount, err)
		}
		if sourceIndexCount != 1 {
			return fmt.Errorf("source supernode primary requires exactly one canonical account index, got %d", sourceIndexCount)
		}
		// Also prove no second primary claims the same canonical account.
		if _, found, err := k.StrictGetSuperNodeByAccount(ctx, sourceSN.SupernodeAccount); err != nil {
			return fmt.Errorf("source supernode account ownership: %w", err)
		} else if !found {
			return fmt.Errorf("source supernode account index is absent")
		}
	}
	return nil
}

func scanMigrationRDistState(
	store storetypes.KVStore,
	sourceValidator, destinationValidator sdk.ValAddress,
) (rows []types.IdentityMigrationRow, sourceRaw []byte, sourceFound, destinationFound bool, err error) {
	rows, err = snapshotMigrationPrefix(store, types.SNDistStatePrefix, IdentityMigrationRDistScanLimit)
	if err != nil {
		return nil, nil, false, false, err
	}
	for _, row := range rows {
		suffix := row.Key[len(types.SNDistStatePrefix):]
		validator, parseErr := sdk.ValAddressFromBech32(string(suffix))
		if parseErr != nil {
			return nil, nil, false, false, fmt.Errorf("malformed rdist validator suffix %q: %w", suffix, parseErr)
		}
		isSource := validator.Equals(sourceValidator)
		isDestination := validator.Equals(destinationValidator)
		if (isSource || isDestination) && string(suffix) != validator.String() {
			return nil, nil, false, false, fmt.Errorf("non-canonical rdist validator suffix %q", suffix)
		}
		if _, _, readErr := strictReadMigrationDistState(row.Value); readErr != nil {
			return nil, nil, false, false, fmt.Errorf("distribution state %q: %w", suffix, readErr)
		}
		if isSource {
			if sourceFound {
				return nil, nil, false, false, fmt.Errorf("duplicate source distribution state for %s", sourceValidator)
			}
			sourceFound = true
			sourceRaw = bytes.Clone(row.Value)
		}
		if isDestination {
			if destinationFound {
				return nil, nil, false, false, fmt.Errorf("duplicate destination distribution state for %s", destinationValidator)
			}
			destinationFound = true
		}
	}
	return rows, sourceRaw, sourceFound, destinationFound, nil
}

func snapshotMigrationPrefix(store storetypes.KVStore, prefix []byte, limit int) ([]types.IdentityMigrationRow, error) {
	iterator := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer func() { _ = iterator.Close() }()
	rows := make([]types.IdentityMigrationRow, 0)
	for ; iterator.Valid(); iterator.Next() {
		if len(rows) == limit {
			return nil, fmt.Errorf("identity migration prefix %q exceeds scan limit %d", prefix, limit)
		}
		rows = append(rows, types.IdentityMigrationRow{Key: bytes.Clone(iterator.Key()), Value: bytes.Clone(iterator.Value())})
	}
	if err := strictIteratorTerminalError(iterator); err != nil {
		return nil, fmt.Errorf("iterate identity migration prefix %q: %w", prefix, err)
	}
	return rows, nil
}

func sameMigrationValue(actual, expected []byte) bool {
	return (actual == nil) == (expected == nil) && bytes.Equal(actual, expected)
}

func equalMigrationRows(actual, expected []types.IdentityMigrationRow) bool {
	if len(actual) != len(expected) {
		return false
	}
	for i := range actual {
		if !bytes.Equal(actual[i].Key, expected[i].Key) || !sameMigrationValue(actual[i].Value, expected[i].Value) {
			return false
		}
	}
	return true
}

func (k Keeper) strictReadMigrationSuperNode(raw []byte, expectedValidator sdk.ValAddress) (types.SuperNode, bool, error) {
	if raw == nil {
		return types.SuperNode{}, false, nil
	}
	var sn types.SuperNode
	if err := k.cdc.Unmarshal(raw, &sn); err != nil {
		return types.SuperNode{}, false, fmt.Errorf("malformed row: %w", err)
	}
	embeddedValidator, err := sdk.ValAddressFromBech32(sn.ValidatorAddress)
	if err != nil {
		return types.SuperNode{}, false, fmt.Errorf("invalid embedded validator %q: %w", sn.ValidatorAddress, err)
	}
	if !embeddedValidator.Equals(expectedValidator) {
		return types.SuperNode{}, false, fmt.Errorf("embedded validator mismatch: got %s, expected %s", sn.ValidatorAddress, expectedValidator)
	}
	return sn, true, nil
}

func (k Keeper) strictReadMigrationMetrics(raw []byte, expectedValidator sdk.ValAddress) (types.SupernodeMetricsState, bool, error) {
	if raw == nil {
		return types.SupernodeMetricsState{}, false, nil
	}
	var state types.SupernodeMetricsState
	if err := k.cdc.Unmarshal(raw, &state); err != nil {
		return types.SupernodeMetricsState{}, false, fmt.Errorf("malformed row: %w", err)
	}
	embeddedValidator, err := sdk.ValAddressFromBech32(state.ValidatorAddress)
	if err != nil {
		return types.SupernodeMetricsState{}, false, fmt.Errorf("invalid embedded validator %q: %w", state.ValidatorAddress, err)
	}
	if !embeddedValidator.Equals(expectedValidator) {
		return types.SupernodeMetricsState{}, false, fmt.Errorf("embedded validator mismatch: got %s, expected %s", state.ValidatorAddress, expectedValidator)
	}
	return state, true, nil
}

func strictReadMigrationDistState(raw []byte) ([]byte, bool, error) {
	if raw == nil {
		return nil, false, nil
	}
	var state *types.SNDistState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, false, fmt.Errorf("malformed row: %w", err)
	}
	if state == nil {
		return nil, false, fmt.Errorf("malformed row: null distribution state")
	}
	return bytes.Clone(raw), true, nil
}
