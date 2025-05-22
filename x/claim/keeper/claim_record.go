package keeper

import (
	"encoding/binary"

	"cosmossdk.io/store/prefix"
	"github.com/LumeraProtocol/lumera/x/claim/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// GetClaimRecord retrieves a claim record by address
func (k Keeper) GetClaimRecord(ctx sdk.Context, address string) (val types.ClaimRecord, found bool, err error) {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, types.KeyPrefix(types.ClaimRecordKey))

	b := store.Get([]byte(address))
	if b == nil {
		return val, false, nil
	}

	err = k.cdc.Unmarshal(b, &val)
	if err != nil {
		return val, true, err
	}
	return val, true, nil
}

func (k Keeper) SetClaimRecord(ctx sdk.Context, claimRecord types.ClaimRecord) error {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, types.KeyPrefix(types.ClaimRecordKey))

	b, err := k.cdc.Marshal(&claimRecord)
	if err != nil {
		return err
	}

	store.Set([]byte(claimRecord.OldAddress), b)
	k.incrementClaimRecordCount(ctx)
	return nil
}

// ListClaimRecords returns all claim records with optional pagination and filtering
// If filter is nil, all records are returned
// Example usage:
// To filter only claimed records:
//
//	records, err := k.ListClaimRecords(ctx, func(record *types.ClaimRecord) bool {
//		return record.Claimed
//	})
//
// To filter claimed records with a specific condition:
//
//	records, err := k.ListClaimRecords(ctx, func(record *types.ClaimRecord) bool {
//		return record.Claimed && someOtherCondition
//	})
func (k Keeper) ListClaimRecords(ctx sdk.Context, filter func(*types.ClaimRecord) bool) ([]*types.ClaimRecord, error) {
	var claimRecords []*types.ClaimRecord

	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, []byte(types.ClaimRecordKey))

	iterator := store.Iterator(nil, nil)
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		var claimRecord types.ClaimRecord
		k.cdc.MustUnmarshal(iterator.Value(), &claimRecord)

		// Apply filter if provided
		if filter == nil || filter(&claimRecord) {
			claimRecords = append(claimRecords, &claimRecord)
		}
	}

	return claimRecords, nil
}

func (k Keeper) GetClaimRecordCount(ctx sdk.Context) uint64 {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, []byte{})
	byteKey := []byte(types.ClaimRecordCountKey)

	bz := store.Get(byteKey)
	if bz == nil {
		return 0
	}

	return binary.BigEndian.Uint64(bz)
}

func (k Keeper) incrementClaimRecordCount(ctx sdk.Context) {
	count := k.GetClaimRecordCount(ctx)
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, []byte{})

	byteKey := []byte(types.ClaimRecordCountKey)
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, count+1)
	store.Set(byteKey, bz)
}
