package app

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	cosmosevmutils "github.com/cosmos/evm/utils"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	corevm "github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"

	lcfg "github.com/LumeraProtocol/lumera/config"
)

// TestBlockedAddressesMatrix verifies BlockedAddresses uses concrete account
// addresses (not module names) and includes EVM precompile recipients.
func TestBlockedAddressesMatrix(t *testing.T) {
	t.Parallel()

	blocked := BlockedAddresses()
	require.NotEmpty(t, blocked)

	for _, moduleName := range blockAccAddrs {
		moduleAddr := authtypes.NewModuleAddress(moduleName).String()
		require.True(t, blocked[moduleAddr], "module account address %s should be blocked", moduleAddr)
		require.False(t, blocked[moduleName], "module name %s must not be used as blocked-address key", moduleName)
	}

	require.NotEmpty(t, corevm.PrecompiledAddressesPrague)
	nativePrecompileAddr := cosmosevmutils.Bech32StringFromHexAddress(corevm.PrecompiledAddressesPrague[0].Hex())
	require.True(t, blocked[nativePrecompileAddr], "native precompile address should be blocked")

	if len(evmtypes.AvailableStaticPrecompiles) > 0 {
		staticPrecompileAddr := cosmosevmutils.Bech32StringFromHexAddress(evmtypes.AvailableStaticPrecompiles[0])
		require.True(t, blocked[staticPrecompileAddr], "Cosmos EVM static precompile should be blocked")
	}
}

// TestPrecompileSendRestriction blocks runtime bank sends into precompile
// addresses while keeping regular account-to-account sends functional.
func TestPrecompileSendRestriction(t *testing.T) {
	app := Setup(t)
	ctx := app.NewContextLegacy(false, tmproto.Header{Height: app.LastBlockHeight() + 1})

	addrs := AddTestAddrsIncremental(app, ctx, 2, sdkmath.NewInt(1_000_000))
	require.Len(t, addrs, 2)

	// sending from one regular account to another should work
	amount := sdk.NewCoins(sdk.NewInt64Coin(lcfg.ChainDenom, 1))
	require.NoError(t, app.BankKeeper.SendCoins(ctx, addrs[0], addrs[1], amount))

	// sending from a regular account to a precompile address should be blocked
	precompileBech32 := cosmosevmutils.Bech32StringFromHexAddress(corevm.PrecompiledAddressesPrague[0].Hex())
	precompileAddr := sdk.MustAccAddressFromBech32(precompileBech32)

	err := app.BankKeeper.SendCoins(ctx, addrs[0], precompileAddr, amount)
	require.Error(t, err)
	require.Contains(t, err.Error(), "precompile address")
}
