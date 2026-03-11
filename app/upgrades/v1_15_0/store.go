package v1_15_0

import (
	storetypes "cosmossdk.io/store/types"
	everlighttypes "github.com/LumeraProtocol/lumera/x/everlight/v1/types"
)

// StoreUpgrades defines store changes for v1.15.0.
// Adds the x/everlight module store.
var StoreUpgrades = storetypes.StoreUpgrades{
	Added: []string{everlighttypes.StoreKey},
}
