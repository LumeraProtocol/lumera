package v1_12_0_test

import (
	"testing"

	"cosmossdk.io/log"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	"github.com/stretchr/testify/require"

	lumeraapp "github.com/LumeraProtocol/lumera/app"
	appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
	upgradev1120 "github.com/LumeraProtocol/lumera/app/upgrades/v1_12_0"
)

func TestV1120InitializesERC20ParamsWhenInitGenesisIsSkipped(t *testing.T) {
	app := lumeraapp.Setup(t)
	ctx := app.BaseApp.NewContext(false)

	store := ctx.KVStore(app.GetKey(erc20types.StoreKey))
	store.Delete(erc20types.ParamStoreKeyEnableErc20)
	store.Delete(erc20types.ParamStoreKeyPermissionlessRegistration)

	// The empty erc20 store reads back as both flags disabled until InitGenesis
	// or SetParams writes the keys.
	require.Equal(t, erc20types.NewParams(false, false), app.Erc20Keeper.GetParams(ctx))

	handler := upgradev1120.CreateUpgradeHandler(appParams.AppUpgradeParams{
		Logger:          log.NewNopLogger(),
		ModuleManager:   module.NewManager(),
		Configurator:    module.NewConfigurator(nil, nil, nil),
		BankKeeper:      app.BankKeeper,
		EVMKeeper:       app.EVMKeeper,
		FeeMarketKeeper: &app.FeeMarketKeeper,
		Erc20Keeper:     &app.Erc20Keeper,
	})

	_, err := handler(sdk.WrapSDKContext(ctx), upgradetypes.Plan{}, module.VersionMap{})
	require.NoError(t, err)
	require.Equal(t, erc20types.DefaultParams(), app.Erc20Keeper.GetParams(ctx))
}
