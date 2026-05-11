package v1_20_0_test

import (
	"testing"

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
