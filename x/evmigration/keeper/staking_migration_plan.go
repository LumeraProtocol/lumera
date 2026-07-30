package keeper

import (
	"bytes"
	"fmt"
	"sort"
	"time"

	corestore "cosmossdk.io/core/store"
	sdk "github.com/cosmos/cosmos-sdk/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// MaxAccountStakingMigrationRecords bounds each account migration's staking
// work. It is deliberately consensus-static: changing it requires a software
// upgrade and cannot accidentally make a governance parameter exceed the
// uint16-bounded staking keeper query API.
const MaxAccountStakingMigrationRecords uint16 = 2500

type stakingQueueWrite struct {
	key      []byte
	expected []byte
	value    []byte
}

type stakingStorePrecondition struct {
	key         []byte
	expected    []byte // nil means the key must remain absent
	description string
}

type ubdMigration struct {
	old []byte
	new []byte
}

type redMigration struct {
	old []byte
	new []byte
}

type delegationMigration struct {
	old, new                   []byte
	oldDelegator, newDelegator sdk.AccAddress
	oldValidator, newValidator sdk.ValAddress
}

// stakingMigrationPlan is a fully marshalled, immutable description of the
// UBD/RED primary and maturity-queue changes. No staking write is performed
// while it is built.
type stakingMigrationPlan struct {
	delegations   []delegationMigration
	ubds          []ubdMigration
	reds          []redMigration
	queueWrites   []stakingQueueWrite
	preconditions []stakingStorePrecondition
}

type stakingAddressTransform struct {
	oldDelegator sdk.AccAddress
	newDelegator sdk.AccAddress
	oldValidator sdk.ValAddress
	newValidator sdk.ValAddress
}

func (x stakingAddressTransform) delegation(old stakingtypes.Delegation) stakingtypes.Delegation {
	out := old
	if x.oldDelegator != nil && old.DelegatorAddress == x.oldDelegator.String() {
		out.DelegatorAddress = x.newDelegator.String()
	}
	if x.oldValidator != nil && old.ValidatorAddress == x.oldValidator.String() {
		out.ValidatorAddress = x.newValidator.String()
	}
	return out
}

func (x stakingAddressTransform) ubd(old stakingtypes.UnbondingDelegation) stakingtypes.UnbondingDelegation {
	out := old
	if x.oldDelegator != nil && old.DelegatorAddress == x.oldDelegator.String() {
		out.DelegatorAddress = x.newDelegator.String()
	}
	if x.oldValidator != nil && old.ValidatorAddress == x.oldValidator.String() {
		out.ValidatorAddress = x.newValidator.String()
	}
	return out
}

func (x stakingAddressTransform) red(old stakingtypes.Redelegation) stakingtypes.Redelegation {
	out := old
	if x.oldDelegator != nil && old.DelegatorAddress == x.oldDelegator.String() {
		out.DelegatorAddress = x.newDelegator.String()
	}
	if x.oldValidator != nil {
		if old.ValidatorSrcAddress == x.oldValidator.String() {
			out.ValidatorSrcAddress = x.newValidator.String()
		}
		if old.ValidatorDstAddress == x.oldValidator.String() {
			out.ValidatorDstAddress = x.newValidator.String()
		}
	}
	return out
}

func (k Keeper) accountStakingRecords(ctx sdk.Context, delegator sdk.AccAddress) (
	[]stakingtypes.Delegation, []stakingtypes.UnbondingDelegation, []stakingtypes.Redelegation, error,
) {
	// Query the keeper's full uint16 range, then enforce our smaller static cap.
	// This preserves exact cap/cap+1 detection across the combined record kinds.
	limit := ^uint16(0)
	delegations, err := k.stakingKeeper.GetDelegatorDelegations(ctx, delegator, limit)
	if err != nil {
		return nil, nil, nil, err
	}
	ubds, err := k.stakingKeeper.GetUnbondingDelegations(ctx, delegator, limit)
	if err != nil {
		return nil, nil, nil, err
	}
	reds, err := k.stakingKeeper.GetRedelegations(ctx, delegator, limit)
	if err != nil {
		return nil, nil, nil, err
	}
	count := len(delegations) + len(ubds) + len(reds)
	if count > int(MaxAccountStakingMigrationRecords) {
		return nil, nil, nil, fmt.Errorf("account staking records %d exceed migration cap %d", count, MaxAccountStakingMigrationRecords)
	}
	return delegations, ubds, reds, nil
}

func (k Keeper) buildStakingMigrationPlan(
	ctx sdk.Context,
	delegations []stakingtypes.Delegation,
	ubds []stakingtypes.UnbondingDelegation,
	reds []stakingtypes.Redelegation,
	x stakingAddressTransform,
) (stakingMigrationPlan, error) {
	var plan stakingMigrationPlan
	if len(delegations) == 0 && len(ubds) == 0 && len(reds) == 0 {
		return plan, nil
	}
	if k.stakingStoreHandle == nil || k.stakingStoreHandle.svc == nil {
		return plan, fmt.Errorf("staking migration plan: staking store service not wired")
	}
	store := k.stakingStoreHandle.svc.OpenKVStore(ctx)

	delegationSeen := make(map[string][]byte, len(delegations))
	delegationDestinations := make(map[string]struct{}, len(delegations))
	ubdSeen := make(map[string]struct{}, len(ubds))
	redSeen := make(map[string]struct{}, len(reds))
	ubdSnapshots := make(map[string][]byte, len(ubds))
	redSnapshots := make(map[string][]byte, len(reds))
	ubdDestinations := make(map[string]struct{}, len(ubds))
	redDestinations := make(map[string]struct{}, len(reds))
	ubdSubs := make(map[string][]pairSubstitution)
	redSubs := make(map[string][]tripletSubstitution)

	for _, old := range delegations {
		newRecord := x.delegation(old)
		if old.DelegatorAddress == newRecord.DelegatorAddress && old.ValidatorAddress == newRecord.ValidatorAddress {
			continue
		}
		oldDel, err := sdk.AccAddressFromBech32(old.DelegatorAddress)
		if err != nil {
			return plan, fmt.Errorf("malformed delegation delegator: %w", err)
		}
		oldVal, err := sdk.ValAddressFromBech32(old.ValidatorAddress)
		if err != nil {
			return plan, fmt.Errorf("malformed delegation validator: %w", err)
		}
		newDel, err := sdk.AccAddressFromBech32(newRecord.DelegatorAddress)
		if err != nil {
			return plan, fmt.Errorf("malformed destination delegation delegator: %w", err)
		}
		newVal, err := sdk.ValAddressFromBech32(newRecord.ValidatorAddress)
		if err != nil {
			return plan, fmt.Errorf("malformed destination delegation validator: %w", err)
		}
		oldKey := stakingtypes.GetDelegationKey(oldDel, oldVal)
		oldBytes := stakingtypes.MustMarshalDelegation(k.cdc, old)
		if snapshot, duplicate := delegationSeen[string(oldKey)]; duplicate {
			if !bytes.Equal(snapshot, oldBytes) {
				return plan, fmt.Errorf("conflicting duplicate delegation snapshot for primary %X", oldKey)
			}
			continue
		}
		delegationSeen[string(oldKey)] = append([]byte(nil), oldBytes...)
		if err := requirePrimaryMatches(store, oldKey, oldBytes, "delegation"); err != nil {
			return plan, err
		}
		newKey := stakingtypes.GetDelegationKey(newDel, newVal)
		if bytes.Equal(oldKey, newKey) {
			return plan, fmt.Errorf("delegation transformation did not change canonical key %X", oldKey)
		}
		if err := requireAbsent(store, newKey, "delegation"); err != nil {
			return plan, err
		}
		if _, duplicate := delegationDestinations[string(newKey)]; duplicate {
			return plan, fmt.Errorf("multiple delegations map to destination primary %X", newKey)
		}
		delegationDestinations[string(newKey)] = struct{}{}
		hasStartingInfo, err := k.distributionKeeper.HasDelegatorStartingInfo(ctx, newVal, newDel)
		if err != nil {
			return plan, fmt.Errorf("check destination delegation starting info: %w", err)
		}
		if hasStartingInfo {
			return plan, fmt.Errorf("destination delegation starting info already exists for %s/%s", newVal, newDel)
		}
		newBytes := stakingtypes.MustMarshalDelegation(k.cdc, newRecord)
		plan.preconditions = append(plan.preconditions,
			stakingStorePrecondition{key: append([]byte(nil), oldKey...), expected: append([]byte(nil), oldBytes...), description: "source delegation primary"},
			stakingStorePrecondition{key: append([]byte(nil), newKey...), description: "destination delegation primary"},
		)
		plan.delegations = append(plan.delegations, delegationMigration{
			old: append([]byte(nil), oldBytes...), new: append([]byte(nil), newBytes...),
			oldDelegator: append(sdk.AccAddress(nil), oldDel...), newDelegator: append(sdk.AccAddress(nil), newDel...),
			oldValidator: append(sdk.ValAddress(nil), oldVal...), newValidator: append(sdk.ValAddress(nil), newVal...),
		})
	}

	for _, old := range ubds {
		newRecord := x.ubd(old)
		if old.DelegatorAddress == newRecord.DelegatorAddress && old.ValidatorAddress == newRecord.ValidatorAddress {
			continue
		}
		oldDel, oldVal, newDel, newVal, err := parseUBDAddresses(old, newRecord)
		if err != nil {
			return plan, err
		}
		oldKey := stakingtypes.GetUBDKey(oldDel, oldVal)
		oldBytes := stakingtypes.MustMarshalUBD(k.cdc, old)
		if _, duplicate := ubdSeen[string(oldKey)]; duplicate {
			if !bytes.Equal(ubdSnapshots[string(oldKey)], oldBytes) {
				return plan, fmt.Errorf("conflicting duplicate UBD snapshot for primary %X", oldKey)
			}
			continue // exact union duplicate from account- and validator-scoped enumerations
		}
		ubdSeen[string(oldKey)] = struct{}{}
		ubdSnapshots[string(oldKey)] = oldBytes
		if err := requirePrimaryMatches(store, oldKey, oldBytes, "unbonding delegation"); err != nil {
			return plan, err
		}
		newKey := stakingtypes.GetUBDKey(newDel, newVal)
		if bytes.Equal(oldKey, newKey) {
			return plan, fmt.Errorf("UBD transformation did not change canonical key %X", oldKey)
		}
		if err := requireAbsent(store, newKey, "unbonding delegation"); err != nil {
			return plan, err
		}
		plan.preconditions = append(plan.preconditions,
			stakingStorePrecondition{key: append([]byte(nil), oldKey...), expected: append([]byte(nil), oldBytes...), description: "source unbonding delegation primary"},
			stakingStorePrecondition{key: append([]byte(nil), newKey...), description: "destination unbonding delegation primary"},
		)
		if _, duplicate := ubdDestinations[string(newKey)]; duplicate {
			return plan, fmt.Errorf("multiple UBD records map to destination primary %X", newKey)
		}
		ubdDestinations[string(newKey)] = struct{}{}
		newBytes := stakingtypes.MustMarshalUBD(k.cdc, newRecord)
		plan.ubds = append(plan.ubds, ubdMigration{
			old: append([]byte(nil), oldBytes...), new: append([]byte(nil), newBytes...),
		})
		oldPair := stakingtypes.DVPair{DelegatorAddress: old.DelegatorAddress, ValidatorAddress: old.ValidatorAddress}
		newPair := stakingtypes.DVPair{DelegatorAddress: newRecord.DelegatorAddress, ValidatorAddress: newRecord.ValidatorAddress}
		for _, completion := range uniqueUBDCompletionTimes(old.Entries) {
			key := stakingtypes.GetUnbondingDelegationTimeKey(completion)
			ubdSubs[string(key)] = append(ubdSubs[string(key)], pairSubstitution{old: oldPair, new: newPair})
		}
	}

	for _, old := range reds {
		newRecord := x.red(old)
		if old.DelegatorAddress == newRecord.DelegatorAddress && old.ValidatorSrcAddress == newRecord.ValidatorSrcAddress && old.ValidatorDstAddress == newRecord.ValidatorDstAddress {
			continue
		}
		oldDel, oldSrc, oldDst, newDel, newSrc, newDst, err := parseREDAddresses(old, newRecord)
		if err != nil {
			return plan, err
		}
		oldKey := stakingtypes.GetREDKey(oldDel, oldSrc, oldDst)
		oldBytes := stakingtypes.MustMarshalRED(k.cdc, old)
		if _, duplicate := redSeen[string(oldKey)]; duplicate {
			if !bytes.Equal(redSnapshots[string(oldKey)], oldBytes) {
				return plan, fmt.Errorf("conflicting duplicate RED snapshot for primary %X", oldKey)
			}
			continue
		}
		redSeen[string(oldKey)] = struct{}{}
		redSnapshots[string(oldKey)] = oldBytes
		if err := requirePrimaryMatches(store, oldKey, oldBytes, "redelegation"); err != nil {
			return plan, err
		}
		newKey := stakingtypes.GetREDKey(newDel, newSrc, newDst)
		if bytes.Equal(oldKey, newKey) {
			return plan, fmt.Errorf("redelegation transformation did not change canonical key %X", oldKey)
		}
		if err := requireAbsent(store, newKey, "redelegation"); err != nil {
			return plan, err
		}
		plan.preconditions = append(plan.preconditions,
			stakingStorePrecondition{key: append([]byte(nil), oldKey...), expected: append([]byte(nil), oldBytes...), description: "source redelegation primary"},
			stakingStorePrecondition{key: append([]byte(nil), newKey...), description: "destination redelegation primary"},
		)
		if _, duplicate := redDestinations[string(newKey)]; duplicate {
			return plan, fmt.Errorf("multiple RED records map to destination primary %X", newKey)
		}
		redDestinations[string(newKey)] = struct{}{}
		newBytes := stakingtypes.MustMarshalRED(k.cdc, newRecord)
		plan.reds = append(plan.reds, redMigration{
			old: append([]byte(nil), oldBytes...), new: append([]byte(nil), newBytes...),
		})
		oldTriplet := stakingtypes.DVVTriplet{DelegatorAddress: old.DelegatorAddress, ValidatorSrcAddress: old.ValidatorSrcAddress, ValidatorDstAddress: old.ValidatorDstAddress}
		newTriplet := stakingtypes.DVVTriplet{DelegatorAddress: newRecord.DelegatorAddress, ValidatorSrcAddress: newRecord.ValidatorSrcAddress, ValidatorDstAddress: newRecord.ValidatorDstAddress}
		for _, completion := range uniqueREDCompletionTimes(old.Entries) {
			key := stakingtypes.GetRedelegationTimeKey(completion)
			redSubs[string(key)] = append(redSubs[string(key)], tripletSubstitution{old: oldTriplet, new: newTriplet})
		}
	}

	writes, err := k.buildQueueWrites(store, ubdSubs, redSubs)
	if err != nil {
		return stakingMigrationPlan{}, err
	}
	plan.queueWrites = writes
	return plan, nil
}

func requireAbsent(store corestore.KVStore, key []byte, kind string) error {
	bz, err := store.Get(key)
	if err != nil {
		return err
	}
	if bz != nil {
		return fmt.Errorf("destination %s primary already exists at %X", kind, key)
	}
	return nil
}

func requirePrimaryMatches(store corestore.KVStore, key, expected []byte, kind string) error {
	bz, err := store.Get(key)
	if err != nil {
		return err
	}
	if bz == nil {
		return fmt.Errorf("source %s primary missing at %X", kind, key)
	}
	if !bytes.Equal(bz, expected) {
		return fmt.Errorf("source %s primary at %X does not match preloaded snapshot", kind, key)
	}
	return nil
}

func parseUBDAddresses(old, newRecord stakingtypes.UnbondingDelegation) (sdk.AccAddress, sdk.ValAddress, sdk.AccAddress, sdk.ValAddress, error) {
	od, err := sdk.AccAddressFromBech32(old.DelegatorAddress)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("malformed UBD delegator: %w", err)
	}
	ov, err := sdk.ValAddressFromBech32(old.ValidatorAddress)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("malformed UBD validator: %w", err)
	}
	nd, err := sdk.AccAddressFromBech32(newRecord.DelegatorAddress)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("malformed destination UBD delegator: %w", err)
	}
	nv, err := sdk.ValAddressFromBech32(newRecord.ValidatorAddress)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("malformed destination UBD validator: %w", err)
	}
	return od, ov, nd, nv, nil
}

func parseREDAddresses(old, newRecord stakingtypes.Redelegation) (sdk.AccAddress, sdk.ValAddress, sdk.ValAddress, sdk.AccAddress, sdk.ValAddress, sdk.ValAddress, error) {
	od, err := sdk.AccAddressFromBech32(old.DelegatorAddress)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("malformed RED delegator: %w", err)
	}
	os, err := sdk.ValAddressFromBech32(old.ValidatorSrcAddress)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("malformed RED source: %w", err)
	}
	oz, err := sdk.ValAddressFromBech32(old.ValidatorDstAddress)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("malformed RED destination: %w", err)
	}
	nd, err := sdk.AccAddressFromBech32(newRecord.DelegatorAddress)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	ns, err := sdk.ValAddressFromBech32(newRecord.ValidatorSrcAddress)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	nz, err := sdk.ValAddressFromBech32(newRecord.ValidatorDstAddress)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	return od, os, oz, nd, ns, nz, nil
}

func uniqueUBDCompletionTimes(entries []stakingtypes.UnbondingDelegationEntry) []time.Time {
	seen := map[int64]bool{}
	out := make([]time.Time, 0, len(entries))
	for _, e := range entries {
		n := e.CompletionTime.UnixNano()
		if !seen[n] {
			seen[n] = true
			out = append(out, e.CompletionTime)
		}
	}
	return out
}
func uniqueREDCompletionTimes(entries []stakingtypes.RedelegationEntry) []time.Time {
	seen := map[int64]bool{}
	out := make([]time.Time, 0, len(entries))
	for _, e := range entries {
		n := e.CompletionTime.UnixNano()
		if !seen[n] {
			seen[n] = true
			out = append(out, e.CompletionTime)
		}
	}
	return out
}

type pairSubstitution struct{ old, new stakingtypes.DVPair }
type tripletSubstitution struct{ old, new stakingtypes.DVVTriplet }

func (k Keeper) buildQueueWrites(store corestore.KVStore, pairs map[string][]pairSubstitution, triplets map[string][]tripletSubstitution) ([]stakingQueueWrite, error) {
	writes := make([]stakingQueueWrite, 0, len(pairs)+len(triplets))
	pairKeys := make([]string, 0, len(pairs))
	for key := range pairs {
		pairKeys = append(pairKeys, key)
	}
	sort.Strings(pairKeys)
	for _, keyString := range pairKeys {
		substitutions := pairs[keyString]
		key := []byte(keyString)
		bz, err := store.Get(key)
		if err != nil {
			return nil, err
		}
		if bz == nil {
			return nil, fmt.Errorf("missing UBD queue timeslice %X", key)
		}
		var slice stakingtypes.DVPairs
		if err := k.cdc.Unmarshal(bz, &slice); err != nil {
			return nil, fmt.Errorf("malformed UBD queue timeslice %X: %w", key, err)
		}
		for _, sub := range substitutions {
			oldCount, newCount := 0, 0
			for i := range slice.Pairs {
				if equalDVPair(slice.Pairs[i], sub.old) {
					oldCount++
				}
				if equalDVPair(slice.Pairs[i], sub.new) {
					newCount++
				}
			}
			if oldCount != 1 || newCount != 0 {
				return nil, fmt.Errorf("unsafe UBD queue timeslice %X: old tuple count %d, new tuple count %d", key, oldCount, newCount)
			}
			for i := range slice.Pairs {
				if equalDVPair(slice.Pairs[i], sub.old) {
					slice.Pairs[i] = sub.new
				}
			}
		}
		out, err := k.cdc.Marshal(&slice)
		if err != nil {
			return nil, err
		}
		writes = append(writes, stakingQueueWrite{key: append([]byte(nil), key...), expected: append([]byte(nil), bz...), value: out})
	}
	tripletKeys := make([]string, 0, len(triplets))
	for key := range triplets {
		tripletKeys = append(tripletKeys, key)
	}
	sort.Strings(tripletKeys)
	for _, keyString := range tripletKeys {
		substitutions := triplets[keyString]
		key := []byte(keyString)
		bz, err := store.Get(key)
		if err != nil {
			return nil, err
		}
		if bz == nil {
			return nil, fmt.Errorf("missing RED queue timeslice %X", key)
		}
		var slice stakingtypes.DVVTriplets
		if err := k.cdc.Unmarshal(bz, &slice); err != nil {
			return nil, fmt.Errorf("malformed RED queue timeslice %X: %w", key, err)
		}
		for _, sub := range substitutions {
			oldCount, newCount := 0, 0
			for i := range slice.Triplets {
				if equalDVVTriplet(slice.Triplets[i], sub.old) {
					oldCount++
				}
				if equalDVVTriplet(slice.Triplets[i], sub.new) {
					newCount++
				}
			}
			if oldCount != 1 || newCount != 0 {
				return nil, fmt.Errorf("unsafe RED queue timeslice %X: old tuple count %d, new tuple count %d", key, oldCount, newCount)
			}
			for i := range slice.Triplets {
				if equalDVVTriplet(slice.Triplets[i], sub.old) {
					slice.Triplets[i] = sub.new
				}
			}
		}
		out, err := k.cdc.Marshal(&slice)
		if err != nil {
			return nil, err
		}
		writes = append(writes, stakingQueueWrite{key: append([]byte(nil), key...), expected: append([]byte(nil), bz...), value: out})
	}
	return writes, nil
}

func equalDVPair(a, b stakingtypes.DVPair) bool {
	return a.DelegatorAddress == b.DelegatorAddress && a.ValidatorAddress == b.ValidatorAddress
}

func equalDVVTriplet(a, b stakingtypes.DVVTriplet) bool {
	return a.DelegatorAddress == b.DelegatorAddress &&
		a.ValidatorSrcAddress == b.ValidatorSrcAddress &&
		a.ValidatorDstAddress == b.ValidatorDstAddress
}

func (k Keeper) applyStakingMigrationPlan(ctx sdk.Context, plan stakingMigrationPlan, applyDelegations bool) error {
	if len(plan.queueWrites) == 0 && len(plan.delegations) == 0 && len(plan.ubds) == 0 && len(plan.reds) == 0 {
		return nil
	}
	if k.stakingStoreHandle == nil || k.stakingStoreHandle.svc == nil {
		return fmt.Errorf("apply staking migration plan: staking store service not wired")
	}
	store := k.stakingStoreHandle.svc.OpenKVStore(ctx)
	// Revalidate every raw-store and cross-module assumption before the first
	// write. Reward withdrawal may legitimately replace source starting info,
	// but destination starting info must remain absent.
	for _, condition := range plan.preconditions {
		bz, err := store.Get(condition.key)
		if err != nil {
			return err
		}
		if !bytes.Equal(bz, condition.expected) {
			return fmt.Errorf("%s at %X changed after preflight", condition.description, condition.key)
		}
	}
	for _, write := range plan.queueWrites {
		bz, err := store.Get(write.key)
		if err != nil {
			return err
		}
		if !bytes.Equal(bz, write.expected) {
			return fmt.Errorf("staking queue timeslice %X changed after preflight", write.key)
		}
	}
	for _, change := range plan.delegations {
		has, err := k.distributionKeeper.HasDelegatorStartingInfo(ctx, change.newValidator, change.newDelegator)
		if err != nil {
			return fmt.Errorf("recheck destination delegation starting info: %w", err)
		}
		if has {
			return fmt.Errorf("destination delegation starting info for %s/%s changed after preflight", change.newValidator, change.newDelegator)
		}
	}

	for _, write := range plan.queueWrites {
		if err := store.Set(write.key, write.value); err != nil {
			return err
		}
	}
	if applyDelegations {
		for _, change := range plan.delegations {
			old, err := stakingtypes.UnmarshalDelegation(k.cdc, change.old)
			if err != nil {
				return fmt.Errorf("decode source delegation snapshot: %w", err)
			}
			newRecord, err := stakingtypes.UnmarshalDelegation(k.cdc, change.new)
			if err != nil {
				return fmt.Errorf("decode destination delegation snapshot: %w", err)
			}
			if err := k.distributionKeeper.DeleteDelegatorStartingInfo(ctx, change.oldValidator, change.oldDelegator); err != nil {
				return err
			}
			if err := k.stakingKeeper.RemoveDelegation(ctx, old); err != nil {
				return err
			}
			if err := k.stakingKeeper.SetDelegation(ctx, newRecord); err != nil {
				return err
			}
			currentRewards, err := k.distributionKeeper.GetValidatorCurrentRewards(ctx, change.newValidator)
			if err != nil {
				return err
			}
			if currentRewards.Period == 0 {
				return fmt.Errorf("validator current rewards period is zero for %s", change.newValidator)
			}
			previousPeriod := currentRewards.Period - 1
			val, err := k.stakingKeeper.GetValidator(ctx, change.newValidator)
			if err != nil {
				return err
			}
			if err := k.incrementHistoricalRewardsReferenceCount(ctx, change.newValidator, previousPeriod); err != nil {
				return err
			}
			startingInfo := distrtypes.DelegatorStartingInfo{
				Height:         uint64(ctx.BlockHeight()),
				PreviousPeriod: previousPeriod,
				Stake:          val.TokensFromSharesTruncated(newRecord.Shares),
			}
			if err := k.distributionKeeper.SetDelegatorStartingInfo(ctx, change.newValidator, change.newDelegator, startingInfo); err != nil {
				return err
			}
		}
	}
	for _, change := range plan.ubds {
		old, err := stakingtypes.UnmarshalUBD(k.cdc, change.old)
		if err != nil {
			return fmt.Errorf("decode source UBD snapshot: %w", err)
		}
		newRecord, err := stakingtypes.UnmarshalUBD(k.cdc, change.new)
		if err != nil {
			return fmt.Errorf("decode destination UBD snapshot: %w", err)
		}
		if err := k.stakingKeeper.RemoveUnbondingDelegation(ctx, old); err != nil {
			return err
		}
		if err := k.stakingKeeper.SetUnbondingDelegation(ctx, newRecord); err != nil {
			return err
		}
		for _, entry := range newRecord.Entries {
			if entry.UnbondingId > 0 {
				if err := k.stakingKeeper.SetUnbondingDelegationByUnbondingID(ctx, newRecord, entry.UnbondingId); err != nil {
					return err
				}
			}
		}
	}
	for _, change := range plan.reds {
		old, err := stakingtypes.UnmarshalRED(k.cdc, change.old)
		if err != nil {
			return fmt.Errorf("decode source RED snapshot: %w", err)
		}
		newRecord, err := stakingtypes.UnmarshalRED(k.cdc, change.new)
		if err != nil {
			return fmt.Errorf("decode destination RED snapshot: %w", err)
		}
		if err := k.stakingKeeper.RemoveRedelegation(ctx, old); err != nil {
			return err
		}
		if err := k.stakingKeeper.SetRedelegation(ctx, newRecord); err != nil {
			return err
		}
		for _, entry := range newRecord.Entries {
			if entry.UnbondingId > 0 {
				if err := k.stakingKeeper.SetRedelegationByUnbondingID(ctx, newRecord, entry.UnbondingId); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// ApplyStakingMigrationPlan atomically applies a preflighted plan. Keeping the
// cache boundary here also makes direct/exported-helper callers safe outside a
// BaseApp transaction.
func (k Keeper) ApplyStakingMigrationPlan(ctx sdk.Context, plan stakingMigrationPlan) error {
	cacheCtx, commit := ctx.CacheContext()
	if err := k.applyStakingMigrationPlan(cacheCtx, plan, true); err != nil {
		return err
	}
	commit()
	return nil
}
