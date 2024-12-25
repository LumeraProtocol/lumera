package keeper

import (
	"fmt"

	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
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
func (k Keeper) QuerySuperNode(ctx sdk.Context, valOperAddr sdk.ValAddress) (sn types.SuperNode, exists bool) {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, []byte(types.SuperNodeKey))

	bz := store.Get(valOperAddr)
	if bz == nil {
		return types.SuperNode{}, false
	}

	if err := k.cdc.Unmarshal(bz, &sn); err != nil {
		k.logger.Error(fmt.Sprintf("failed to unmarshal supernode: %s", err))
		return types.SuperNode{}, false
	}

	return sn, true
}

// GetAllSuperNodes returns all supernodes, optionally filtered by state
func (k Keeper) GetAllSuperNodes(ctx sdk.Context, stateFilters ...types.SuperNodeState) ([]types.SuperNode, error) {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, []byte(types.SuperNodeKey))

	iterator := store.Iterator(nil, nil)
	defer iterator.Close()

	var supernodes []types.SuperNode
	filtering := shouldFilter(stateFilters...)

	for ; iterator.Valid(); iterator.Next() {
		bz := iterator.Value()
		var sn types.SuperNode
		if err := k.cdc.Unmarshal(bz, &sn); err != nil {
			return nil, fmt.Errorf("failed to unmarshal supernode: %w", err)
		}

		if len(sn.States) == 0 {
			continue
		}

		if !filtering || stateIn(sn.States[len(sn.States)-1].State, stateFilters...) {
			supernodes = append(supernodes, sn)
		}
	}

	return supernodes, nil
}

// GetSuperNodesPaginated returns paginated supernodes, optionally filtered by state
func (k Keeper) GetSuperNodesPaginated(ctx sdk.Context, pagination *query.PageRequest, stateFilters ...types.SuperNodeState) ([]*types.SuperNode, *query.PageResponse, error) {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, []byte(types.SuperNodeKey))

	var supernodes []*types.SuperNode
	filtering := shouldFilter(stateFilters...)

	pageRes, err := query.Paginate(store, pagination, func(key, value []byte) error {
		var sn types.SuperNode
		if err := k.cdc.Unmarshal(value, &sn); err != nil {
			return err
		}

		if len(sn.States) == 0 {
			return nil
		}

		if !filtering || stateIn(sn.States[len(sn.States)-1].State, stateFilters...) {
			supernodes = append(supernodes, &sn)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return supernodes, pageRes, nil
}

func stateIn(state types.SuperNodeState, stateFilters ...types.SuperNodeState) bool {
	for _, sf := range stateFilters {
		if sf == state {
			return true
		}
	}
	return false
}

func shouldFilter(stateFilters ...types.SuperNodeState) bool {
	if len(stateFilters) == 0 {
		return false
	}
	// If SuperNodeStateUnspecified is present, it means no filtering
	for _, sf := range stateFilters {
		if sf == types.SuperNodeStateUnspecified {
			return false
		}
	}
	return true
}
