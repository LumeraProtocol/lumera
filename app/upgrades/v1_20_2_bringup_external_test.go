package upgrades_test

import (
	"testing"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/stretchr/testify/require"

	lumeraapp "github.com/LumeraProtocol/lumera/app"
	appevm "github.com/LumeraProtocol/lumera/app/evm"
	"github.com/LumeraProtocol/lumera/app/upgrades"
	appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
	evmigrationtypes "github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// v1.20.2's on-chain name. Defined locally because this external test package
// cannot see the unexported constant in package upgrades.
const upgradeNameV1202 = "v1.20.2"

// newV1202Params builds the real keeper wiring a coordinated upgrade would have.
func newV1202Params(app *lumeraapp.App, chainID string) appParams.AppUpgradeParams {
	return appParams.AppUpgradeParams{
		ChainID:           chainID,
		Logger:            log.NewNopLogger(),
		ModuleManager:     module.NewManager(),
		Configurator:      module.NewConfigurator(nil, nil, nil),
		BankKeeper:        app.BankKeeper,
		EVMKeeper:         app.EVMKeeper,
		FeeMarketKeeper:   &app.FeeMarketKeeper,
		Erc20Keeper:       &app.Erc20Keeper,
		Erc20StoreKey:     app.GetKey(erc20types.StoreKey),
		EvmigrationKeeper: &app.EvmigrationKeeper,
	}
}

// allEVMModulesPresent is the fromVM shape a chain already running v1.20.1
// presents (testnet). Versions mirror what the bring-up registers.
func allEVMModulesPresent() module.VersionMap {
	return module.VersionMap{
		evmtypes.ModuleName:         1,
		feemarkettypes.ModuleName:   1,
		precisebanktypes.ModuleName: 1,
		erc20types.ModuleName:       1,
	}
}

// TestV1202MainnetOneHopRunsFullEVMBringup proves the mainnet path.
//
// Mainnet is on v1.12.0 with NO EVM modules and has executed neither v1.20.0
// nor v1.20.1, so v1.20.2 is its first and only EVM boundary. Everything the
// bring-up would have done must therefore happen here: Lumera EVM params
// (overwriting cosmos/evm's "aatom" upstream defaults), feemarket params,
// erc20 params, and a finite migration_end_time.
//
// If this regresses, mainnet either panics at upgrade or comes up with an
// EVM stack configured for the wrong denom.
func TestV1202MainnetOneHopRunsFullEVMBringup(t *testing.T) {
	app := lumeraapp.Setup(t)
	ctx := app.BaseApp.NewContext(false).WithChainID("lumera-mainnet-1")

	// Clobber EVM params to upstream defaults so a passing assertion proves the
	// handler actually re-applied Lumera's params rather than finding them set.
	require.NoError(t, app.EVMKeeper.SetParams(ctx, evmtypes.DefaultParams()))

	params := newV1202Params(app, "lumera-mainnet-1")

	config, found := upgrades.SetupUpgrades(upgradeNameV1202, params)
	require.True(t, found, "v1.20.2 must be registered")
	require.NotNil(t, config.Handler, "v1.20.2 must carry a handler on mainnet")
	require.NotNil(t, config.StoreUpgrade, "v1.20.2 must mount the EVM stores on the mainnet one-hop")

	// The evmigration store is mounted for the first time on this path. Omitting
	// it panics at load with "version of store evmigration mismatch".
	require.Contains(t, config.StoreUpgrade.Added, evmigrationtypes.StoreKey)

	wantEnd := ctx.BlockTime().AddDate(0, 3, 0).Unix()

	// fromVM is EMPTY: mainnet carries no EVM module versions at 1.12.0.
	newVM, err := config.Handler(sdk.WrapSDKContext(ctx), upgradetypes.Plan{}, module.VersionMap{})
	require.NoError(t, err, "the mainnet 1.12.0 -> 1.20.2 one-hop must succeed")
	require.NotNil(t, newVM)

	require.Equal(t, appevm.LumeraEVMGenesisState().Params, app.EVMKeeper.GetParams(ctx),
		"v1.20.2 on the mainnet one-hop must apply Lumera EVM params, overwriting upstream aatom defaults")
	require.True(t,
		app.FeeMarketKeeper.GetParams(ctx).BaseFee.Equal(sdkmath.LegacyMustNewDecFromStr("0.0125")),
		"v1.20.2 on the mainnet one-hop must initialize the five-times-higher base fee")

	emParams, err := app.EvmigrationKeeper.Params.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, wantEnd, emParams.MigrationEndTime,
		"v1.20.2 on mainnet must set migration_end_time to upgrade block time + 3 months")
	require.True(t, emParams.EnableMigration,
		"enable_migration stays at its module default on this release; it is not forced off by the handler")
}

// TestV1202TestnetPreservesStateAndUpdatesBaseFee proves the testnet path.
//
// Testnet already executed v1.20.0 AND v1.20.1, so both are spent and cannot
// run again. v1.20.2 must NOT re-run the bring-up, because doing so would
// re-initialize unrelated EVM params and stomp the live migration_end_time that
// governance is running against. It updates only the feemarket base fee.
func TestV1202TestnetPreservesStateAndUpdatesBaseFee(t *testing.T) {
	app := lumeraapp.Setup(t)
	ctx := app.BaseApp.NewContext(false).WithChainID("lumera-testnet-2")

	params := newV1202Params(app, "lumera-testnet-2")

	// Seed a live migration deadline the way testnet has one today, then assert
	// the upgrade leaves it untouched.
	emParams, err := app.EvmigrationKeeper.Params.Get(ctx)
	require.NoError(t, err)
	const liveDeadline int64 = 1790940497 // observed on lumera-testnet-2, 2026-08-01
	emParams.MigrationEndTime = liveDeadline
	require.NoError(t, app.EvmigrationKeeper.Params.Set(ctx, emParams))

	feeParams := app.FeeMarketKeeper.GetParams(ctx)
	feeParams.BaseFee = sdkmath.LegacyMustNewDecFromStr("0.0025")
	require.NoError(t, app.FeeMarketKeeper.SetParams(ctx, feeParams))
	wantFeeParams := feeParams
	wantFeeParams.BaseFee = sdkmath.LegacyMustNewDecFromStr("0.0125")

	config, found := upgrades.SetupUpgrades(upgradeNameV1202, params)
	require.True(t, found)
	require.NotNil(t, config.Handler)

	newVM, err := config.Handler(sdk.WrapSDKContext(ctx), upgradetypes.Plan{}, allEVMModulesPresent())
	require.NoError(t, err, "the testnet 1.20.1 -> 1.20.2 upgrade must succeed")
	require.NotNil(t, newVM)

	after, err := app.EvmigrationKeeper.Params.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, liveDeadline, after.MigrationEndTime,
		"the existing-EVM path must NOT recompute migration_end_time and stomp the live governance deadline")
	require.Equal(t, wantFeeParams, app.FeeMarketKeeper.GetParams(ctx),
		"v1.20.2 must increase the base fee five times without changing other feemarket params")
}

// TestV1202StoreUpgradeIsAddOnlyOnBothPaths guards the destructive direction.
//
// The add-only store loader mounts missing keys and never deletes. A Deleted or
// Renamed entry appearing here would silently destroy committed state on a live
// chain, so assert the declaration stays purely additive on every network.
func TestV1202StoreUpgradeIsAddOnlyOnBothPaths(t *testing.T) {
	app := lumeraapp.Setup(t)

	for _, chainID := range []string{"lumera-mainnet-1", "lumera-testnet-2", "lumera-devnet-1"} {
		config, found := upgrades.SetupUpgrades(upgradeNameV1202, newV1202Params(app, chainID))
		require.True(t, found, "v1.20.2 must be registered on %s", chainID)
		require.NotNil(t, config.StoreUpgrade, "v1.20.2 must declare stores on %s", chainID)
		require.Empty(t, config.StoreUpgrade.Deleted, "v1.20.2 must delete no store on %s", chainID)
		require.Empty(t, config.StoreUpgrade.Renamed, "v1.20.2 must rename no store on %s", chainID)
	}
}

// TestV1202IsIdempotentAcrossReplay proves replay safety.
//
// A validator that crashes mid-upgrade and restarts replays the upgrade block.
// Running the handler twice against the same state must converge on the same
// result rather than erroring or producing different params.
func TestV1202IsIdempotentAcrossReplay(t *testing.T) {
	app := lumeraapp.Setup(t)
	ctx := app.BaseApp.NewContext(false).WithChainID("lumera-mainnet-1")
	params := newV1202Params(app, "lumera-mainnet-1")

	config, found := upgrades.SetupUpgrades(upgradeNameV1202, params)
	require.True(t, found)

	_, err := config.Handler(sdk.WrapSDKContext(ctx), upgradetypes.Plan{}, module.VersionMap{})
	require.NoError(t, err)

	firstEVM := app.EVMKeeper.GetParams(ctx)
	firstFeeMarket := app.FeeMarketKeeper.GetParams(ctx)
	firstEM, err := app.EvmigrationKeeper.Params.Get(ctx)
	require.NoError(t, err)

	// Replay the SAME arrival shape against the now-upgraded state.
	_, err = config.Handler(sdk.WrapSDKContext(ctx), upgradetypes.Plan{}, module.VersionMap{})
	require.NoError(t, err, "replaying the upgrade must not error")

	require.Equal(t, firstEVM, app.EVMKeeper.GetParams(ctx),
		"replay must not change EVM params")
	require.Equal(t, firstFeeMarket, app.FeeMarketKeeper.GetParams(ctx),
		"replay must not change feemarket params")
	secondEM, err := app.EvmigrationKeeper.Params.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, firstEM.EnableMigration, secondEM.EnableMigration,
		"replay must not flip the migration gate")
}
