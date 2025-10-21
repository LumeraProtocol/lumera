
package supernode

import (
	"cosmossdk.io/core/store"
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/depinject"
	"cosmossdk.io/depinject/appconfig"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

var _ depinject.OnePerModuleType = AppModule{}

// IsOnePerModuleType implements the depinject.OnePerModuleType interface.
func (AppModule) IsOnePerModuleType() {}

// ----------------------------------------------------------------------------
// App Wiring Setup
// ----------------------------------------------------------------------------

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

	AccountKeeper  types.AccountKeeper
	BankKeeper     types.BankKeeper
	StakingKeeper  types.StakingKeeper
	SlashingKeeper types.SlashingKeeper
}

type ModuleOutputs struct {
	depinject.Out

	SupernodeKeeper types.SupernodeKeeper
	Module          appmodule.AppModule
	Hooks           stakingtypes.StakingHooksWrapper
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
		in.StakingKeeper,
		in.SlashingKeeper,
	)

	m := NewAppModule(
		in.Cdc,
		k,
		in.AccountKeeper,
		in.BankKeeper,
	)

	return ModuleOutputs{
		SupernodeKeeper: &k,
		Module:          m,
		Hooks:           stakingtypes.StakingHooksWrapper{StakingHooks: k.Hooks()},
	}
}
