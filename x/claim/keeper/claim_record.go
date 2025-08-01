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
