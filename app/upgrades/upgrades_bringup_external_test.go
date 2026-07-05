package upgrades_test

import (
	"testing"

	"cosmossdk.io/log"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/stretchr/testify/require"

	lumeraapp "github.com/LumeraProtocol/lumera/app"
	appevm "github.com/LumeraProtocol/lumera/app/evm"
	"github.com/LumeraProtocol/lumera/app/upgrades"
	appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
)

// v1.20.1's on-chain name. Defined locally because this external test package
// cannot see the unexported constant in package upgrades.
const upgradeNameV1201 = "v1.20.1"

// TestV1201MainnetRunsFullEVMBringup exercises the real v1.20.1 handler wiring
// on a mainnet context and proves it performs the same EVM finalization the
// v1.20.0 handler would have: it re-applies the Lumera EVM params (overwriting
// upstream defaults) and sets the 3-calendar-month migration deadline.
//
// This locks the user's requirement — "everything that runs in v1.20.0 must run
// in v1.20.1" on mainnet — at the SetupUpgrades entry point, not just at the
// v1.20.0 handler in isolation.
func TestV1201MainnetRunsFullEVMBringup(t *testing.T) {
	app := lumeraapp.Setup(t)
	ctx := app.BaseApp.NewContext(false).WithChainID("lumera-mainnet-1")

	// Clobber EVM params to upstream defaults (extended "aatom" denom) so a passing
	// assertion proves the handler actually re-applied Lumera's params.
	require.NoError(t, app.EVMKeeper.SetParams(ctx, evmtypes.DefaultParams()))

	// A coordinated mainnet upgrade starts the node with its real chain ID, so the
	// setup-time params.ChainID that gates store mounting is "lumera-mainnet-1"
	// (the same assumption the v1.8.4 mainnet store upgrade relies on).
	params := appParams.AppUpgradeParams{
		ChainID:           "lumera-mainnet-1",
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

	config, found := upgrades.SetupUpgrades(upgradeNameV1201, params)
	require.True(t, found)
	require.NotNil(t, config.Handler, "v1.20.1 must carry a handler on mainnet")
	require.NotNil(t, config.StoreUpgrade, "v1.20.1 must mount the EVM stores on mainnet")

	wantEnd := ctx.BlockTime().AddDate(0, 3, 0).Unix()

	_, err := config.Handler(sdk.WrapSDKContext(ctx), upgradetypes.Plan{}, module.VersionMap{})
	require.NoError(t, err)

	require.Equal(t, appevm.LumeraEVMGenesisState().Params, app.EVMKeeper.GetParams(ctx),
		"v1.20.1 on mainnet should apply the Lumera EVM params, overwriting upstream defaults")

	emParams, err := app.EvmigrationKeeper.Params.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, wantEnd, emParams.MigrationEndTime,
		"v1.20.1 on mainnet should set migration_end_time to upgrade block time + 3 months")
}
