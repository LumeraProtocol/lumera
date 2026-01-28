package v1_10_0

import (
	storetypes "cosmossdk.io/store/types"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
)

// StoreUpgrades defines store changes for v1.10.0.
var StoreUpgrades = storetypes.StoreUpgrades{
	Deleted: []string{crisistypes.StoreKey},
}
