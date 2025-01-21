package keeper_test

import (
	"testing"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	"github.com/stretchr/testify/require"
)

func TestBlockClaimCount(t *testing.T) {
	testCases := []struct {
		name          string
		operations    []string
		expectedCount uint64
	}{
		{
			name:          "initial count is zero",
			operations:    []string{},
			expectedCount: 0,
		},
		{
			name:          "single increment",
			operations:    []string{"increment"},
			expectedCount: 1,
		},
		{
			name:          "multiple increments",
			operations:    []string{"increment", "increment", "increment"},
			expectedCount: 3,
		},
		{
			name:          "reset after increment",
			operations:    []string{"increment", "increment", "reset"},
			expectedCount: 0,
		},
		{
			name:          "increment after reset",
			operations:    []string{"increment", "increment", "reset", "increment"},
			expectedCount: 1,
		},
		{
			name:          "multiple resets",
			operations:    []string{"increment", "reset", "increment", "increment", "reset"},
			expectedCount: 0,
		},
		{
			name:          "reset with no prior increments",
			operations:    []string{"reset"},
			expectedCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			keeper, ctx := keepertest.ClaimKeeper(t)

			// Initial count should be zero
			count := keeper.GetBlockClaimCount(ctx)
			require.Equal(t, uint64(0), count, "initial count should be zero")

			// Perform operations
			for _, op := range tc.operations {
				switch op {
				case "increment":
					keeper.IncrementBlockClaimCount(ctx)
				case "reset":
					keeper.ResetBlockClaimCount(ctx)
				}
			}

			// Check final count
			count = keeper.GetBlockClaimCount(ctx)
			require.Equal(t, tc.expectedCount, count, "unexpected final count")
		})
	}
}
