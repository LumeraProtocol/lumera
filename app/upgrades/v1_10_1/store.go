package v1_10_1

import (
	storetypes "cosmossdk.io/store/types"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
)

// StoreUpgrades defines store changes for v1.10.1.
// This mirrors v1.10.0 so chains upgrading directly from v1.9.1 still drop the crisis store.
var StoreUpgrades = storetypes.StoreUpgrades{
	Deleted: []string{crisistypes.StoreKey},
}
