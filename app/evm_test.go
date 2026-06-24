package app

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/types/module"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	appevm "github.com/LumeraProtocol/lumera/app/evm"
	lcfg "github.com/LumeraProtocol/lumera/config"
)

// TestRegisterEVMDefaultGenesis verifies that EVM-related modules are
// registered in module basics and expose Lumera-customized default genesis.
func TestRegisterEVMDefaultGenesis(t *testing.T) {
	t.Parallel()

	encCfg := MakeEncodingConfig(t)

	modules := appevm.RegisterModules(encCfg.Codec)
	require.Contains(t, modules, feemarkettypes.ModuleName)
	require.Contains(t, modules, precisebanktypes.ModuleName)
	require.Contains(t, modules, evmtypes.ModuleName)
	require.Contains(t, modules, erc20types.ModuleName)

	mbm := module.BasicManager{}
	for name, mod := range modules {
		mbm[name] = module.CoreAppModuleBasicAdaptor(name, mod)
	}

	genesis := mbm.DefaultGenesis(encCfg.Codec)
	require.Contains(t, genesis, feemarkettypes.ModuleName)
	require.Contains(t, genesis, precisebanktypes.ModuleName)
	require.Contains(t, genesis, evmtypes.ModuleName)
	require.Contains(t, genesis, erc20types.ModuleName)

	// Feemarket uses Lumera overrides (dynamic base fee enabled).
	var feemarketGenesis feemarkettypes.GenesisState
	encCfg.Codec.MustUnmarshalJSON(genesis[feemarkettypes.ModuleName], &feemarketGenesis)
	require.False(t, feemarketGenesis.Params.NoBaseFee, "feemarket NoBaseFee should be false")
	require.True(
		t,
		feemarketGenesis.Params.BaseFee.Equal(appevm.LumeraFeemarketGenesisState().Params.BaseFee),
		"feemarket BaseFee should match configured Lumera default",
	)

	// EVM uses Lumera denominations.
	require.Contains(t, genesis, evmtypes.ModuleName)
	var evmGenesis evmtypes.GenesisState
	encCfg.Codec.MustUnmarshalJSON(genesis[evmtypes.ModuleName], &evmGenesis)
	require.Equal(t, lcfg.ChainDenom, evmGenesis.Params.EvmDenom, "EVM denom should match chain base denom")
	require.NotNil(t, evmGenesis.Params.ExtendedDenomOptions)
	require.Equal(
		t,
		lcfg.ChainEVMExtendedDenom,
		evmGenesis.Params.ExtendedDenomOptions.ExtendedDenom,
		"EVM extended denom should match chain extended denom",
	)

	var precisebankGenesis precisebanktypes.GenesisState
	encCfg.Codec.MustUnmarshalJSON(genesis[precisebanktypes.ModuleName], &precisebankGenesis)
	require.Equal(t, precisebanktypes.DefaultGenesisState(), &precisebankGenesis)
}

// TestEVMModuleOrderAndPermissions verifies module ordering constraints and
// module-account permissions for EVM stack modules.
func TestEVMModuleOrderAndPermissions(t *testing.T) {
	t.Parallel()

	feemarketGenesisIdx := indexOfModule(genesisModuleOrder, feemarkettypes.ModuleName)
	precisebankGenesisIdx := indexOfModule(genesisModuleOrder, precisebanktypes.ModuleName)
	evmGenesisIdx := indexOfModule(genesisModuleOrder, evmtypes.ModuleName)
	erc20GenesisIdx := indexOfModule(genesisModuleOrder, erc20types.ModuleName)
	genutilGenesisIdx := indexOfModule(genesisModuleOrder, genutiltypes.ModuleName)

	require.NotEqual(t, -1, feemarketGenesisIdx)
	require.NotEqual(t, -1, precisebankGenesisIdx)
	require.NotEqual(t, -1, evmGenesisIdx)
	require.NotEqual(t, -1, erc20GenesisIdx)
	require.NotEqual(t, -1, genutilGenesisIdx)
	// EVM must initialize before dependent EVM modules.
	require.Less(t, evmGenesisIdx, feemarketGenesisIdx)
	require.Less(t, evmGenesisIdx, precisebankGenesisIdx)
	require.Less(t, evmGenesisIdx, erc20GenesisIdx)
	// Feemarket must be initialized before genutil (gentx processing path).
	require.Less(t, feemarketGenesisIdx, genutilGenesisIdx)
	require.Less(t, precisebankGenesisIdx, genutilGenesisIdx)
	require.Less(t, erc20GenesisIdx, genutilGenesisIdx)

	require.NotEqual(t, -1, indexOfModule(beginBlockers, feemarkettypes.ModuleName))
	require.NotEqual(t, -1, indexOfModule(beginBlockers, precisebanktypes.ModuleName))
	require.NotEqual(t, -1, indexOfModule(beginBlockers, evmtypes.ModuleName))
	require.NotEqual(t, -1, indexOfModule(beginBlockers, erc20types.ModuleName))

	require.NotEqual(t, -1, indexOfModule(endBlockers, precisebanktypes.ModuleName))
	require.NotEqual(t, -1, indexOfModule(endBlockers, evmtypes.ModuleName))
	require.NotEqual(t, -1, indexOfModule(endBlockers, erc20types.ModuleName))
	require.Equal(t, feemarkettypes.ModuleName, endBlockers[len(endBlockers)-1])

	maccPerms := GetMaccPerms()
	require.Contains(t, maccPerms, feemarkettypes.ModuleName)
	require.Contains(t, maccPerms, precisebanktypes.ModuleName)
	require.Contains(t, maccPerms, evmtypes.ModuleName)
	require.Contains(t, maccPerms, erc20types.ModuleName)
	require.Len(t, maccPerms[feemarkettypes.ModuleName], 0)
	require.ElementsMatch(t, []string{authtypes.Minter, authtypes.Burner}, maccPerms[precisebanktypes.ModuleName])
	require.ElementsMatch(t, []string{authtypes.Minter, authtypes.Burner}, maccPerms[evmtypes.ModuleName])
	require.ElementsMatch(t, []string{authtypes.Minter, authtypes.Burner}, maccPerms[erc20types.ModuleName])
}

// TestEVMStoresAndModuleAccountsInitialized ensures EVM store keys and module
// accounts are initialized in a fully bootstrapped test app.
func TestEVMStoresAndModuleAccountsInitialized(t *testing.T) {
	app := Setup(t)

	require.NotNil(t, app.GetKey(feemarkettypes.StoreKey))
	require.NotNil(t, app.GetTransientKey(feemarkettypes.TransientKey))
	require.NotNil(t, app.GetKey(precisebanktypes.StoreKey))
	require.NotNil(t, app.GetKey(evmtypes.StoreKey))
	require.NotNil(t, app.GetTransientKey(evmtypes.TransientKey))
	require.NotNil(t, app.GetKey(erc20types.StoreKey))

	genesis := app.DefaultGenesis()
	require.Contains(t, genesis, feemarkettypes.ModuleName)
	require.Contains(t, genesis, precisebanktypes.ModuleName)
	require.Contains(t, genesis, evmtypes.ModuleName)
	require.Contains(t, genesis, erc20types.ModuleName)

	ctx := app.BaseApp.NewContext(false)
	require.NotNil(t, app.AuthKeeper.GetModuleAccount(ctx, feemarkettypes.ModuleName))
	require.NotNil(t, app.AuthKeeper.GetModuleAccount(ctx, precisebanktypes.ModuleName))
	require.NotNil(t, app.AuthKeeper.GetModuleAccount(ctx, evmtypes.ModuleName))
	require.NotNil(t, app.AuthKeeper.GetModuleAccount(ctx, erc20types.ModuleName))
}

// indexOfModule returns index of module name or -1 when absent.
func indexOfModule(modules []string, name string) int {
	for i, moduleName := range modules {
		if moduleName == name {
			return i
		}
	}

	return -1
}
