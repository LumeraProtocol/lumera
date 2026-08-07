package keeper

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"

	storetypes "cosmossdk.io/store/types"
	db "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// StrictGetSuperNodeByAccount resolves account ownership without treating
// corrupt or incomplete state as absence. It scans the complete primary prefix
// to prove absence and detect duplicate claims while retaining only the records
// relevant to the requested account and its secondary-index entry.
func (k Keeper) StrictGetSuperNodeByAccount(ctx sdk.Context, account string) (types.SuperNode, bool, error) {
	requestedAccount, err := sdk.AccAddressFromBech32(account)
	if err != nil {
		return types.SuperNode{}, false, fmt.Errorf("invalid requested supernode account %q: %w", account, err)
	}

	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	indexIterator := storeAdapter.Iterator(types.SuperNodeByAccountKey, storetypes.PrefixEndBytes(types.SuperNodeByAccountKey))
	indexKey, indexCount, err := scanStrictSuperNodeAccountIndexes(indexIterator, requestedAccount)
	if err != nil {
		return types.SuperNode{}, false, err
	}
	if indexCount > 1 {
		return types.SuperNode{}, false, fmt.Errorf("multiple account index entries claim supernode account %s", account)
	}
	indexFound := indexCount == 1

	primaryIterator := storeAdapter.Iterator(types.SuperNodeKey, storetypes.PrefixEndBytes(types.SuperNodeKey))
	matching, matchingCount, indexed, indexedFound, err := k.scanStrictSuperNodes(primaryIterator, requestedAccount, indexKey, indexFound)
	if err != nil {
		return types.SuperNode{}, false, err
	}

	if matchingCount > 1 {
		return types.SuperNode{}, false, fmt.Errorf("multiple primary records claim supernode account %s", account)
	}
	if indexFound {
		if !indexedFound {
			return types.SuperNode{}, false, fmt.Errorf("account index for %s does not resolve to a primary record", account)
		}
		indexedAccount, err := sdk.AccAddressFromBech32(indexed.SupernodeAccount)
		if err != nil {
			return types.SuperNode{}, false, fmt.Errorf("invalid indexed supernode account %q: %w", indexed.SupernodeAccount, err)
		}
		if !indexedAccount.Equals(requestedAccount) {
			return types.SuperNode{}, false, fmt.Errorf("account mismatch for index %s: primary record owns %s", account, indexed.SupernodeAccount)
		}
		return indexed, true, nil
	}
	if matchingCount == 1 {
		return types.SuperNode{}, false, fmt.Errorf("missing account index for supernode account %s", matching.SupernodeAccount)
	}
	return types.SuperNode{}, false, nil
}

func scanStrictSuperNodeAccountIndexes(
	iterator db.Iterator,
	requestedAccount sdk.AccAddress,
) (indexKey []byte, matchingCount int, err error) {
	defer closeStrictIterator(iterator, &err, "close supernode account-index iterator")

	for ; iterator.Valid(); iterator.Next() {
		storeKey := iterator.Key()
		if !bytes.HasPrefix(storeKey, types.SuperNodeByAccountKey) {
			return indexKey, matchingCount, fmt.Errorf("supernode account-index iterator returned key outside prefix: %X", storeKey)
		}
		accountText := storeKey[len(types.SuperNodeByAccountKey):]
		indexedAccount, err := sdk.AccAddressFromBech32(string(accountText))
		if err != nil {
			return indexKey, matchingCount, fmt.Errorf("invalid supernode account-index key %q: %w", accountText, err)
		}
		if !indexedAccount.Equals(requestedAccount) {
			continue
		}

		validatorKey := iterator.Value()
		if err := sdk.VerifyAddressFormat(validatorKey); err != nil {
			return indexKey, matchingCount, fmt.Errorf("invalid validator key in account index for %s: %w", requestedAccount, err)
		}
		indexKey = bytes.Clone(validatorKey)
		matchingCount++
	}
	if err := strictIteratorTerminalError(iterator); err != nil {
		return indexKey, matchingCount, fmt.Errorf("iterate supernode account-index records: %w", err)
	}
	return indexKey, matchingCount, nil
}

func (k Keeper) scanStrictSuperNodes(
	iterator db.Iterator,
	requestedAccount sdk.AccAddress,
	indexKey []byte,
	indexFound bool,
) (matching types.SuperNode, matchingCount int, indexed types.SuperNode, indexedFound bool, err error) {
	defer closeStrictIterator(iterator, &err, "close supernode primary iterator")

	for ; iterator.Valid(); iterator.Next() {
		storeKey := iterator.Key()
		if !bytes.HasPrefix(storeKey, types.SuperNodeKey) {
			return matching, matchingCount, indexed, indexedFound, fmt.Errorf("supernode iterator returned key outside primary prefix: %X", storeKey)
		}
		primaryKey := storeKey[len(types.SuperNodeKey):]
		if err := sdk.VerifyAddressFormat(primaryKey); err != nil {
			return matching, matchingCount, indexed, indexedFound, fmt.Errorf("invalid supernode primary key %X: %w", primaryKey, err)
		}

		var sn types.SuperNode
		if err := k.cdc.Unmarshal(iterator.Value(), &sn); err != nil {
			return matching, matchingCount, indexed, indexedFound, fmt.Errorf("unmarshal supernode at primary key %X: %w", primaryKey, err)
		}

		validatorAddress, err := sdk.ValAddressFromBech32(sn.ValidatorAddress)
		if err != nil {
			return matching, matchingCount, indexed, indexedFound, fmt.Errorf("invalid embedded validator address at primary key %X: %w", primaryKey, err)
		}
		if !bytes.Equal(primaryKey, validatorAddress) {
			return matching, matchingCount, indexed, indexedFound, fmt.Errorf("supernode validator mismatch at primary key %X: embedded validator is %s", primaryKey, sn.ValidatorAddress)
		}
		supernodeAccount, err := sdk.AccAddressFromBech32(sn.SupernodeAccount)
		if err != nil {
			return matching, matchingCount, indexed, indexedFound, fmt.Errorf("invalid embedded supernode account at primary key %X: %w", primaryKey, err)
		}

		if supernodeAccount.Equals(requestedAccount) {
			matching = sn
			matchingCount++
		}
		if indexFound && bytes.Equal(primaryKey, indexKey) {
			indexed = sn
			indexedFound = true
		}
	}
	if err := strictIteratorTerminalError(iterator); err != nil {
		return matching, matchingCount, indexed, indexedFound, fmt.Errorf("iterate supernode primary records: %w", err)
	}
	return matching, matchingCount, indexed, indexedFound, nil
}

func closeStrictIterator(iterator db.Iterator, scanErr *error, operation string) {
	if closeErr := iterator.Close(); closeErr != nil {
		*scanErr = errors.Join(*scanErr, fmt.Errorf("%s: %w", operation, closeErr))
	}
}

func strictIteratorTerminalError(iterator db.Iterator) error {
	err := iterator.Error()
	if err == nil {
		return nil
	}

	// cosmossdk.io/store/cachekv's cacheMergeIterator violates the db.Iterator
	// contract by returning this sentinel whenever normal exhaustion makes it
	// invalid. DeliverTx reads run through this iterator, so treat only that exact
	// SDK sentinel as clean exhaustion; every other terminal error still
	// fails closed.
	if err.Error() == "invalid cacheMergeIterator" {
		typ := reflect.TypeOf(iterator)
		if typ != nil && typ.Kind() == reflect.Ptr {
			typ = typ.Elem()
			if (typ.PkgPath() == "cosmossdk.io/store/cachekv/internal" && typ.Name() == "cacheMergeIterator") ||
				(typ.PkgPath() == "cosmossdk.io/store/gaskv" && typ.Name() == "gasIterator") {
				return nil
			}
		}
	}
	return err
}
