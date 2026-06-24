package app

import (
	"testing"

	appevm "github.com/LumeraProtocol/lumera/app/evm"
	"github.com/ethereum/go-ethereum/common"
	corevm "github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

// TestEVMStaticPrecompilesConfigured ensures static precompile instances are
// registered in the EVM keeper and active in module params.
func TestEVMStaticPrecompilesConfigured(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)

	params := app.EVMKeeper.GetParams(ctx)
	require.ElementsMatch(t, appevm.LumeraActiveStaticPrecompiles, params.ActiveStaticPrecompiles)

	for _, precompileHex := range appevm.LumeraActiveStaticPrecompiles {
		_, found, err := app.EVMKeeper.GetStaticPrecompileInstance(&params, common.HexToAddress(precompileHex))
		require.NoError(t, err)
		require.True(t, found, "expected static precompile %s to be registered", precompileHex)
	}

	// Native geth precompiles are also part of the static registry.
	require.NotEmpty(t, corevm.PrecompiledAddressesPrague)
	_, found, err := app.EVMKeeper.GetStaticPrecompileInstance(&params, corevm.PrecompiledAddressesPrague[0])
	require.NoError(t, err)
	require.True(t, found, "expected native precompile %s to be registered", corevm.PrecompiledAddressesPrague[0].Hex())
}
