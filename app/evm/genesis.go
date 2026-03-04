package evm

import (
	"cosmossdk.io/math"

	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	lcfg "github.com/LumeraProtocol/lumera/config"
)

// LumeraEVMGenesisState returns the EVM genesis state customized for Lumera.
func LumeraEVMGenesisState() *evmtypes.GenesisState {
	params := evmtypes.DefaultParams()
	params.EvmDenom = lcfg.ChainDenom
	params.ActiveStaticPrecompiles = append([]string{}, LumeraActiveStaticPrecompiles...)
	params.ExtendedDenomOptions = &evmtypes.ExtendedDenomOptions{
		ExtendedDenom: lcfg.ChainEVMExtendedDenom,
	}
	return evmtypes.NewGenesisState(params, []evmtypes.GenesisAccount{}, []evmtypes.Preinstall{})
}

// LumeraFeemarketGenesisState returns the feemarket genesis state customized for Lumera.
// EIP-1559 dynamic base fee is enabled with a chain-specific default base fee,
// a minimum gas price floor to prevent decay to zero, and a gentler change
// denominator for smoother adjustments.
func LumeraFeemarketGenesisState() *feemarkettypes.GenesisState {
	genesis := feemarkettypes.DefaultGenesisState()
	genesis.Params.NoBaseFee = false
	genesis.Params.BaseFee = math.LegacyMustNewDecFromStr(lcfg.FeeMarketDefaultBaseFee)
	genesis.Params.MinGasPrice = math.LegacyMustNewDecFromStr(lcfg.FeeMarketMinGasPrice)
	genesis.Params.BaseFeeChangeDenominator = lcfg.FeeMarketBaseFeeChangeDenominator
	return genesis
}
