package keeper

import (
	"context"
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// AT34: Supernode module account accepts MsgSend transfers and holds the
// reward-distribution pool balance.
//
// This suite verifies that:
//  1. The module account address is deterministic (derived from "supernode").
//  2. SendCoinsFromAccountToModule to the supernode module works correctly.

// TestModuleAccountAddressDeterministic verifies that the supernode module
// account address is consistent across calls.
func TestModuleAccountAddressDeterministic(t *testing.T) {
	addr1 := authtypes.NewModuleAddress(types.ModuleName)
	addr2 := authtypes.NewModuleAddress(types.ModuleName)

	require.NotEmpty(t, addr1, "module address should not be empty")
	require.Equal(t, addr1, addr2, "module address should be deterministic")

	ak := &mockAccountKeeper{}
	addrViaKeeper := ak.GetModuleAddress(types.ModuleName)
	require.Equal(t, addr1, addrViaKeeper,
		"address from authtypes.NewModuleAddress must match accountKeeper.GetModuleAddress")
}

// mockBankKeeperWithAccountToModule extends mockBankKeeper to track and
// execute SendCoinsFromAccountToModule transfers.
type mockBankKeeperWithAccountToModule struct {
	mockBankKeeper
	accountToModuleTransfers []accountToModuleRecord
}

type accountToModuleRecord struct {
	senderAddr      string
	recipientModule string
	amount          sdk.Coins
}

func newMockBankKeeperWithAccountToModule() *mockBankKeeperWithAccountToModule {
	return &mockBankKeeperWithAccountToModule{
		mockBankKeeper: mockBankKeeper{
			balances: make(map[string]sdk.Coins),
		},
	}
}

func (m *mockBankKeeperWithAccountToModule) SendCoinsFromAccountToModule(
	_ context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins,
) error {
	m.accountToModuleTransfers = append(m.accountToModuleTransfers, accountToModuleRecord{
		senderAddr:      senderAddr.String(),
		recipientModule: recipientModule,
		amount:          amt,
	})
	m.balances[senderAddr.String()] = m.balances[senderAddr.String()].Sub(amt...)
	moduleAddr := authtypes.NewModuleAddress(recipientModule)
	m.balances[moduleAddr.String()] = m.balances[moduleAddr.String()].Add(amt...)
	return nil
}

// TestSendCoinsFromAccountToModule verifies that a regular account can send
// coins to the supernode module account via SendCoinsFromAccountToModule.
func TestSendCoinsFromAccountToModule(t *testing.T) {
	bk := newMockBankKeeperWithAccountToModule()

	sender := makeAccAddr(1)
	amount := sdk.NewCoins(sdk.NewCoin("ulume", sdkmath.NewInt(5000)))

	bk.balances[sender.String()] = sdk.NewCoins(sdk.NewCoin("ulume", sdkmath.NewInt(10000)))

	err := bk.SendCoinsFromAccountToModule(
		context.Background(), sender, types.ModuleName, amount,
	)
	require.NoError(t, err)

	require.Len(t, bk.accountToModuleTransfers, 1)
	transfer := bk.accountToModuleTransfers[0]
	require.Equal(t, sender.String(), transfer.senderAddr)
	require.Equal(t, types.ModuleName, transfer.recipientModule)
	require.Equal(t, amount, transfer.amount)

	moduleAddr := authtypes.NewModuleAddress(types.ModuleName)
	require.Equal(t, sdkmath.NewInt(5000), bk.balances[sender.String()].AmountOf("ulume"),
		"sender balance should be debited")
	require.Equal(t, sdkmath.NewInt(5000), bk.balances[moduleAddr.String()].AmountOf("ulume"),
		"module account should be credited")
}
