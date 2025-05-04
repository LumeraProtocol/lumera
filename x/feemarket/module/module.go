package feemarketwiring

import (
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/depinject"
	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "cosmossdk.io/store/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	"github.com/evmos/evmos/v20/x/feemarket/keeper"
	"github.com/evmos/evmos/v20/x/feemarket"
	feemarkettypes "github.com/evmos/evmos/v20/x/feemarket/types"
	modulev1 "github.com/LumeraProtocol/lumera/api/lumera/feemarket/v1/module"
)

// ProvideModule constructs the FeeMarket Keeper.
func ProvideModule(in ModuleInputs) ModuleOutputs {
	storeKey := storetypes.NewKVStoreKey(feemarkettypes.StoreKey)
	transientStoreKey := storetypes.NewTransientStoreKey(feemarkettypes.TransientKey)
	authority := authtypes.NewModuleAddress(govtypes.ModuleName)

	k := keeper.NewKeeper(
		in.Codec,
		authority,
		storeKey,
		transientStoreKey,
		in.Subspace,
	)

	return ModuleOutputs{
		FeeMarketKeeper: k,
		AppModule: feemarket.NewAppModule(
			k,
			in.Subspace,
		),
	}
}

func init() {
	appmodule.Register(
		&modulev1.Module{},
		appmodule.Provide(ProvideModule),
	)
}

// ModuleInputs defines inputs for the feemarket module wiring.
type ModuleInputs struct {
	depinject.In

	Config       *modulev1.Module
	Codec        codec.Codec
	Subspace     paramstypes.Subspace `cosmos:"feemarket"`
}

// ModuleOutputs defines what this module provides.
type ModuleOutputs struct {
	depinject.Out

	FeeMarketKeeper    keeper.Keeper
	AppModule appmodule.AppModule
}