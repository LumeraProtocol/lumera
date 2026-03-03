package audit

import (
	"cosmossdk.io/core/address"
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/store"
	"cosmossdk.io/depinject"
	"cosmossdk.io/depinject/appconfig"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	audittypes "github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

var _ depinject.OnePerModuleType = AppModule{}

func (AppModule) IsOnePerModuleType() {}

func init() {
	appconfig.Register(
		&Module{},
		appconfig.Provide(ProvideModule),
	)
}

type ModuleInputs struct {
	depinject.In

	Config       *Module
	StoreService store.KVStoreService
	Cdc          codec.Codec
	AddressCodec address.Codec
	Logger       log.Logger

	SupernodeKeeper sntypes.SupernodeKeeper
	AuthKeeper      audittypes.AuthKeeper
	BankKeeper      audittypes.BankKeeper
}

type ModuleOutputs struct {
	depinject.Out

	AuditKeeper keeper.Keeper
	Module      appmodule.AppModule
}

func ProvideModule(in ModuleInputs) ModuleOutputs {
	authority := authtypes.NewModuleAddress(govtypes.ModuleName)
	if in.Config.Authority != "" {
		authority = authtypes.NewModuleAddressOrBech32Address(in.Config.Authority)
	}

	k := keeper.NewKeeper(
		in.Cdc,
		in.AddressCodec,
		in.StoreService,
		in.Logger,
		authority,
		in.SupernodeKeeper,
	)

	m := NewAppModule(in.Cdc, k, in.AuthKeeper, in.BankKeeper)

	return ModuleOutputs{AuditKeeper: k, Module: m}
}
