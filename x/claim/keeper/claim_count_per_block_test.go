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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			keeper, ctx := keepertest.ClaimKeeper(t)

			// Initial count should be zero.
			count, err := keeper.GetBlockClaimCount(ctx)
			require.NoError(t, err)
			require.Equal(t, uint64(0), count, "initial count should be zero")

			// Perform operations.
			for _, op := range tc.operations {
				if op == "increment" {
					err := keeper.IncrementBlockClaimCount(ctx)
					require.NoError(t, err)
				}
			}

			// Check final count.
			count, err = keeper.GetBlockClaimCount(ctx)
			require.NoError(t, err)
			require.Equal(t, tc.expectedCount, count, "unexpected final count")
		})
	}
}

// Optional: Test block transition behavior
func TestBlockClaimCountReset(t *testing.T) {
	keeper, ctx := keepertest.ClaimKeeper(t)

	// Increment in current block.
	err := keeper.IncrementBlockClaimCount(ctx)
	require.NoError(t, err)

	count, err := keeper.GetBlockClaimCount(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(1), count)

	// Simulate new block by creating a new context.
	newCtx := ctx.WithBlockHeight(ctx.BlockHeight() + 1)

	// Count should be reset in new block.
	count, err = keeper.GetBlockClaimCount(newCtx)
	require.NoError(t, err)
	require.Equal(t, uint64(0), count)
}
