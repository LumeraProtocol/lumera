package v1_12_0

import (
	storetypes "cosmossdk.io/store/types"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
)

// StoreUpgrades defines store changes for v1.12.0.
var StoreUpgrades = storetypes.StoreUpgrades{
	Deleted: []string{crisistypes.StoreKey},
}
