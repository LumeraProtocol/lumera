package upgrades

import (
	"strings"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
	upgrade_v1_6_1 "github.com/LumeraProtocol/lumera/app/upgrades/v1_6_1"
	upgrade_v1_7_0 "github.com/LumeraProtocol/lumera/app/upgrades/v1_7_0"
	upgrade_v1_7_2 "github.com/LumeraProtocol/lumera/app/upgrades/v1_7_2"
	upgrade_v1_8_0 "github.com/LumeraProtocol/lumera/app/upgrades/v1_8_0"
	upgrade_v1_8_4 "github.com/LumeraProtocol/lumera/app/upgrades/v1_8_4"
)

type UpgradeConfig struct {
	StoreUpgrade *storetypes.StoreUpgrades
	Handler      upgradetypes.UpgradeHandler
}

var upgradeNames = []string{
	upgrade_v1_6_1.UpgradeName,
	upgrade_v1_7_0.UpgradeName,
	upgrade_v1_7_2.UpgradeName,
	upgrade_v1_8_0.UpgradeName,
	upgrade_v1_8_4.UpgradeName,
}

var NoUpgradeConfig = UpgradeConfig{
	StoreUpgrade: nil,
	Handler:      nil,
}

// SetupUpgrades returns the configuration for the requested upgrade (if any).
// Do not define StoreUpgrades if there are no store changes for that upgrade.
// Do not define an UpgradeHandler if no custom logic is needed beyond module migrations.
func SetupUpgrades(upgradeName string, params appParams.AppUpgradeParams) (UpgradeConfig, bool) {
	switch upgradeName {
	case upgrade_v1_6_1.UpgradeName:
		return UpgradeConfig{
			Handler: upgrade_v1_6_1.CreateUpgradeHandler(params),
		}, true
	case upgrade_v1_7_0.UpgradeName:
		return UpgradeConfig{
			Handler: upgrade_v1_7_0.CreateUpgradeHandler(params),
		}, true
	case upgrade_v1_7_2.UpgradeName:
		return UpgradeConfig{
			Handler: upgrade_v1_7_2.CreateUpgradeHandler(params),
		}, true
	case upgrade_v1_8_0.UpgradeName:
		if strings.HasPrefix(params.ChainID, "lumera-testnet") || strings.HasPrefix(params.ChainID, "lumera-devnet") {
			return UpgradeConfig{
				StoreUpgrade: &upgrade_v1_8_0.StoreUpgrades,
				Handler:      upgrade_v1_8_0.CreateUpgradeHandler(params),
			}, true
		}
		return NoUpgradeConfig, true
	case upgrade_v1_8_4.UpgradeName:
		config := UpgradeConfig{
			Handler: upgrade_v1_8_4.CreateUpgradeHandler(params),
		}
		if strings.HasPrefix(params.ChainID, "lumera-mainnet") {
			config.StoreUpgrade = &upgrade_v1_8_4.StoreUpgrades
		}
		return config, true

	// add future upgrades here
	default:
		return UpgradeConfig{}, false
	}
}

// AllUpgrades returns the upgrade configuration for every known upgrade name.
func AllUpgrades(params appParams.AppUpgradeParams) map[string]UpgradeConfig {
	configs := make(map[string]UpgradeConfig, len(upgradeNames))
	for _, upgradeName := range upgradeNames {
		if config, found := SetupUpgrades(upgradeName, params); found {
			configs[upgradeName] = config
		}
	}
	return configs
}
