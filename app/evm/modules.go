package evm

import (
	"encoding/json"

	"cosmossdk.io/core/appmodule"

	"github.com/cosmos/cosmos-sdk/codec"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/types/module"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"

	erc20module "github.com/cosmos/evm/x/erc20"
	erc20keeper "github.com/cosmos/evm/x/erc20/keeper"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarket "github.com/cosmos/evm/x/feemarket"
	feemarketkeeper "github.com/cosmos/evm/x/feemarket/keeper"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	precisebank "github.com/cosmos/evm/x/precisebank"
	precisebankkeeper "github.com/cosmos/evm/x/precisebank/keeper"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	evmmodule "github.com/cosmos/evm/x/vm"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	lcfg "github.com/LumeraProtocol/lumera/config"
)

// RegisterModules registers non-depinject EVM modules for CLI-side module basics and AutoCLI.
// Wrapper types override DefaultGenesis for evm and feemarket so that CLI-generated
// genesis files (lumerad init, lumerad testnet init-files) use Lumera denoms and fee
// settings instead of upstream defaults (aatom, base_fee=1Gwei).
func RegisterModules(cdc codec.Codec) map[string]appmodule.AppModule {
	var (
		bankKeeper    precisebanktypes.BankKeeper
		accountKeeper precisebanktypes.AccountKeeper
	)

	modules := map[string]appmodule.AppModule{
		feemarkettypes.ModuleName:   lumeraFeemarketModule{feemarket.NewAppModule(feemarketkeeper.Keeper{})},
		precisebanktypes.ModuleName: precisebank.NewAppModule(precisebankkeeper.Keeper{}, bankKeeper, accountKeeper),
		evmtypes.ModuleName:         lumeraEVMModule{evmmodule.NewAppModule(nil, nil, nil, addresscodec.NewBech32Codec(lcfg.Bech32AccountAddressPrefix))},
		erc20types.ModuleName:       erc20module.NewAppModule(erc20keeper.Keeper{}, authkeeper.AccountKeeper{}),
	}

	for _, m := range modules {
		if mr, ok := m.(module.AppModuleBasic); ok {
			mr.RegisterInterfaces(cdc.InterfaceRegistry())
		}
	}

	return modules
}

// lumeraEVMModule wraps the upstream EVM AppModule to override DefaultGenesis
// with Lumera-specific denominations (ulume/alume instead of uatom/aatom).
type lumeraEVMModule struct {
	evmmodule.AppModule
}

func (lumeraEVMModule) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	return cdc.MustMarshalJSON(LumeraEVMGenesisState())
}

// lumeraFeemarketModule wraps the upstream feemarket AppModule to override
// DefaultGenesis with Lumera settings (dynamic base fee enabled).
type lumeraFeemarketModule struct {
	feemarket.AppModule
}

func (lumeraFeemarketModule) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	return cdc.MustMarshalJSON(LumeraFeemarketGenesisState())
}
