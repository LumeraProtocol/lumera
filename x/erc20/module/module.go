package erc20wiring

import (
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/depinject"
	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "cosmossdk.io/store/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	"github.com/evmos/evmos/v20/x/erc20/keeper"
	"github.com/evmos/evmos/v20/x/erc20"
	erc20types "github.com/evmos/evmos/v20/x/erc20/types"
	evmmodulekeeper "github.com/evmos/evmos/v20/x/evm/keeper"
	lumevmtypes "github.com/LumeraProtocol/lumera/x/evm/types"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	transferkeeper "github.com/evmos/evmos/v20/x/ibc/transfer/keeper"
	modulev1 "github.com/LumeraProtocol/lumera/api/lumera/erc20/v1/module"
)

// ModuleInputs defines inputs for the erc20 module wiring.
type ModuleInputs struct {
	depinject.In

	Config         *modulev1.Module
	Codec          codec.Codec
	AccountKeeperERC20  erc20types.AccountKeeper
	AccountKeeper  authkeeper.AccountKeeper
	BankKeeper     bankkeeper.Keeper
	EvmKeeper      *evmmodulekeeper.Keeper
	StakingKeeper  lumevmtypes.StakingKeeper
	AuthzKeeper    authzkeeper.Keeper
	TransferKeeper *transferkeeper.Keeper `optional:"true"`
	Subspace       paramstypes.Subspace `cosmos:"erc20"`
}

// ModuleOutputs defines what this module provides.
type ModuleOutputs struct {
	depinject.Out

	Keeper    keeper.Keeper
	AppModule appmodule.AppModule
}

// ProvideModule constructs the ERC20 Keeper.
func ProvideModule(in ModuleInputs) ModuleOutputs {
	storeKey := storetypes.NewKVStoreKey(erc20types.StoreKey)
	authority := authtypes.NewModuleAddress(govtypes.ModuleName)

	k := keeper.NewKeeper(
		storeKey,
		in.Codec,
		authority,
		in.AccountKeeper,
		in.BankKeeper,
		in.EvmKeeper,
		in.StakingKeeper,
		in.AuthzKeeper,
		in.TransferKeeper,
	)

	return ModuleOutputs{
		Keeper:    k,
		AppModule: erc20.NewAppModule(k, in.AccountKeeper, in.Subspace),
	}
}

func init() {
	appmodule.Register(
		&modulev1.Module{},
		appmodule.Provide(ProvideModule),
	)
}
