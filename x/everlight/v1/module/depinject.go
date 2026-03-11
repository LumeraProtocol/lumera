package everlight

import (
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/store"
	"cosmossdk.io/depinject"
	"cosmossdk.io/depinject/appconfig"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	"github.com/LumeraProtocol/lumera/x/everlight/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/everlight/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

var _ depinject.OnePerModuleType = AppModule{}

// IsOnePerModuleType implements the depinject.OnePerModuleType interface.
func (AppModule) IsOnePerModuleType() {}

func init() {
	appconfig.Register(
		&Module{},
		appconfig.Provide(ProvideModule),
	)
}

type ModuleInputs struct {
	depinject.In

	StoreService store.KVStoreService
	Cdc          codec.Codec
	Config       *Module
	Logger       log.Logger

	AccountKeeper   types.AccountKeeper
	BankKeeper      types.BankKeeper
	SupernodeKeeper sntypes.SupernodeKeeper
}

type ModuleOutputs struct {
	depinject.Out

	EverlightKeeper keeper.Keeper
	Module          appmodule.AppModule
}

func ProvideModule(in ModuleInputs) ModuleOutputs {
	authority := authtypes.NewModuleAddress(govtypes.ModuleName)
	if in.Config.Authority != "" {
		authority = authtypes.NewModuleAddressOrBech32Address(in.Config.Authority)
	}

	k := keeper.NewKeeper(
		in.Cdc,
		in.StoreService,
		in.Logger,
		authority.String(),
		in.BankKeeper,
		in.AccountKeeper,
		in.SupernodeKeeper,
	)

	m := NewAppModule(in.Cdc, k)

	return ModuleOutputs{
		EverlightKeeper: k,
		Module:          m,
	}
}
