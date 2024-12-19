package keeper

import (
	"fmt"

	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pastelnetwork/pastel/x/supernode/types"
)

// SetSuperNode sets a supernode record in the store
func (k Keeper) SetSuperNode(ctx sdk.Context, supernode types.SuperNode) error {
	// Convert context store to a KVStore interface
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	// Create a prefix store so that all keys are under SuperNodeKey
	store := prefix.NewStore(storeAdapter, []byte(types.SuperNodeKey))

	// Marshal the SuperNode into bytes
	b, err := k.cdc.Marshal(&supernode)
	if err != nil {
		return err
	}

	// Use the validator address as the key (since it's unique).
	valOperAddr, err := sdk.ValAddressFromBech32(supernode.ValidatorAddress)
	if err != nil {
		return err
	}

	// Set the supernode record under [SuperNodeKeyPrefix + valOperAddr]
	// Note: prefix.NewStore automatically prepends the prefix we defined above.
	store.Set(valOperAddr, b)

	return nil
}

// GetSuperNode returns the supernode record for a given validator address
func (k Keeper) QuerySuperNode(ctx sdk.Context, valOperAddr sdk.ValAddress) (sn types.SuperNode, exists bool, err error) {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, []byte(types.SuperNodeKey))

	bz := store.Get(valOperAddr)
	if bz == nil {
		return types.SuperNode{}, false, nil
	}

	if err := k.cdc.Unmarshal(bz, &sn); err != nil {
		return types.SuperNode{}, false, fmt.Errorf("failed to unmarshal supernode: %w", err)
	}

	return sn, true, nil
}

// GetAllSuperNodes returns all supernodes
func (k Keeper) GetAllSuperNodes(ctx sdk.Context) ([]types.SuperNode, error) {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, []byte(types.SuperNodeKey))

	iterator := store.Iterator(nil, nil)
	defer iterator.Close()

	var supernodes []types.SuperNode
	for ; iterator.Valid(); iterator.Next() {
		bz := iterator.Value()
		var sn types.SuperNode
		if err := k.cdc.Unmarshal(bz, &sn); err != nil {
			return nil, fmt.Errorf("failed to unmarshal supernode: %w", err)
		}
		supernodes = append(supernodes, sn)
	}

	return supernodes, nil
}
