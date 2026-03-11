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

// --- Fee routing mock helpers ---

// mockBankKeeperWithModuleTransfers extends the base mockBankKeeper to track
// SendCoinsFromModuleToModule transfers (the base mock discards them).
type mockBankKeeperWithModuleTransfers struct {
	mockBankKeeper
	moduleTransfers []moduleTransferRecord
}

type moduleTransferRecord struct {
	senderModule    string
	recipientModule string
	amount          sdk.Coins
}

func newMockBankKeeperWithModuleTransfers() *mockBankKeeperWithModuleTransfers {
	return &mockBankKeeperWithModuleTransfers{
		mockBankKeeper: mockBankKeeper{
			balances: make(map[string]sdk.Coins),
		},
	}
}

func (m *mockBankKeeperWithModuleTransfers) SendCoinsFromModuleToModule(_ context.Context, senderModule, recipientModule string, amt sdk.Coins) error {
	m.moduleTransfers = append(m.moduleTransfers, moduleTransferRecord{
		senderModule:    senderModule,
		recipientModule: recipientModule,
		amount:          amt,
	})
	// Deduct from sender module balance, credit to recipient module balance.
	senderAddr := authtypes.NewModuleAddress(senderModule)
	recipientAddr := authtypes.NewModuleAddress(recipientModule)
	m.balances[senderAddr.String()] = m.balances[senderAddr.String()].Sub(amt...)
	m.balances[recipientAddr.String()] = m.balances[recipientAddr.String()].Add(amt...)
	return nil
}

// --- Tests ---

// AT39: Registration fee share flows to Everlight pool on action finalization.
//
// This test verifies that the Everlight keeper correctly exposes
// GetRegistrationFeeShareBps, which the action module's DistributeFees
// uses to calculate and route the registration fee share.
func TestGetRegistrationFeeShareBps(t *testing.T) {
	k, ctx, _, _ := setupTestKeeper(t)

	// Default params have RegistrationFeeShareBps = 200 (2%).
	bps := k.GetRegistrationFeeShareBps(ctx)
	require.Equal(t, uint64(200), bps, "default registration_fee_share_bps should be 200")

	// Update params and verify.
	params := k.GetParams(ctx)
	params.RegistrationFeeShareBps = 500 // 5%
	require.NoError(t, k.SetParams(ctx, params))

	bps = k.GetRegistrationFeeShareBps(ctx)
	require.Equal(t, uint64(500), bps, "updated registration_fee_share_bps should be 500")

	// Set to zero and verify.
	params.RegistrationFeeShareBps = 0
	require.NoError(t, k.SetParams(ctx, params))

	bps = k.GetRegistrationFeeShareBps(ctx)
	require.Equal(t, uint64(0), bps, "zero registration_fee_share_bps should be 0")
}

// AT39: Verify the math for registration fee share calculation.
// This mirrors the calculation done in the action module's DistributeFees.
func TestRegistrationFeeShareCalculation(t *testing.T) {
	tests := []struct {
		name           string
		feeAmount      int64
		shareBps       uint64
		expectedShare  int64
		expectedRemain int64
	}{
		{
			name:           "2% of 10000",
			feeAmount:      10000,
			shareBps:       200,
			expectedShare:  200,
			expectedRemain: 9800,
		},
		{
			name:           "5% of 10000",
			feeAmount:      10000,
			shareBps:       500,
			expectedShare:  500,
			expectedRemain: 9500,
		},
		{
			name:           "1% of 100",
			feeAmount:      100,
			shareBps:       100,
			expectedShare:  1,
			expectedRemain: 99,
		},
		{
			name:           "2% of 99 (truncation)",
			feeAmount:      99,
			shareBps:       200,
			expectedShare:  1, // 99 * 200 / 10000 = 1.98 -> truncated to 1
			expectedRemain: 98,
		},
		{
			name:           "0 bps means no share",
			feeAmount:      10000,
			shareBps:       0,
			expectedShare:  0,
			expectedRemain: 10000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			feeAmount := sdkmath.NewInt(tc.feeAmount)
			if tc.shareBps == 0 {
				require.Equal(t, tc.expectedRemain, feeAmount.Int64())
				return
			}
			everlightAmount := feeAmount.MulRaw(int64(tc.shareBps)).QuoRaw(10000)
			require.Equal(t, tc.expectedShare, everlightAmount.Int64())
			remaining := feeAmount.Sub(everlightAmount)
			require.Equal(t, tc.expectedRemain, remaining.Int64())
		})
	}
}

// AT40: Block reward share flows to Everlight pool.
func TestBeginBlockerBlockRewardShare(t *testing.T) {
	// Set up keeper with a bank keeper that tracks module-to-module transfers.
	k, ctx, _, _ := setupTestKeeper(t)

	// Create a bank keeper with module transfer tracking.
	bankKeeperWithTransfers := newMockBankKeeperWithModuleTransfers()

	// Fund the fee collector with 10000 ulume.
	feeCollectorAddr := authtypes.NewModuleAddress(authtypes.FeeCollectorName)
	bankKeeperWithTransfers.balances[feeCollectorAddr.String()] = sdk.NewCoins(
		sdk.NewCoin("ulume", sdkmath.NewInt(10000)),
	)

	// Replace the bank keeper in the keeper (we need to use reflection or re-create).
	// Since the Keeper struct fields are unexported, we'll create a new keeper with
	// the enhanced mock.
	k = NewKeeper(
		k.cdc,
		k.storeService,
		k.logger,
		k.authority,
		bankKeeperWithTransfers,
		&mockAccountKeeper{},
		newMockSupernodeKeeper(),
	)

	// Set params: 1% validator reward share.
	params := types.DefaultParams()
	params.ValidatorRewardShareBps = 100 // 1%
	require.NoError(t, k.SetParams(ctx, params))

	// Run BeginBlocker.
	err := k.BeginBlocker(ctx)
	require.NoError(t, err)

	// Verify: 1% of 10000 = 100 ulume should be transferred from fee_collector to everlight.
	require.Len(t, bankKeeperWithTransfers.moduleTransfers, 1)
	transfer := bankKeeperWithTransfers.moduleTransfers[0]
	require.Equal(t, authtypes.FeeCollectorName, transfer.senderModule)
	require.Equal(t, types.ModuleName, transfer.recipientModule)
	require.Equal(t, sdkmath.NewInt(100), transfer.amount.AmountOf("ulume"))
}

// AT40: BeginBlocker with zero validator_reward_share_bps skips transfer.
func TestBeginBlockerZeroShareBps(t *testing.T) {
	k, ctx, _, _ := setupTestKeeper(t)

	bankKeeperWithTransfers := newMockBankKeeperWithModuleTransfers()
	feeCollectorAddr := authtypes.NewModuleAddress(authtypes.FeeCollectorName)
	bankKeeperWithTransfers.balances[feeCollectorAddr.String()] = sdk.NewCoins(
		sdk.NewCoin("ulume", sdkmath.NewInt(10000)),
	)

	k = NewKeeper(
		k.cdc,
		k.storeService,
		k.logger,
		k.authority,
		bankKeeperWithTransfers,
		&mockAccountKeeper{},
		newMockSupernodeKeeper(),
	)

	// Set params with zero share.
	params := types.DefaultParams()
	params.ValidatorRewardShareBps = 0
	require.NoError(t, k.SetParams(ctx, params))

	err := k.BeginBlocker(ctx)
	require.NoError(t, err)

	// No transfers should occur.
	require.Empty(t, bankKeeperWithTransfers.moduleTransfers)
}

// AT40: BeginBlocker with empty fee collector skips transfer.
func TestBeginBlockerEmptyFeeCollector(t *testing.T) {
	k, ctx, _, _ := setupTestKeeper(t)

	bankKeeperWithTransfers := newMockBankKeeperWithModuleTransfers()
	// Fee collector has no balance.

	k = NewKeeper(
		k.cdc,
		k.storeService,
		k.logger,
		k.authority,
		bankKeeperWithTransfers,
		&mockAccountKeeper{},
		newMockSupernodeKeeper(),
	)

	params := types.DefaultParams()
	params.ValidatorRewardShareBps = 100 // 1%
	require.NoError(t, k.SetParams(ctx, params))

	err := k.BeginBlocker(ctx)
	require.NoError(t, err)

	// No transfers should occur.
	require.Empty(t, bankKeeperWithTransfers.moduleTransfers)
}

// AT40: BeginBlocker with small fee collector balance (truncation to zero).
func TestBeginBlockerSmallBalance(t *testing.T) {
	k, ctx, _, _ := setupTestKeeper(t)

	bankKeeperWithTransfers := newMockBankKeeperWithModuleTransfers()
	feeCollectorAddr := authtypes.NewModuleAddress(authtypes.FeeCollectorName)
	// Balance of 50 ulume with 1% share = 0.5 ulume, truncated to 0.
	bankKeeperWithTransfers.balances[feeCollectorAddr.String()] = sdk.NewCoins(
		sdk.NewCoin("ulume", sdkmath.NewInt(50)),
	)

	k = NewKeeper(
		k.cdc,
		k.storeService,
		k.logger,
		k.authority,
		bankKeeperWithTransfers,
		&mockAccountKeeper{},
		newMockSupernodeKeeper(),
	)

	params := types.DefaultParams()
	params.ValidatorRewardShareBps = 100 // 1%
	require.NoError(t, k.SetParams(ctx, params))

	err := k.BeginBlocker(ctx)
	require.NoError(t, err)

	// 50 * 100 / 10000 = 0, so no transfer.
	require.Empty(t, bankKeeperWithTransfers.moduleTransfers)
}

// AT40: BeginBlocker with large fee collector balance.
func TestBeginBlockerLargeBalance(t *testing.T) {
	k, ctx, _, _ := setupTestKeeper(t)

	bankKeeperWithTransfers := newMockBankKeeperWithModuleTransfers()
	feeCollectorAddr := authtypes.NewModuleAddress(authtypes.FeeCollectorName)
	// Balance of 1_000_000 ulume with 5% share = 50_000 ulume.
	bankKeeperWithTransfers.balances[feeCollectorAddr.String()] = sdk.NewCoins(
		sdk.NewCoin("ulume", sdkmath.NewInt(1_000_000)),
	)

	k = NewKeeper(
		k.cdc,
		k.storeService,
		k.logger,
		k.authority,
		bankKeeperWithTransfers,
		&mockAccountKeeper{},
		newMockSupernodeKeeper(),
	)

	params := types.DefaultParams()
	params.ValidatorRewardShareBps = 500 // 5%
	require.NoError(t, k.SetParams(ctx, params))

	err := k.BeginBlocker(ctx)
	require.NoError(t, err)

	require.Len(t, bankKeeperWithTransfers.moduleTransfers, 1)
	transfer := bankKeeperWithTransfers.moduleTransfers[0]
	require.Equal(t, authtypes.FeeCollectorName, transfer.senderModule)
	require.Equal(t, types.ModuleName, transfer.recipientModule)
	require.Equal(t, sdkmath.NewInt(50_000), transfer.amount.AmountOf("ulume"))
}
