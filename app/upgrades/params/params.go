package params

import (
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/types/module"
)

type AppUpgradeParams struct {
	ChainID string

	Logger log.Logger
	ModuleManager *module.Manager
	Configurator module.Configurator
}
