package upgrades_test

import (
	"testing"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/stretchr/testify/require"

	lumeraapp "github.com/LumeraProtocol/lumera/app"
	"github.com/LumeraProtocol/lumera/app/upgrades"
	audittypes "github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// TestV1202ModuleVersionsMatchLiveChains pins the module consensus versions the
// v1.20.2 binary declares against what mainnet and testnet actually report.
//
// Verified live on 2026-08-01 via /cosmos/upgrade/v1beta1/module_versions:
//
//	lumera-mainnet-1   audit 2   supernode 1   (evmigration absent)
//	lumera-testnet-2   audit 2   supernode 1   evmigration 1
//
// This release intentionally carries NO module version bump: it is a behavior
// activation boundary, not a state migration. If someone later raises one of
// these without adding the matching RegisterMigration, RunMigrations fails at
// the halt height on a live chain -- with every validator already stopped.
// Fail here instead.
func TestV1202ModuleVersionsMatchLiveChains(t *testing.T) {
	app := lumeraapp.Setup(t)

	vm := app.ModuleManager.GetVersionMap()

	require.Equal(t, uint64(2), vm["audit"],
		"audit must stay at ConsensusVersion 2: both mainnet and testnet report 2, "+
			"and v1.20.2 registers no audit migration")
	require.Equal(t, uint64(audittypes.ConsensusVersion), vm["audit"],
		"the module and its types package must agree on the audit consensus version")
	require.Equal(t, uint64(1), vm["supernode"],
		"supernode must stay at ConsensusVersion 1: both live chains report 1")
	require.Equal(t, uint64(1), vm["evmigration"],
		"evmigration must stay at ConsensusVersion 1: testnet reports 1 and mainnet "+
			"initializes it at 1 during the one-hop bring-up")
}

// TestV1202TestnetRunMigrationsIsANoop proves the testnet path carries no module
// migration. The enclosing handler still applies the feemarket base-fee update.
//
// Testnet arrives with exactly the versions the new binary declares, so
// RunMigrations must find no work. Asserting the returned version map equals
// the binary's own map verifies that no module consensus migration runs.
func TestV1202TestnetRunMigrationsIsANoop(t *testing.T) {
	app := lumeraapp.Setup(t)
	ctx := app.BaseApp.NewContext(false).WithChainID("lumera-testnet-2")

	params := newV1202Params(app, "lumera-testnet-2")
	params.ModuleManager = app.ModuleManager
	params.Configurator = app.Configurator()

	config, found := upgrades.SetupUpgrades(upgradeNameV1202, params)
	require.True(t, found)

	// Testnet's real arrival shape: every module at the version it reports live.
	fromVM := app.ModuleManager.GetVersionMap()

	newVM, err := config.Handler(sdk.WrapSDKContext(ctx), upgradetypes.Plan{}, fromVM)
	require.NoError(t, err, "the testnet existing-EVM path must succeed")
	require.Equal(t, fromVM, newVM,
		"v1.20.2 on testnet must be a pure no-op at the module-version layer: "+
			"no module may be migrated by this release")
}

// TestV1202MainnetArrivesAtSameVersionsAsTestnet proves both networks converge.
//
// After the upgrade, a mainnet node that took the 1.12.0 one-hop and a testnet
// node that took the 1.20.1 hop must be running the same module versions.
// Divergence here means the two networks are on different state machines.
func TestV1202MainnetArrivesAtSameVersionsAsTestnet(t *testing.T) {
	app := lumeraapp.Setup(t)
	ctx := app.BaseApp.NewContext(false).WithChainID("lumera-mainnet-1")

	params := newV1202Params(app, "lumera-mainnet-1")
	params.ModuleManager = app.ModuleManager
	params.Configurator = app.Configurator()

	config, found := upgrades.SetupUpgrades(upgradeNameV1202, params)
	require.True(t, found)

	// Mainnet's real arrival shape: every NON-EVM module at its current version,
	// with the four EVM modules and evmigration absent.
	//
	// Note this is NOT module.VersionMap{}. An empty map tells RunMigrations that
	// every module is brand new, so it calls InitGenesis on all of them and panics
	// with "groups: sequence: already initialized". A live 1.12.0 chain reports
	// auth/bank/staking/group/audit/supernode normally -- only the EVM stack is
	// missing -- so an empty map models a state no chain is ever in.
	fromVM := app.ModuleManager.GetVersionMap()
	for _, name := range []string{
		evmtypes.ModuleName, // "evm" -- NOT the "vm" store key
		feemarkettypes.ModuleName,
		precisebanktypes.ModuleName,
		erc20types.ModuleName,
		"evmigration",
	} {
		delete(fromVM, name)
	}

	newVM, err := config.Handler(sdk.WrapSDKContext(ctx), upgradetypes.Plan{}, fromVM)
	require.NoError(t, err)

	require.Equal(t, uint64(2), newVM["audit"], "mainnet must land on audit v2, same as testnet")
	require.Equal(t, uint64(1), newVM["supernode"], "mainnet must land on supernode v1, same as testnet")
	require.Equal(t, uint64(1), newVM["evmigration"], "mainnet must land on evmigration v1, same as testnet")

	for _, name := range []string{
		evmtypes.ModuleName,
		feemarkettypes.ModuleName,
		precisebanktypes.ModuleName,
		erc20types.ModuleName,
	} {
		require.Contains(t, newVM, name, "the mainnet one-hop must register the %s module", name)
	}
}
