package upgrades

import (
	"testing"

	"cosmossdk.io/log"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/stretchr/testify/require"

	appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
	upgrade_v1_6_1 "github.com/LumeraProtocol/lumera/app/upgrades/v1_6_1"
	upgrade_v1_8_0 "github.com/LumeraProtocol/lumera/app/upgrades/v1_8_0"
	upgrade_v1_8_4 "github.com/LumeraProtocol/lumera/app/upgrades/v1_8_4"
	upgrade_v1_9_0 "github.com/LumeraProtocol/lumera/app/upgrades/v1_9_0"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
)

func TestUpgradeNamesOrder(t *testing.T) {
	expected := []string{
		upgrade_v1_6_1.UpgradeName,
		upgradeNameV170,
		upgradeNameV172,
		upgrade_v1_8_0.UpgradeName,
		upgrade_v1_8_4.UpgradeName,
		upgradeNameV185,
		upgrade_v1_9_0.UpgradeName,
	}
	require.Equal(t, expected, upgradeNames, "upgradeNames should stay in ascending order")
}

func TestSetupUpgradesAndHandlers(t *testing.T) {
	tests := []struct {
		name    string
		chainID string
	}{
		{name: "mainnet", chainID: "lumera-mainnet-1"},
		{name: "devnet", chainID: "lumera-devnet-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := newTestUpgradeParams(tt.chainID)
			ctx := sdk.NewContext(nil, tmproto.Header{ChainID: tt.chainID}, false, params.Logger)
			goCtx := sdk.WrapSDKContext(ctx)

			for _, upgradeName := range upgradeNames {
				config, found := SetupUpgrades(upgradeName, params)
				require.True(t, found, "upgrade %s should be known", upgradeName)

				require.Equal(t,
					expectHandler(upgradeName, tt.chainID),
					config.Handler != nil,
					"handler presence mismatch for %s on %s", upgradeName, tt.chainID,
				)

				require.Equal(t,
					expectStoreUpgrade(upgradeName, tt.chainID),
					config.StoreUpgrade != nil,
					"store upgrade presence mismatch for %s on %s", upgradeName, tt.chainID,
				)

				if config.Handler == nil {
					continue
				}

				// v1.8.8 requires full keeper wiring; exercising it here would require
				// a full app harness. This test only verifies registration and gating.
				if upgradeName == upgrade_v1_9_0.UpgradeName {
					continue
				}

				vm, err := config.Handler(goCtx, upgradetypes.Plan{}, module.VersionMap{})
				require.NoError(t, err, "handler should succeed for %s on %s", upgradeName, tt.chainID)
				require.NotNil(t, vm, "handler should return a version map")

				// v1.6.1 explicitly adds the action module consensus version.
				if upgradeName == upgrade_v1_6_1.UpgradeName {
					_, ok := vm[actiontypes.ModuleName]
					require.True(t, ok, "v1.6.1 should set action module version")
				}
			}
		})
	}
}

func newTestUpgradeParams(chainID string) appParams.AppUpgradeParams {
	return appParams.AppUpgradeParams{
		ChainID:       chainID,
		Logger:        log.NewNopLogger(),
		ModuleManager: module.NewManager(),
		Configurator:  module.NewConfigurator(nil, nil, nil),
	}
}

func expectHandler(upgradeName, chainID string) bool {
	switch upgradeName {
	case upgrade_v1_8_0.UpgradeName:
		return IsTestnet(chainID) || IsDevnet(chainID)
	default:
		return true
	}
}

func expectStoreUpgrade(upgradeName, chainID string) bool {
	switch upgradeName {
	case upgrade_v1_8_0.UpgradeName:
		return IsTestnet(chainID) || IsDevnet(chainID)
	case upgrade_v1_8_4.UpgradeName:
		return IsMainnet(chainID)
	default:
		return false
	}
}
