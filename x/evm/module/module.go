package evmwiring

import (
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/depinject"
	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "cosmossdk.io/store/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	evmtypes "github.com/evmos/evmos/v20/x/evm/types"
	lumevmtypes "github.com/LumeraProtocol/lumera/x/evm/types"
	"github.com/evmos/evmos/v20/x/evm/keeper"
	"github.com/evmos/evmos/v20/x/evm"
	erc20keeper "github.com/evmos/evmos/v20/x/erc20/keeper"
	modulev1 "github.com/LumeraProtocol/lumera/api/lumera/evm/v1/module"
	lumstakingkeeper "github.com/LumeraProtocol/lumera/x/staking/keeper"
)

type Erc20KeeperGetter func() *erc20keeper.Keeper
type LumStakingKeeperGetter func() lumevmtypes.StakingKeeper

// ProvideModule provides all dependencies needed for the evm module AppModule.
func ProvideModule(in ModuleInputs) ModuleOutputs {
	storeKey := storetypes.NewKVStoreKey(evmtypes.StoreKey)
	transientStoreKey := storetypes.NewTransientStoreKey(evmtypes.TransientKey)
	authority := authtypes.NewModuleAddress(govtypes.ModuleName)
	erc20keeper := in.Erc20Getter()

    k := keeper.NewKeeper(
        in.Codec,                // codec
        storeKey,             	 // store key
        transientStoreKey,	 	 // transient store key
        authority,			     // authority
        in.AccountKeeper,        // account keeper
        in.BankKeeper,           // bank keeper
        in.LumStakingKeeper,     // staking keeper
        in.FeeMarketKeeper,      // fee market keeper
        erc20keeper,   	     	 // erc20 keeper
        "",                      // tracer (empty string if not using)
        in.Subspace,             // params subspace
    )
	return ModuleOutputs{
		EvmKeeper: k,
		AppModule: evm.NewAppModule(
			k,
			in.AccountKeeper,
			in.Subspace,
		),
	}
}

// ModuleInputs declares the dependencies required by the EVM module.
type ModuleInputs struct {
	depinject.In

	Config       *modulev1.Module
	Codec	      codec.Codec

	Erc20Getter  Erc20KeeperGetter
	AccountKeeper evmtypes.AccountKeeper
	BankKeeper    evmtypes.BankKeeper
	LumStakingKeeper *lumstakingkeeper.Keeper
	FeeMarketKeeper evmtypes.FeeMarketKeeper
	Subspace 	  paramstypes.Subspace `cosmos:"evm"`
}

// ModuleOutputs declares what this module provides.
type ModuleOutputs struct {
	depinject.Out

	EvmKeeper *keeper.Keeper
	AppModule appmodule.AppModule
}

func init() {
	appmodule.Register(
		&modulev1.Module{},
		appmodule.Provide(ProvideModule),
	)
}
