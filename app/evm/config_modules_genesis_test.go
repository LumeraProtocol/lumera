package evm_test

import (
	"reflect"
	"testing"

	"github.com/LumeraProtocol/lumera/app/evm"
	lcfg "github.com/LumeraProtocol/lumera/config"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/stretchr/testify/require"
)

// TestConfigureNoOp verifies Configure remains a safe no-op. The global EVM
// config is set by module InitGenesis/PreBlock, not by this helper.
func TestConfigureNoOp(t *testing.T) {
	t.Parallel()

	require.NoError(t, evm.Configure())
}

// TestProvideCustomGetSigners verifies depinject signer registration for
// MsgEthereumTx uses Cosmos EVM's canonical custom signer function.
func TestProvideCustomGetSigners(t *testing.T) {
	t.Parallel()

	custom := evm.ProvideCustomGetSigners()

	require.Equal(t, evmtypes.MsgEthereumTxCustomGetSigner.MsgType, custom.MsgType)
	require.Equal(
		t,
		reflect.ValueOf(evmtypes.MsgEthereumTxCustomGetSigner.Fn).Pointer(),
		reflect.ValueOf(custom.Fn).Pointer(),
	)
}

// TestLumeraGenesisDefaults validates Lumera-specific EVM and feemarket
// genesis overrides (denoms + base fee policy).
func TestLumeraGenesisDefaults(t *testing.T) {
	t.Parallel()

	evmGenesis := evm.LumeraEVMGenesisState()
	require.Equal(t, lcfg.ChainDenom, evmGenesis.Params.EvmDenom)
	require.ElementsMatch(t, evm.LumeraActiveStaticPrecompiles, evmGenesis.Params.ActiveStaticPrecompiles)
	require.NotNil(t, evmGenesis.Params.ExtendedDenomOptions)
	require.Equal(t, lcfg.ChainEVMExtendedDenom, evmGenesis.Params.ExtendedDenomOptions.ExtendedDenom)
	require.Empty(t, evmGenesis.Accounts)
	require.Empty(t, evmGenesis.Preinstalls)

	feeGenesis := evm.LumeraFeemarketGenesisState()
	require.False(t, feeGenesis.Params.NoBaseFee)
	require.True(
		t,
		feeGenesis.Params.BaseFee.Equal(sdkmath.LegacyMustNewDecFromStr(lcfg.FeeMarketDefaultBaseFee)),
	)
}

// TestUpstreamDefaultEvmDenomIsNotLumera documents that cosmos/evm v0.6.0
// DefaultParams().EvmDenom = DefaultEVMExtendedDenom = "aatom", NOT "ulume".
// This is why the v1.12.0 upgrade handler must skip InitGenesis for EVM modules
// (via fromVM pre-population) and manually set Lumera params. If this test
// fails, the upstream default has changed and the upgrade handler may need updating.
func TestUpstreamDefaultEvmDenomIsNotLumera(t *testing.T) {
	t.Parallel()

	upstreamParams := evmtypes.DefaultParams()

	// Upstream EvmDenom must NOT be the Lumera chain denom — if it were,
	// the InitGenesis skip in the upgrade handler would be unnecessary.
	require.NotEqual(t, lcfg.ChainDenom, upstreamParams.EvmDenom,
		"upstream DefaultParams().EvmDenom should differ from Lumera ChainDenom")
	require.Equal(t, evmtypes.DefaultEVMExtendedDenom, upstreamParams.EvmDenom,
		"upstream DefaultParams().EvmDenom should be DefaultEVMExtendedDenom (aatom)")

	// Lumera's genesis state must use the correct denoms.
	lumeraGenesis := evm.LumeraEVMGenesisState()
	require.Equal(t, lcfg.ChainDenom, lumeraGenesis.Params.EvmDenom,
		"Lumera EVM genesis should use ChainDenom (ulume)")
	require.Equal(t, lcfg.ChainEVMExtendedDenom, lumeraGenesis.Params.ExtendedDenomOptions.ExtendedDenom,
		"Lumera EVM genesis should use ChainEVMExtendedDenom (alume)")
}

// TestRegisterModulesMatrix checks EVM module registration wiring used by CLI
// module basics / default genesis generation.
func TestRegisterModulesMatrix(t *testing.T) {
	t.Parallel()

	interfaceRegistry := codectypes.NewInterfaceRegistry()
	lcfg.RegisterExtraInterfaces(interfaceRegistry)
	cdc := codec.NewProtoCodec(interfaceRegistry)

	modules := evm.RegisterModules(cdc)
	require.Len(t, modules, 4)
	require.Contains(t, modules, evmtypes.ModuleName)
	require.Contains(t, modules, feemarkettypes.ModuleName)
	require.Contains(t, modules, precisebanktypes.ModuleName)
	require.Contains(t, modules, erc20types.ModuleName)

	// Wrapper modules should expose Lumera-specific DefaultGenesis content.
	evmBasic, ok := modules[evmtypes.ModuleName].(module.HasGenesisBasics)
	require.True(t, ok)
	var evmGenesis evmtypes.GenesisState
	require.NoError(t, cdc.UnmarshalJSON(evmBasic.DefaultGenesis(cdc), &evmGenesis))
	require.Equal(t, lcfg.ChainDenom, evmGenesis.Params.EvmDenom)
	require.ElementsMatch(t, evm.LumeraActiveStaticPrecompiles, evmGenesis.Params.ActiveStaticPrecompiles)
	require.NotNil(t, evmGenesis.Params.ExtendedDenomOptions)
	require.Equal(t, lcfg.ChainEVMExtendedDenom, evmGenesis.Params.ExtendedDenomOptions.ExtendedDenom)

	feemarketBasic, ok := modules[feemarkettypes.ModuleName].(module.HasGenesisBasics)
	require.True(t, ok)
	var feeGenesis feemarkettypes.GenesisState
	require.NoError(t, cdc.UnmarshalJSON(feemarketBasic.DefaultGenesis(cdc), &feeGenesis))
	require.False(t, feeGenesis.Params.NoBaseFee)
	require.True(
		t,
		feeGenesis.Params.BaseFee.Equal(sdkmath.LegacyMustNewDecFromStr(lcfg.FeeMarketDefaultBaseFee)),
	)
}
