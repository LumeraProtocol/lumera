package keeper

import (
	"fmt"

	corestore "cosmossdk.io/core/store"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type historicalRewardsEntry struct {
	period  uint64
	rewards distrtypes.ValidatorHistoricalRewards
}

type slashEventEntry struct {
	height uint64
	event  distrtypes.ValidatorSlashEvent
}

func (k Keeper) redelegationsForValidator(ctx sdk.Context, valAddr sdk.ValAddress) ([]stakingtypes.Redelegation, error) {
	if k.stakingStoreHandle == nil || k.stakingStoreHandle.svc == nil {
		var reds []stakingtypes.Redelegation
		if err := k.stakingKeeper.IterateRedelegations(ctx, func(_ int64, red stakingtypes.Redelegation) bool {
			if red.ValidatorSrcAddress == valAddr.String() || red.ValidatorDstAddress == valAddr.String() {
				reds = append(reds, red)
			}
			return false
		}); err != nil {
			return nil, err
		}
		return reds, nil
	}

	store := k.stakingStoreHandle.svc.OpenKVStore(ctx)
	seen := make(map[string]stakingtypes.Redelegation)
	if err := k.collectRedelegationsByIndex(store, stakingtypes.GetREDsFromValSrcIndexKey(valAddr), stakingtypes.GetREDKeyFromValSrcIndexKey, seen); err != nil {
		return nil, err
	}
	if err := k.collectRedelegationsByIndex(store, stakingtypes.GetREDsToValDstIndexKey(valAddr), stakingtypes.GetREDKeyFromValDstIndexKey, seen); err != nil {
		return nil, err
	}

	reds := make([]stakingtypes.Redelegation, 0, len(seen))
	for _, red := range seen {
		reds = append(reds, red)
	}
	return reds, nil
}

func (k Keeper) collectRedelegationsByIndex(
	store corestore.KVStore,
	prefix []byte,
	indexToRedelegationKey func([]byte) []byte,
	out map[string]stakingtypes.Redelegation,
) error {
	iterator, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return err
	}
	defer func() { _ = iterator.Close() }()

	for ; iterator.Valid(); iterator.Next() {
		redKey := indexToRedelegationKey(iterator.Key())
		bz, err := store.Get(redKey)
		if err != nil {
			return err
		}
		if bz == nil {
			return fmt.Errorf("redelegation index %X points to missing record %X", iterator.Key(), redKey)
		}
		red, err := stakingtypes.UnmarshalRED(k.cdc, bz)
		if err != nil {
			return err
		}
		out[string(redKey)] = red
	}
	return nil
}

func (k Keeper) validatorHistoricalRewards(ctx sdk.Context, valAddr sdk.ValAddress) ([]historicalRewardsEntry, error) {
	if k.distributionStoreHandle == nil || k.distributionStoreHandle.svc == nil {
		var entries []historicalRewardsEntry
		k.distributionKeeper.IterateValidatorHistoricalRewards(ctx, func(val sdk.ValAddress, period uint64, rewards distrtypes.ValidatorHistoricalRewards) bool {
			if val.Equals(valAddr) {
				entries = append(entries, historicalRewardsEntry{period: period, rewards: rewards})
			}
			return false
		})
		return entries, nil
	}

	store := k.distributionStoreHandle.svc.OpenKVStore(ctx)
	prefix := distrtypes.GetValidatorHistoricalRewardsPrefix(valAddr)
	iterator, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return nil, err
	}
	defer func() { _ = iterator.Close() }()

	var entries []historicalRewardsEntry
	for ; iterator.Valid(); iterator.Next() {
		var rewards distrtypes.ValidatorHistoricalRewards
		k.cdc.MustUnmarshal(iterator.Value(), &rewards)
		_, period := distrtypes.GetValidatorHistoricalRewardsAddressPeriod(iterator.Key())
		entries = append(entries, historicalRewardsEntry{period: period, rewards: rewards})
	}
	return entries, nil
}

// validatorHistoricalReward fetches a single (valAddr, period) historical
// rewards row. With the distribution store wired it is an O(1) key lookup;
// it only falls back to a full IterateValidatorHistoricalRewards scan when the
// store service is unwired (tests that do not wire the scoped stores).
func (k Keeper) validatorHistoricalReward(ctx sdk.Context, valAddr sdk.ValAddress, period uint64) (distrtypes.ValidatorHistoricalRewards, bool, error) {
	if k.distributionStoreHandle == nil || k.distributionStoreHandle.svc == nil {
		var (
			found      bool
			historical distrtypes.ValidatorHistoricalRewards
		)
		k.distributionKeeper.IterateValidatorHistoricalRewards(ctx, func(val sdk.ValAddress, p uint64, rewards distrtypes.ValidatorHistoricalRewards) bool {
			if val.Equals(valAddr) && p == period {
				found = true
				historical = rewards
				return true
			}
			return false
		})
		return historical, found, nil
	}

	store := k.distributionStoreHandle.svc.OpenKVStore(ctx)
	bz, err := store.Get(distrtypes.GetValidatorHistoricalRewardsKey(valAddr, period))
	if err != nil {
		return distrtypes.ValidatorHistoricalRewards{}, false, err
	}
	if bz == nil {
		return distrtypes.ValidatorHistoricalRewards{}, false, nil
	}
	var historical distrtypes.ValidatorHistoricalRewards
	if err := k.cdc.Unmarshal(bz, &historical); err != nil {
		return distrtypes.ValidatorHistoricalRewards{}, false, err
	}
	return historical, true, nil
}

func (k Keeper) validatorSlashEvents(ctx sdk.Context, valAddr sdk.ValAddress) ([]slashEventEntry, error) {
	if k.distributionStoreHandle == nil || k.distributionStoreHandle.svc == nil {
		var entries []slashEventEntry
		k.distributionKeeper.IterateValidatorSlashEvents(ctx, func(val sdk.ValAddress, height uint64, event distrtypes.ValidatorSlashEvent) bool {
			if val.Equals(valAddr) {
				entries = append(entries, slashEventEntry{height: height, event: event})
			}
			return false
		})
		return entries, nil
	}

	store := k.distributionStoreHandle.svc.OpenKVStore(ctx)
	prefix := distrtypes.GetValidatorSlashEventPrefix(valAddr)
	iterator, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return nil, err
	}
	defer func() { _ = iterator.Close() }()

	var entries []slashEventEntry
	for ; iterator.Valid(); iterator.Next() {
		var event distrtypes.ValidatorSlashEvent
		k.cdc.MustUnmarshal(iterator.Value(), &event)
		_, height := distrtypes.GetValidatorSlashEventAddressHeight(iterator.Key())
		entries = append(entries, slashEventEntry{height: height, event: event})
	}
	return entries, nil
}
