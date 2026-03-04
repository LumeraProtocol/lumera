//go:build test
// +build test

package ibc_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	lcfg "github.com/LumeraProtocol/lumera/config"
	"github.com/LumeraProtocol/lumera/tests/ibctesting"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/contracts"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

// testIBCERC20RoundTripTransfer sends tokens from chainB→chainA via IBC,
// verifies ERC20 registration and ERC20 balance via keeper, then sends
// back chainA→chainB and verifies balances are restored.
func testIBCERC20RoundTripTransfer(t *testing.T) {
	coord, chainA, chainB, path := setupERC20MiddlewarePath(t)

	senderB := chainB.SenderAccount.GetAddress()
	receiverA := chainA.SenderAccount.GetAddress()

	// Record initial balances.
	initialBalanceB := chainB.Balance(senderB, lcfg.ChainDenom)

	// --- Forward transfer: chainB → chainA ---
	amount := sdkmath.NewInt(1000)
	msg := transfertypes.NewMsgTransfer(
		path.EndpointB.ChannelConfig.PortID,
		path.EndpointB.ChannelID,
		sdk.NewCoin(lcfg.ChainDenom, amount),
		senderB.String(),
		receiverA.String(),
		chainB.GetTimeoutHeight(),
		0,
		"",
	)

	_, err := chainB.SendMsgs(msg)
	require.NoError(t, err)
	require.Len(t, *chainB.PendingSendPackets, 1)
	require.NoError(t, path.RelayAndAckPendingPackets())

	// Verify IBC denom received on chainA.
	ibcDenom := transferDenom(
		path.EndpointA.ChannelConfig.PortID,
		path.EndpointA.ChannelID,
		lcfg.ChainDenom,
	)
	received := chainA.Balance(receiverA, ibcDenom)
	require.True(t, received.Amount.Equal(amount),
		"chainA should receive %s, got %s", amount, received.Amount)

	// Verify ERC20 token pair was auto-registered.
	appA := chainA.GetLumeraApp()
	ctxA := chainA.GetContext()

	tokenPairID := appA.Erc20Keeper.GetTokenPairID(ctxA, ibcDenom)
	tokenPair, found := appA.Erc20Keeper.GetTokenPair(ctxA, tokenPairID)
	require.True(t, found, "ERC20 token pair should be registered for %s", ibcDenom)
	require.Equal(t, ibcDenom, tokenPair.Denom)
	require.True(t, tokenPair.Enabled, "token pair should be enabled")

	// Verify ERC20 balance via keeper BalanceOf.
	erc20ABI := contracts.ERC20MinterBurnerDecimalsContract.ABI
	contractAddr := tokenPair.GetERC20Contract()
	evmAddr := common.BytesToAddress(receiverA.Bytes())

	erc20Balance := appA.Erc20Keeper.BalanceOf(ctxA, erc20ABI, contractAddr, evmAddr)
	require.NotNil(t, erc20Balance, "ERC20 balanceOf should return non-nil")

	// Keeper BalanceOf returns the ERC20-visible token amount for this pair.
	// For this middleware path, it should match the transferred amount.
	expectedERC20 := amount.BigInt()
	require.Equal(t, 0, erc20Balance.Cmp(expectedERC20),
		"ERC20 balance should be %s, got %s", expectedERC20, erc20Balance)

	// --- Reverse transfer: chainA → chainB ---
	msgBack := transfertypes.NewMsgTransfer(
		path.EndpointA.ChannelConfig.PortID,
		path.EndpointA.ChannelID,
		sdk.NewCoin(ibcDenom, amount),
		receiverA.String(),
		senderB.String(),
		chainA.GetTimeoutHeight(),
		0,
		"",
	)

	_, err = chainA.SendMsgs(msgBack)
	require.NoError(t, err)
	require.Len(t, *chainA.PendingSendPackets, 1)
	require.NoError(t, path.RelayAndAckPendingPackets())

	// Commit to finalize state.
	coord.CommitBlock(chainA, chainB)

	// ChainA should have zero IBC denom balance.
	remainA := chainA.Balance(receiverA, ibcDenom)
	require.True(t, remainA.Amount.IsZero(),
		"chainA IBC denom balance should be zero, got %s", remainA.Amount)

	// ChainB should have original balance restored.
	finalBalanceB := chainB.Balance(senderB, lcfg.ChainDenom)
	require.True(t, finalBalanceB.Amount.Equal(initialBalanceB.Amount),
		"chainB balance should be restored: want %s, got %s",
		initialBalanceB.Amount, finalBalanceB.Amount)
}

// testIBCERC20SecondaryDenomRegistration verifies that a non-native denom
// (ufoo) also gets ERC20 auto-registration when received via IBC.
func testIBCERC20SecondaryDenomRegistration(t *testing.T) {
	_, chainA, chainB, path := setupERC20MiddlewarePath(t)

	senderB := chainB.SenderAccount.GetAddress()
	receiverA := chainA.SenderAccount.GetAddress()

	// Transfer ufoo from chainB to chainA.
	amount := sdkmath.NewInt(500)
	msg := transfertypes.NewMsgTransfer(
		path.EndpointB.ChannelConfig.PortID,
		path.EndpointB.ChannelID,
		sdk.NewCoin(ibctesting.SecondaryDenom, amount),
		senderB.String(),
		receiverA.String(),
		chainB.GetTimeoutHeight(),
		0,
		"",
	)

	_, err := chainB.SendMsgs(msg)
	require.NoError(t, err)
	require.Len(t, *chainB.PendingSendPackets, 1)
	require.NoError(t, path.RelayAndAckPendingPackets())

	// Verify IBC denom received on chainA.
	ibcDenom := transferDenom(
		path.EndpointA.ChannelConfig.PortID,
		path.EndpointA.ChannelID,
		ibctesting.SecondaryDenom,
	)
	received := chainA.Balance(receiverA, ibcDenom)
	require.True(t, received.Amount.Equal(amount),
		"chainA should receive %s ufoo, got %s", amount, received.Amount)

	// Verify ERC20 token pair was auto-registered for the secondary denom.
	appA := chainA.GetLumeraApp()
	ctxA := chainA.GetContext()

	tokenPairID := appA.Erc20Keeper.GetTokenPairID(ctxA, ibcDenom)
	tokenPair, found := appA.Erc20Keeper.GetTokenPair(ctxA, tokenPairID)
	require.True(t, found, "ERC20 token pair should be registered for secondary denom %s", ibcDenom)
	require.Equal(t, ibcDenom, tokenPair.Denom)
	require.True(t, tokenPair.Enabled, "token pair should be enabled")

	// Verify dynamic precompile is available.
	require.True(t,
		appA.Erc20Keeper.IsDynamicPrecompileAvailable(ctxA, common.HexToAddress(tokenPair.Erc20Address)),
		"dynamic precompile should be registered for secondary denom")
}

// testIBCERC20TransferBackBurnsVoucher verifies that sending IBC tokens
// back to the source chain properly reduces the balance on the destination
// and the ERC20 balance reflects zero.
func testIBCERC20TransferBackBurnsVoucher(t *testing.T) {
	coord, chainA, chainB, path := setupERC20MiddlewarePath(t)

	senderB := chainB.SenderAccount.GetAddress()
	receiverA := chainA.SenderAccount.GetAddress()

	// Forward transfer: chainB → chainA.
	amount := sdkmath.NewInt(2000)
	msg := transfertypes.NewMsgTransfer(
		path.EndpointB.ChannelConfig.PortID,
		path.EndpointB.ChannelID,
		sdk.NewCoin(lcfg.ChainDenom, amount),
		senderB.String(),
		receiverA.String(),
		chainB.GetTimeoutHeight(),
		0,
		"",
	)

	_, err := chainB.SendMsgs(msg)
	require.NoError(t, err)
	require.Len(t, *chainB.PendingSendPackets, 1)
	require.NoError(t, path.RelayAndAckPendingPackets())

	ibcDenom := transferDenom(
		path.EndpointA.ChannelConfig.PortID,
		path.EndpointA.ChannelID,
		lcfg.ChainDenom,
	)

	// Confirm token pair exists.
	appA := chainA.GetLumeraApp()
	ctxA := chainA.GetContext()

	tokenPairID := appA.Erc20Keeper.GetTokenPairID(ctxA, ibcDenom)
	tokenPair, found := appA.Erc20Keeper.GetTokenPair(ctxA, tokenPairID)
	require.True(t, found, "token pair should exist after forward transfer")

	// Record ERC20 balance after forward transfer.
	erc20ABI := contracts.ERC20MinterBurnerDecimalsContract.ABI
	contractAddr := tokenPair.GetERC20Contract()
	evmAddr := common.BytesToAddress(receiverA.Bytes())

	erc20BalanceBefore := appA.Erc20Keeper.BalanceOf(ctxA, erc20ABI, contractAddr, evmAddr)
	require.NotNil(t, erc20BalanceBefore, "ERC20 balance should be non-nil after forward transfer")
	require.True(t, erc20BalanceBefore.Sign() > 0,
		"ERC20 balance should be positive, got %s", erc20BalanceBefore)

	// Reverse transfer: send all IBC tokens back chainA → chainB.
	msgBack := transfertypes.NewMsgTransfer(
		path.EndpointA.ChannelConfig.PortID,
		path.EndpointA.ChannelID,
		sdk.NewCoin(ibcDenom, amount),
		receiverA.String(),
		senderB.String(),
		chainA.GetTimeoutHeight(),
		0,
		"",
	)

	_, err = chainA.SendMsgs(msgBack)
	require.NoError(t, err)
	require.Len(t, *chainA.PendingSendPackets, 1)
	require.NoError(t, path.RelayAndAckPendingPackets())

	coord.CommitBlock(chainA, chainB)

	// ChainA bank balance of IBC denom should be zero.
	remainA := chainA.Balance(receiverA, ibcDenom)
	require.True(t, remainA.Amount.IsZero(),
		"chainA IBC denom balance should be zero after sending back, got %s", remainA.Amount)

	// Token pair should still exist (registration is permanent).
	ctxA = chainA.GetContext()
	tokenPairID = appA.Erc20Keeper.GetTokenPairID(ctxA, ibcDenom)
	_, found = appA.Erc20Keeper.GetTokenPair(ctxA, tokenPairID)
	require.True(t, found, "token pair should still exist after burn-back")

	// ERC20 balance should now be zero.
	erc20BalanceAfter := appA.Erc20Keeper.BalanceOf(ctxA, erc20ABI, contractAddr, evmAddr)
	if erc20BalanceAfter != nil {
		require.True(t, erc20BalanceAfter.Sign() == 0,
			"ERC20 balance should be zero after sending back, got %s", erc20BalanceAfter)
	}
}
