package v1_20_0_test

import (
	"testing"
	"time"

	"cosmossdk.io/log"
	"cosmossdk.io/store/prefix"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	"github.com/stretchr/testify/require"

	lumeraapp "github.com/LumeraProtocol/lumera/app"
	appevm "github.com/LumeraProtocol/lumera/app/evm"
	appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
	upgradev1200 "github.com/LumeraProtocol/lumera/app/upgrades/v1_20_0"
	erc20policytypes "github.com/LumeraProtocol/lumera/x/erc20policy/types"
)

// upgradeParamsForChain returns AppUpgradeParams wired with the app keepers the
// v1.20.0 handler needs, for the given chain ID.
func upgradeParamsForChain(app *lumeraapp.App, chainID string) appParams.AppUpgradeParams {
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

// On devnet the handler derives a finite migration_end_time from the upgrade
// block time (block time + 2 days) so rehearsals run against a real deadline.
func TestV1200SetsDevnetMigrationEndTime(t *testing.T) {
	app := lumeraapp.Setup(t)
	// The handler identifies the network from the SDK context (genesis-derived
	// chain ID), not the app-level ChainID captured during setup — which on a
	// default `lumerad start` is the non-empty "lumera" flag default. Seed the
	// context with the real chain ID and pass the misleading default in params
	// to pin that ctx.ChainID() is what's used.
	ctx := app.BaseApp.NewContext(false).WithChainID("lumera-devnet-1")

	// Default genesis params seed migration with no deadline.
	before, err := app.EvmigrationKeeper.Params.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(0), before.MigrationEndTime)

	want := ctx.BlockTime().Add(2 * 24 * time.Hour).Unix()

	handler := upgradev1200.CreateUpgradeHandler(upgradeParamsForChain(app, "lumera"))
	_, err = handler(sdk.WrapSDKContext(ctx), upgradetypes.Plan{}, module.VersionMap{})
	require.NoError(t, err)

	after, err := app.EvmigrationKeeper.Params.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, want, after.MigrationEndTime,
		"devnet upgrade should set migration_end_time to upgrade block time + 2 days")
	require.True(t, after.EnableMigration, "enable_migration should remain true (immediate-open)")
	require.Equal(t, uint64(2500), after.MaxValidatorDelegations,
		"max_validator_delegations default should be 2500")
}

// On testnet the handler derives a 3-calendar-month migration window from the
// upgrade block time.
func TestV1200SetsTestnetMigrationEndTime(t *testing.T) {
	app := lumeraapp.Setup(t)
	// See TestV1200SetsDevnetMigrationEndTime: network is taken from ctx.ChainID().
	ctx := app.BaseApp.NewContext(false).WithChainID("lumera-testnet-1")

	want := ctx.BlockTime().AddDate(0, 3, 0).Unix()

	handler := upgradev1200.CreateUpgradeHandler(upgradeParamsForChain(app, "lumera"))
	_, err := handler(sdk.WrapSDKContext(ctx), upgradetypes.Plan{}, module.VersionMap{})
	require.NoError(t, err)

	after, err := app.EvmigrationKeeper.Params.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, want, after.MigrationEndTime,
		"testnet upgrade should set migration_end_time to upgrade block time + 3 months")
}

// On mainnet the handler derives a 3-calendar-month migration window from the
// upgrade block time, the same window as testnet.
func TestV1200SetsMainnetMigrationEndTime(t *testing.T) {
	app := lumeraapp.Setup(t)
	// See TestV1200SetsDevnetMigrationEndTime: network is taken from ctx.ChainID().
	// This is the case the review flagged — the app-level ChainID would be the
	// "lumera" default, leaving the mainnet deadline silently unset if used.
	ctx := app.BaseApp.NewContext(false).WithChainID("lumera-mainnet-1")

	// Default genesis params seed migration with no deadline.
	before, err := app.EvmigrationKeeper.Params.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(0), before.MigrationEndTime)

	want := ctx.BlockTime().AddDate(0, 3, 0).Unix()

	handler := upgradev1200.CreateUpgradeHandler(upgradeParamsForChain(app, "lumera"))
	_, err = handler(sdk.WrapSDKContext(ctx), upgradetypes.Plan{}, module.VersionMap{})
	require.NoError(t, err)

	after, err := app.EvmigrationKeeper.Params.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, want, after.MigrationEndTime,
		"mainnet upgrade should set migration_end_time to upgrade block time + 3 months")
}

// The bare "lumera" CLI default is not a real network ID; the handler must not
// treat it as one and must leave migration_end_time unset (0).
func TestV1200LeavesMigrationEndTimeUnsetForDefaultChainID(t *testing.T) {
	app := lumeraapp.Setup(t)
	ctx := app.BaseApp.NewContext(false).WithChainID("lumera")

	handler := upgradev1200.CreateUpgradeHandler(upgradeParamsForChain(app, "lumera"))
	_, err := handler(sdk.WrapSDKContext(ctx), upgradetypes.Plan{}, module.VersionMap{})
	require.NoError(t, err)

	after, err := app.EvmigrationKeeper.Params.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(0), after.MigrationEndTime,
		"unrecognized chain ID must leave migration_end_time at the default 0")
}

func TestV1200InitializesERC20ParamsWhenInitGenesisIsSkipped(t *testing.T) {
	app := lumeraapp.Setup(t)
	ctx := app.BaseApp.NewContext(false)

	store := ctx.KVStore(app.GetKey(erc20types.StoreKey))
	store.Delete(erc20types.ParamStoreKeyEnableErc20)
	store.Delete(erc20types.ParamStoreKeyPermissionlessRegistration)

	// The empty erc20 store reads back as both flags disabled until InitGenesis
	// or SetParams writes the keys.
	require.Equal(t, erc20types.NewParams(false, false), app.Erc20Keeper.GetParams(ctx))

	erc20StoreKey := app.GetKey(erc20types.StoreKey)

	handler := upgradev1200.CreateUpgradeHandler(appParams.AppUpgradeParams{
		Logger:          log.NewNopLogger(),
		ModuleManager:   module.NewManager(),
		Configurator:    module.NewConfigurator(nil, nil, nil),
		BankKeeper:      app.BankKeeper,
		EVMKeeper:       app.EVMKeeper,
		FeeMarketKeeper: &app.FeeMarketKeeper,
		Erc20Keeper:     &app.Erc20Keeper,
		Erc20StoreKey:   erc20StoreKey,
	})

	_, err := handler(sdk.WrapSDKContext(ctx), upgradetypes.Plan{}, module.VersionMap{})
	require.NoError(t, err)
	require.Equal(t, appevm.LumeraERC20DefaultParams(), app.Erc20Keeper.GetParams(ctx))

	// Verify the ERC20 registration policy was initialized.
	erc20Store := ctx.KVStore(erc20StoreKey)
	require.True(t, erc20Store.Has(erc20policytypes.PolicyModeKey), "policy mode key should be set")
	require.Equal(t, erc20policytypes.PolicyModeAllowlist, string(erc20Store.Get(erc20policytypes.PolicyModeKey)))

	// Verify default base denom traces are in the allowlist (empty traces = inert placeholders).
	tracePfxStore := prefix.NewStore(erc20Store, erc20policytypes.PolicyAllowBaseTracePfx)
	for _, entry := range erc20policytypes.DefaultAllowedBaseDenomTraces {
		traceKey := erc20policytypes.EncodeTraceKey(entry.Trace)
		key := append([]byte(entry.BaseDenom), 0x00)
		key = append(key, traceKey...)
		require.True(t, tracePfxStore.Has(key), "base denom trace %s should be in allowlist", entry.BaseDenom)
	}
}
