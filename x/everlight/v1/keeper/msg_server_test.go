package keeper

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/everlight/v1/types"
)

func TestMsgUpdateParams(t *testing.T) {
	k, ctx, _, _ := setupTestKeeper(t)
	ms := NewMsgServerImpl(k)

	t.Run("rejects invalid authority", func(t *testing.T) {
		_, err := ms.UpdateParams(ctx, &types.MsgUpdateParams{
			Authority: "invalid",
			Params:    types.DefaultParams(),
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid authority")
	})

	t.Run("rejects invalid params", func(t *testing.T) {
		params := types.DefaultParams()
		params.PaymentPeriodBlocks = 0

		_, err := ms.UpdateParams(ctx, &types.MsgUpdateParams{
			Authority: k.GetAuthority(),
			Params:    params,
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "payment_period_blocks must be > 0")
	})

	t.Run("updates all params through msg server", func(t *testing.T) {
		updated := types.Params{
			PaymentPeriodBlocks:         144,
			ValidatorRewardShareBps:     250,
			RegistrationFeeShareBps:     375,
			MinCascadeBytesForPayment:   2048,
			NewSnRampUpPeriods:          9,
			MeasurementSmoothingPeriods: 6,
			UsageGrowthCapBpsPerPeriod:  1250,
		}

		_, err := ms.UpdateParams(ctx, &types.MsgUpdateParams{
			Authority: k.GetAuthority(),
			Params:    updated,
		})

		require.NoError(t, err)
		require.Equal(t, updated, k.GetParams(ctx))
	})
}
