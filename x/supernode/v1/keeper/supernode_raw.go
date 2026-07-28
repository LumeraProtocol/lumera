package keeper

import (
	"bytes"
	"fmt"

	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// StrictGetSuperNodeByAccount resolves account ownership without treating
// corrupt or incomplete state as absence. It scans the complete primary prefix
// to prove absence and detect duplicate claims while retaining only the records
// relevant to the requested account and its secondary-index entry.
func (k Keeper) StrictGetSuperNodeByAccount(ctx sdk.Context, account string) (types.SuperNode, bool, error) {
	if _, err := sdk.AccAddressFromBech32(account); err != nil {
		return types.SuperNode{}, false, fmt.Errorf("invalid requested supernode account %q: %w", account, err)
	}

	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	indexStore := prefix.NewStore(storeAdapter, types.SuperNodeByAccountKey)
	indexKey := indexStore.Get([]byte(account))
	indexFound := indexKey != nil
	if indexFound {
		indexKey = bytes.Clone(indexKey)
		if err := sdk.VerifyAddressFormat(indexKey); err != nil {
			return types.SuperNode{}, false, fmt.Errorf("invalid validator key in account index for %s: %w", account, err)
		}
	}

	primaryStore := prefix.NewStore(storeAdapter, types.SuperNodeKey)
	iterator := primaryStore.Iterator(nil, nil)
	defer func() { _ = iterator.Close() }()

	var matching types.SuperNode
	matchingCount := 0
	var indexed types.SuperNode
	indexedFound := false

	for ; iterator.Valid(); iterator.Next() {
		primaryKey := iterator.Key()
		if err := sdk.VerifyAddressFormat(primaryKey); err != nil {
			return types.SuperNode{}, false, fmt.Errorf("invalid supernode primary key %X: %w", primaryKey, err)
		}

		var sn types.SuperNode
		if err := k.cdc.Unmarshal(iterator.Value(), &sn); err != nil {
			return types.SuperNode{}, false, fmt.Errorf("unmarshal supernode at primary key %X: %w", primaryKey, err)
		}

		validatorAddress, err := sdk.ValAddressFromBech32(sn.ValidatorAddress)
		if err != nil {
			return types.SuperNode{}, false, fmt.Errorf("invalid embedded validator address at primary key %X: %w", primaryKey, err)
		}
		if !bytes.Equal(primaryKey, validatorAddress) {
			return types.SuperNode{}, false, fmt.Errorf("supernode validator mismatch at primary key %X: embedded validator is %s", primaryKey, sn.ValidatorAddress)
		}
		if _, err := sdk.AccAddressFromBech32(sn.SupernodeAccount); err != nil {
			return types.SuperNode{}, false, fmt.Errorf("invalid embedded supernode account at primary key %X: %w", primaryKey, err)
		}

		if sn.SupernodeAccount == account {
			matching = sn
			matchingCount++
		}
		if indexFound && bytes.Equal(primaryKey, indexKey) {
			indexed = sn
			indexedFound = true
		}
	}

	if matchingCount > 1 {
		return types.SuperNode{}, false, fmt.Errorf("multiple primary records claim supernode account %s", account)
	}
	if indexFound {
		if !indexedFound {
			return types.SuperNode{}, false, fmt.Errorf("account index for %s does not resolve to a primary record", account)
		}
		if indexed.SupernodeAccount != account {
			return types.SuperNode{}, false, fmt.Errorf("account mismatch for index %s: primary record owns %s", account, indexed.SupernodeAccount)
		}
		return indexed, true, nil
	}
	if matchingCount == 1 {
		return types.SuperNode{}, false, fmt.Errorf("missing account index for supernode account %s", matching.SupernodeAccount)
	}
	return types.SuperNode{}, false, nil
}
