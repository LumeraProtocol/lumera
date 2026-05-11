package evmigration

import (
	"cosmossdk.io/core/address"
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/store"
	"cosmossdk.io/depinject"
	"cosmossdk.io/depinject/appconfig"
	feegrantkeeper "cosmossdk.io/x/feegrant/keeper"
	"github.com/cosmos/cosmos-sdk/codec"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"

	actionkeeper "github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	claimkeeper "github.com/LumeraProtocol/lumera/x/claim/keeper"
	"github.com/LumeraProtocol/lumera/x/evmigration/keeper"
	"github.com/LumeraProtocol/lumera/x/evmigration/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

var _ depinject.OnePerModuleType = AppModule{}

// IsOnePerModuleType implements the depinject.OnePerModuleType interface.
func (AppModule) IsOnePerModuleType() {}

func init() {
	appconfig.Register(
		&types.Module{},
		appconfig.Provide(ProvideModule),
	)
}

// ModuleInputs uses the exact types provided by each module's depinject output
// so that the DI container can resolve them without ambiguity.
type ModuleInputs struct {
	depinject.In

	Config       *types.Module
	StoreService store.KVStoreService
	Cdc          codec.Codec
	AddressCodec address.Codec

	AccountKeeper      authkeeper.AccountKeeper
	BankKeeper         bankkeeper.BaseKeeper
	StakingKeeper      *stakingkeeper.Keeper
	DistributionKeeper distrkeeper.Keeper
	AuthzKeeper        authzkeeper.Keeper
	FeegrantKeeper     feegrantkeeper.Keeper
	SupernodeKeeper    sntypes.SupernodeKeeper
	ActionKeeper       actionkeeper.Keeper
	ClaimKeeper        claimkeeper.Keeper
}

type ModuleOutputs struct {
	depinject.Out

	EvmigrationKeeper keeper.Keeper
	Module            appmodule.AppModule
}

func ProvideModule(in ModuleInputs) ModuleOutputs {
	// default to governance authority if not provided
	authority := authtypes.NewModuleAddress(types.GovModuleName)
	if in.Config.Authority != "" {
		authority = authtypes.NewModuleAddressOrBech32Address(in.Config.Authority)
	}
	k := keeper.NewKeeper(
		in.StoreService,
		in.Cdc,
		in.AddressCodec,
		authority,
		in.AccountKeeper,
		in.BankKeeper,
		in.StakingKeeper,
		in.DistributionKeeper,
		in.AuthzKeeper,
		in.FeegrantKeeper,
		in.SupernodeKeeper,
		&in.ActionKeeper,
		&in.ClaimKeeper,
	)
	m := NewAppModule(in.Cdc, k)

	return ModuleOutputs{EvmigrationKeeper: k, Module: m}
}
