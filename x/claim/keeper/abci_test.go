package keeper_test

import (
	"testing"
	"time"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	"github.com/LumeraProtocol/lumera/x/claim/types"
	_ "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestBeginBlocker(t *testing.T) {

}

func TestEndBlocker(t *testing.T) {
	keeper, ctx := keepertest.ClaimKeeper(t)

	testCases := []struct {
		name          string
		setupParams   types.Params
		blockTime     time.Time
		expectDisable bool
	}{
		{
			name: "claims still active",
			setupParams: types.Params{
				EnableClaims: true,
				ClaimEndTime: time.Now().Add(time.Hour).Unix(),
			},
			blockTime:     time.Now(),
			expectDisable: false,
		},
		{
			name: "claims should end",
			setupParams: types.Params{
				EnableClaims: true,
				ClaimEndTime: time.Now().Unix(),
			},
			blockTime:     time.Now().Add(time.Second),
			expectDisable: true,
		},
		{
			name: "claims already disabled",
			setupParams: types.Params{
				EnableClaims: false,
				ClaimEndTime: time.Now().Unix(),
			},
			blockTime:     time.Now(),
			expectDisable: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup test case
			ctx = ctx.WithBlockTime(tc.blockTime)
			err := keeper.SetParams(ctx, tc.setupParams)
			require.NoError(t, err)

			// Run EndBlocker
			err = keeper.EndBlocker(ctx)
			require.NoError(t, err)

			// Verify final state
			params := keeper.GetParams(ctx)
			if tc.expectDisable {
				require.False(t, params.EnableClaims, "claims should be disabled")
			} else {
				require.Equal(t, tc.setupParams.EnableClaims, params.EnableClaims,
					"claims enable state should not change")
			}
		})
	}
}
