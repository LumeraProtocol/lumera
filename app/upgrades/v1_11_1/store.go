package v1_11_1

import (
	storetypes "cosmossdk.io/store/types"

	audittypes "github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// StoreUpgrades declares store additions/deletions for v1.11.1.
//
// The audit store is included so direct upgrades from pre-audit binaries
// (e.g. v1.10.1) can add it. The store loader for this upgrade is conditional
// and will skip adding it when the store already exists (e.g. upgrading from
// v1.11.0).
var StoreUpgrades = storetypes.StoreUpgrades{
	Added: []string{audittypes.StoreKey},
}
