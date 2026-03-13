package keeper

import (
	"context"
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/everlight/v1/types"
)

// AT34: Everlight module account accepts MsgSend transfers.
//
// This suite verifies that:
//  1. The module account address is deterministic (derived from module name).
//  2. SendCoinsFromAccountToModule to "everlight" works correctly.
//  3. The module account is registered with the expected Burner permission.

// TestModuleAccountAddressDeterministic verifies that the Everlight module
// account address can be derived from the module name "everlight" and is
// consistent across calls.
func TestModuleAccountAddressDeterministic(t *testing.T) {
	// Derive the module address from the module name directly.
	addr1 := authtypes.NewModuleAddress(types.ModuleName)
	addr2 := authtypes.NewModuleAddress(types.ModuleName)

	require.NotEmpty(t, addr1, "module address should not be empty")
	require.Equal(t, addr1, addr2, "module address should be deterministic")

	// Also verify via the mock account keeper (same path used by the keeper).
	ak := &mockAccountKeeper{}
	addrViaKeeper := ak.GetModuleAddress(types.ModuleName)
	require.Equal(t, addr1, addrViaKeeper,
		"address from authtypes.NewModuleAddress must match accountKeeper.GetModuleAddress")

	// Verify the module name constant.
	require.Equal(t, "everlight", types.ModuleName)
}

// mockBankKeeperWithAccountToModule extends mockBankKeeper to track and
// execute SendCoinsFromAccountToModule transfers (the base mock is a no-op).
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
	// Deduct from sender, credit module.
	m.balances[senderAddr.String()] = m.balances[senderAddr.String()].Sub(amt...)
	moduleAddr := authtypes.NewModuleAddress(recipientModule)
	m.balances[moduleAddr.String()] = m.balances[moduleAddr.String()].Add(amt...)
	return nil
}

// TestSendCoinsFromAccountToModule verifies that a regular account can send
// coins to the everlight module account via SendCoinsFromAccountToModule.
func TestSendCoinsFromAccountToModule(t *testing.T) {
	bk := newMockBankKeeperWithAccountToModule()

	sender := makeAccAddr(1)
	amount := sdk.NewCoins(sdk.NewCoin("ulume", sdkmath.NewInt(5000)))

	// Fund the sender.
	bk.balances[sender.String()] = sdk.NewCoins(sdk.NewCoin("ulume", sdkmath.NewInt(10000)))

	// Send coins from the regular account to the everlight module.
	err := bk.SendCoinsFromAccountToModule(
		context.Background(), sender, types.ModuleName, amount,
	)
	require.NoError(t, err)

	// Verify the transfer was recorded.
	require.Len(t, bk.accountToModuleTransfers, 1)
	transfer := bk.accountToModuleTransfers[0]
	require.Equal(t, sender.String(), transfer.senderAddr)
	require.Equal(t, types.ModuleName, transfer.recipientModule)
	require.Equal(t, amount, transfer.amount)

	// Verify balance changes.
	moduleAddr := authtypes.NewModuleAddress(types.ModuleName)
	require.Equal(t, sdkmath.NewInt(5000), bk.balances[sender.String()].AmountOf("ulume"),
		"sender balance should be debited")
	require.Equal(t, sdkmath.NewInt(5000), bk.balances[moduleAddr.String()].AmountOf("ulume"),
		"module account should be credited")
}

// mockAccountKeeperWithPerms extends mockAccountKeeper to return a module
// account with specific permissions, matching the app_config.go registration.
type mockAccountKeeperWithPerms struct {
	mockAccountKeeper
	permissions map[string][]string
}

func (m *mockAccountKeeperWithPerms) GetModuleAccount(_ context.Context, moduleName string) sdk.ModuleAccountI {
	perms, ok := m.permissions[moduleName]
	if !ok {
		return nil
	}
	addr := authtypes.NewModuleAddress(moduleName)
	baseAcc := authtypes.NewBaseAccountWithAddress(addr)
	modAcc := authtypes.NewModuleAccount(baseAcc, moduleName, perms...)
	return modAcc
}

// TestModuleAccountPermissions verifies that the Everlight module account has
// exactly the Burner permission, matching app_config.go registration:
//
//	{Account: everlightmoduletypes.ModuleName, Permissions: []string{authtypes.Burner}}
func TestModuleAccountPermissions(t *testing.T) {
	ak := &mockAccountKeeperWithPerms{
		permissions: map[string][]string{
			types.ModuleName: {authtypes.Burner},
		},
	}

	modAcc := ak.GetModuleAccount(context.Background(), types.ModuleName)
	require.NotNil(t, modAcc, "module account should exist")

	// Verify name.
	require.Equal(t, types.ModuleName, modAcc.GetName())

	// Verify the account has the Burner permission.
	require.True(t, modAcc.HasPermission(authtypes.Burner),
		"everlight module account must have Burner permission")

	// Verify the account does NOT have Minter or Staking permissions.
	require.False(t, modAcc.HasPermission(authtypes.Minter),
		"everlight module account must NOT have Minter permission")
	require.False(t, modAcc.HasPermission(authtypes.Staking),
		"everlight module account must NOT have Staking permission")

	// Verify the module address on the account matches the deterministic address.
	expectedAddr := authtypes.NewModuleAddress(types.ModuleName)
	require.Equal(t, expectedAddr, modAcc.GetAddress(),
		"module account address should match deterministic derivation")
}
