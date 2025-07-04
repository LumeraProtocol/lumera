package action

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
	snkeeper "github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"

	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	"github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/action/v1/types"
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

	Config       *Module
	StoreService store.KVStoreService
	Cdc          codec.Codec
	AddressCodec address.Codec
	Logger       log.Logger

	AuthKeeper         types.AuthKeeper
	BankKeeper         types.BankKeeper
	StakingKeeper      types.StakingKeeper
	distributionKeeper types.DistributionKeeper
	SupernodeKeeper    sntypes.SupernodeKeeper
	IBCKeeperFn 	   func() *ibckeeper.Keeper `optional:"true"`
}

type ModuleOutputs struct {
	depinject.Out

	ActionKeeper keeper.Keeper
	Module       appmodule.AppModule
}

func ProvideModule(in ModuleInputs) ModuleOutputs {
	// default to governance authority if not provided
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
		in.BankKeeper,
		in.AuthKeeper,
		in.StakingKeeper,
		in.distributionKeeper,
		in.SupernodeKeeper,
		func () sntypes.QueryServer {
			return snkeeper.NewQueryServerImpl(in.SupernodeKeeper)
		},
		in.IBCKeeperFn,
	)

	m := NewAppModule(
		in.Cdc,
		k,
		in.AuthKeeper,
		in.BankKeeper,
		in.StakingKeeper,
		in.distributionKeeper,
		in.SupernodeKeeper,
	)

	return ModuleOutputs{ActionKeeper: k, Module: m}
}
