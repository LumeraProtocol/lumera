package params

import (
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/types/module"
	consensuskeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"

	actionmodulekeeper "github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	auditmodulekeeper "github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

type AppUpgradeParams struct {
	ChainID string

	Logger        log.Logger
	ModuleManager *module.Manager
	Configurator  module.Configurator

	// Keepers required by custom upgrade handlers. These are populated by the app
	// at startup (before state load) so upgrade handlers can safely perform
	// bespoke store migrations beyond RunMigrations.
	ActionKeeper          *actionmodulekeeper.Keeper
	SupernodeKeeper       sntypes.SupernodeKeeper
	ParamsKeeper          *paramskeeper.Keeper
	ConsensusParamsKeeper *consensuskeeper.Keeper
	AuditKeeper           *auditmodulekeeper.Keeper
}
