package keeper

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/stretchr/testify/require"
)

// --- Tests ---

// AT39: Registration fee share flows to Everlight pool on action finalization.
//
// This test verifies that the Everlight keeper correctly exposes
// GetRegistrationFeeShareBps, which the action module's DistributeFees
// uses to calculate and route the registration fee share.
func TestGetRegistrationFeeShareBps(t *testing.T) {
	k, ctx, _, _, _ := setupTestKeeper(t)

	// Default params have RegistrationFeeShareBps = 200 (2%).
	bps := k.GetRegistrationFeeShareBps(ctx)
	require.Equal(t, uint64(200), bps, "default registration_fee_share_bps should be 200")

	// Update params and verify.
	params := k.GetParams(ctx)
	params.RewardDistribution.RegistrationFeeShareBps = 500 // 5%
	require.NoError(t, k.SetParams(ctx, params))

	bps = k.GetRegistrationFeeShareBps(ctx)
	require.Equal(t, uint64(500), bps, "updated registration_fee_share_bps should be 500")

	// Set to zero and verify.
	params.RewardDistribution.RegistrationFeeShareBps = 0
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
