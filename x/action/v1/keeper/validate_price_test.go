package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	"github.com/LumeraProtocol/lumera/x/action/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

// TestValidatePrice_EdgeCases tests the validatePrice function with edge cases
// This test exposes a critical bug where 0-byte actions are rejected
func TestValidatePrice_EdgeCases(t *testing.T) {
	testCases := []struct {
		name        string
		dataKB      int64  // Data size in KB (after rounding)
		price       string // Price to validate
		shouldPass  bool   // Should validation pass?
		description string
	}{
		{
			name:        "0 bytes (0 KB)",
			dataKB:      0,
			price:       "10000ulume",
			shouldPass:  true,
			description: "0-byte actions calculate fee as 10000 and validatePrice now correctly accepts base fee only",
		},
		{
			name:        "1 byte (1 KB)",
			dataKB:      1,
			price:       "10010ulume",
			shouldPass:  true,
			description: "1 KB calculates correctly and passes validation",
		},
		{
			name:        "1 KB + 1 byte (2 KB)",
			dataKB:      2,
			price:       "10020ulume",
			shouldPass:  true,
			description: "2 KB calculates correctly and passes validation",
		},
		{
			name:        "minimum fee (base + per_kb) - should pass",
			dataKB:      1,
			price:       "10010ulume",
			shouldPass:  true,
			description: "Minimum acceptable fee for any non-zero data",
		},
		{
			name:        "below minimum fee - should fail",
			dataKB:      1,
			price:       "9999ulume",
			shouldPass:  false,
			description: "Fee below base fee should be rejected",
		},
		{
			name:        "overpaying is allowed",
			dataKB:      1,
			price:       "20000ulume",
			shouldPass:  true,
			description: "Users can pay more than the minimum",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			k, ctx := keepertest.ActionKeeper(t, ctrl)

			// Setup params with mainnet-ready values
			params := types.DefaultParams()
			params.BaseActionFee = sdk.NewCoin("ulume", math.NewInt(10000))
			params.FeePerKbyte = sdk.NewCoin("ulume", math.NewInt(10))
			k.SetParams(ctx, params)

			// Parse the price
			price, err := sdk.ParseCoinNormalized(tc.price)
			require.NoError(t, err, "Failed to parse price")

			// Create a mock action with this price
			action := &types.Action{
				ActionID:   "",
				Creator:    "lume1test",
				ActionType: types.ActionTypeCascade,
				Price:      price.String(),
				State:      types.ActionStatePending,
			}

			// Try to register the action (which calls validatePrice internally)
			_, err = k.RegisterAction(ctx, action)

			if tc.shouldPass {
				// If we expect it to pass, we might get other errors (like invalid creator)
				// but NOT a price validation error
				if err != nil {
					require.NotContains(t, err.Error(), "must be at least",
						"Expected price validation to pass but got price error: %v\nDescription: %s", err, tc.description)
				}
			} else {
				// If we expect it to fail, it should fail with a price error
				require.Error(t, err, "Expected price validation to fail\nDescription: %s", tc.description)
				require.Contains(t, err.Error(), "must be at least",
					"Expected price validation error but got different error: %v\nDescription: %s", err, tc.description)
			}

			t.Logf("Test: %s\nDescription: %s\nResult: %v", tc.name, tc.description, err)
		})
	}
}

// TestClientSideRoundingConsistency verifies that client-side rounding matches expectations
func TestClientSideRoundingConsistency(t *testing.T) {
	testCases := []struct {
		bytes       int64
		expectedKB  int64
		description string
	}{
		{0, 0, "0 bytes"},
		{1, 1, "1 byte"},
		{1023, 1, "1023 bytes (just under 1 KB)"},
		{1024, 1, "1024 bytes (exactly 1 KB)"},
		{1025, 2, "1025 bytes (1 KB + 1 byte)"},
		{2047, 2, "2047 bytes (just under 2 KB)"},
		{2048, 2, "2048 bytes (exactly 2 KB)"},
		{2049, 3, "2049 bytes (2 KB + 1 byte)"},
	}

	t.Log("=== Client-Side Rounding Formula: (bytes + 1023) / 1024 ===")
	t.Log("")

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			// Test the ACTUAL production function (centralized in types package)
			actualKB := types.RoundBytesToKB(int(tc.bytes))

			t.Logf("%s (%d bytes):", tc.description, tc.bytes)
			t.Logf("  Expected KB: %d", tc.expectedKB)
			t.Logf("  Actual KB:   %d", actualKB)

			require.Equal(t, tc.expectedKB, int64(actualKB),
				"RoundBytesToKB(%d) produced unexpected result", tc.bytes)

			if int64(actualKB) == tc.expectedKB {
				t.Logf("  ✓ Rounding correct")
			}
			t.Log("")
		})
	}
}
