//go:build test
// +build test

package ibc_test

import (
	"bytes"
	"testing"

	sdkmath "cosmossdk.io/math"
	lcfg "github.com/LumeraProtocol/lumera/config"
	"github.com/LumeraProtocol/lumera/tests/ibctesting"
	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

// testIBCERC20MiddlewareRegistersTokenPairOnRecv verifies that receiving a
// valid ICS20 transfer auto-registers an ERC20 token pair and precompile map.
func testIBCERC20MiddlewareRegistersTokenPairOnRecv(t *testing.T) {
	_, chainA, chainB, path := setupERC20MiddlewarePath(t)

	amount := sdkmath.NewInt(1234)
	msg := transfertypes.NewMsgTransfer(
		path.EndpointB.ChannelConfig.PortID,
		path.EndpointB.ChannelID,
		sdk.NewCoin(lcfg.ChainDenom, amount),
		chainB.SenderAccount.GetAddress().String(),
		chainA.SenderAccount.GetAddress().String(),
		chainB.GetTimeoutHeight(),
		0,
		"",
	)

	_, err := chainB.SendMsgs(msg)
	require.NoError(t, err)
	require.Len(t, *chainB.PendingSendPackets, 1)

	require.NoError(t, path.RelayAndAckPendingPackets())

	ibcDenom := transferDenom(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID, lcfg.ChainDenom)
	received := chainA.Balance(chainA.SenderAccount.GetAddress(), ibcDenom)
	require.True(t, received.Amount.Equal(amount), "expected %s, got %s", amount.String(), received.Amount.String())

	appA := chainA.GetLumeraApp()
	ctxA := chainA.GetContext()

	tokenPairID := appA.Erc20Keeper.GetTokenPairID(ctxA, ibcDenom)
	tokenPair, found := appA.Erc20Keeper.GetTokenPair(ctxA, tokenPairID)
	require.True(t, found)
	require.Equal(t, ibcDenom, tokenPair.Denom)
	require.True(t, appA.Erc20Keeper.IsDynamicPrecompileAvailable(ctxA, common.HexToAddress(tokenPair.Erc20Address)))
}

// testIBCERC20MiddlewareNoRegistrationWhenDisabled verifies that ERC20
// auto-registration is skipped when the module feature flag is disabled.
func testIBCERC20MiddlewareNoRegistrationWhenDisabled(t *testing.T) {
	coord, chainA, chainB, path := setupERC20MiddlewarePath(t)

	appA := chainA.GetLumeraApp()
	ctxA := chainA.GetContext()
	params := appA.Erc20Keeper.GetParams(ctxA)
	params.EnableErc20 = false
	require.NoError(t, appA.Erc20Keeper.SetParams(ctxA, params))
	coord.CommitBlock(chainA)

	amount := sdkmath.NewInt(999)
	msg := transfertypes.NewMsgTransfer(
		path.EndpointB.ChannelConfig.PortID,
		path.EndpointB.ChannelID,
		sdk.NewCoin(lcfg.ChainDenom, amount),
		chainB.SenderAccount.GetAddress().String(),
		chainA.SenderAccount.GetAddress().String(),
		chainB.GetTimeoutHeight(),
		0,
		"",
	)

	_, err := chainB.SendMsgs(msg)
	require.NoError(t, err)
	require.Len(t, *chainB.PendingSendPackets, 1)

	require.NoError(t, path.RelayAndAckPendingPackets())

	ibcDenom := transferDenom(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID, lcfg.ChainDenom)
	received := chainA.Balance(chainA.SenderAccount.GetAddress(), ibcDenom)
	require.True(t, received.Amount.Equal(amount), "expected %s, got %s", amount.String(), received.Amount.String())

	tokenPairID := appA.Erc20Keeper.GetTokenPairID(chainA.GetContext(), ibcDenom)
	_, found := appA.Erc20Keeper.GetTokenPair(chainA.GetContext(), tokenPairID)
	require.False(t, found)
}

// testIBCERC20MiddlewareNoRegistrationForInvalidReceiver verifies defensive
// behavior when packet receiver is malformed.
func testIBCERC20MiddlewareNoRegistrationForInvalidReceiver(t *testing.T) {
	_, chainA, chainB, path := setupERC20MiddlewarePath(t)

	amount := sdkmath.NewInt(1111)
	msg := transfertypes.NewMsgTransfer(
		path.EndpointB.ChannelConfig.PortID,
		path.EndpointB.ChannelID,
		sdk.NewCoin(lcfg.ChainDenom, amount),
		chainB.SenderAccount.GetAddress().String(),
		"not_a_valid_recipient",
		chainB.GetTimeoutHeight(),
		0,
		"",
	)

	_, err := chainB.SendMsgs(msg)
	require.NoError(t, err)
	require.Len(t, *chainB.PendingSendPackets, 1)

	require.NoError(t, path.RelayAndAckPendingPackets())

	ibcDenom := transferDenom(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID, lcfg.ChainDenom)
	appA := chainA.GetLumeraApp()
	ctxA := chainA.GetContext()

	tokenPairID := appA.Erc20Keeper.GetTokenPairID(ctxA, ibcDenom)
	_, found := appA.Erc20Keeper.GetTokenPair(ctxA, tokenPairID)
	require.False(t, found, "token pair should not be registered for invalid receiver packet")
}

// testIBCERC20MiddlewareDenomCollisionKeepsExistingMap verifies that an existing
// denom-map collision entry is preserved and not overwritten by middleware.
func testIBCERC20MiddlewareDenomCollisionKeepsExistingMap(t *testing.T) {
	coord, chainA, chainB, path := setupERC20MiddlewarePath(t)

	ibcDenom := transferDenom(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID, lcfg.ChainDenom)
	appA := chainA.GetLumeraApp()
	ctxA := chainA.GetContext()
	dummyID := []byte("existing-collision-id")
	appA.Erc20Keeper.SetDenomMap(ctxA, ibcDenom, dummyID)
	coord.CommitBlock(chainA)

	amount := sdkmath.NewInt(2222)
	msg := transfertypes.NewMsgTransfer(
		path.EndpointB.ChannelConfig.PortID,
		path.EndpointB.ChannelID,
		sdk.NewCoin(lcfg.ChainDenom, amount),
		chainB.SenderAccount.GetAddress().String(),
		chainA.SenderAccount.GetAddress().String(),
		chainB.GetTimeoutHeight(),
		0,
		"",
	)

	_, err := chainB.SendMsgs(msg)
	require.NoError(t, err)
	require.Len(t, *chainB.PendingSendPackets, 1)

	require.NoError(t, path.RelayAndAckPendingPackets())

	mappedID := appA.Erc20Keeper.GetDenomMap(chainA.GetContext(), ibcDenom)
	require.True(t, bytes.Equal(dummyID, mappedID), "collision entry should remain untouched")

	_, found := appA.Erc20Keeper.GetTokenPair(chainA.GetContext(), dummyID)
	require.False(t, found, "collision map should not create token pair entry")
}

// setupERC20MiddlewarePath boots a two-chain IBC path with base fee disabled
// for deterministic packet fee behavior in tests, and ERC20 registration policy
// set to "all" so auto-registration works for any IBC denom (the default policy
// is "allowlist" which only allows uatom/uosmo/uusdc base denoms).
func setupERC20MiddlewarePath(t *testing.T) (*ibctesting.Coordinator, *ibctesting.TestChain, *ibctesting.TestChain, *ibctesting.Path) {
	t.Helper()
	coord := ibctesting.NewCoordinator(t, 2)
	chainA := coord.GetChain(ibctesting.GetChainID(1))
	chainB := coord.GetChain(ibctesting.GetChainID(2))

	disableBaseFeeForIBCTestChain(t, chainA)
	disableBaseFeeForIBCTestChain(t, chainB)
	setERC20PolicyAllForIBCTestChain(t, chainA)
	setERC20PolicyAllForIBCTestChain(t, chainB)
	coord.CommitBlock(chainA, chainB)

	path := ibctesting.NewTransferPath(chainA, chainB)
	path.Setup()
	return coord, chainA, chainB, path
}

// setERC20PolicyAllForIBCTestChain sets the ERC20 registration policy to "all"
// so that any IBC denom triggers auto-registration. Without this, the default
// "allowlist" policy silently skips registration for denoms not in the allowlist.
func setERC20PolicyAllForIBCTestChain(t *testing.T, chain *ibctesting.TestChain) {
	t.Helper()
	app := chain.GetLumeraApp()
	ctx := chain.GetContext()
	app.SetERC20RegistrationMode(ctx, "all")
}

// disableBaseFeeForIBCTestChain forces zero-fee-market constraints so ICS20
// transfer tests are not impacted by dynamic base-fee checks.
func disableBaseFeeForIBCTestChain(t *testing.T, chain *ibctesting.TestChain) {
	t.Helper()

	app := chain.GetLumeraApp()
	ctx := chain.GetContext()

	params := app.FeeMarketKeeper.GetParams(ctx)
	params.NoBaseFee = true
	params.BaseFee = sdkmath.LegacyZeroDec()
	params.MinGasPrice = sdkmath.LegacyZeroDec()
	require.NoError(t, app.FeeMarketKeeper.SetParams(ctx, params))
}

// transferDenom returns canonical ibc/<hash> denom for port/channel/base denom.
func transferDenom(portID, channelID, baseDenom string) string {
	trace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(portID, channelID, baseDenom))
	return trace.IBCDenom()
}
