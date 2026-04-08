package v1_15_0

import (
	storetypes "cosmossdk.io/store/types"
)

// StoreUpgrades defines store changes for v1.15.0.
// Everlight now lives inside x/supernode, so no dedicated store is added.
var StoreUpgrades = storetypes.StoreUpgrades{}
