package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	supernodemocks "github.com/LumeraProtocol/lumera/x/supernode/v1/mocks"
)

func TestKeeper_GetMetrics(t *testing.T) {
	valAddr := sdk.ValAddress([]byte("validator"))
	anotherValAddr := sdk.ValAddress([]byte("another-validator"))

	metricsState := types.SupernodeMetricsState{
		ValidatorAddress: valAddr.String(),
		Metrics: &types.SupernodeMetrics{
			VersionMajor: 1,
			VersionMinor: 2,
			VersionPatch: 3,
		},
		ReportCount: 2,
		Height:      100,
	}

	testCases := []struct {
		name        string
		req         *types.QueryGetMetricsRequest
		setupState  func(k keeper.Keeper, ctx sdk.Context)
		expectedErr error
		checkResult func(t *testing.T, resp *types.QueryGetMetricsResponse)
	}{
		{
			name:        "invalid request (nil)",
			req:         nil,
			expectedErr: status.Error(codes.InvalidArgument, "invalid request"),
		},
		{
			name: "invalid validator address",
			req: &types.QueryGetMetricsRequest{
				ValidatorAddress: "invalid",
			},
			expectedErr: status.Error(codes.InvalidArgument, "invalid validator address"),
		},
		{
			name: "metrics not found",
			req: &types.QueryGetMetricsRequest{
				ValidatorAddress: anotherValAddr.String(),
			},
			expectedErr: status.Error(codes.NotFound, "no metrics found"),
		},
		{
			name: "metrics found",
			req: &types.QueryGetMetricsRequest{
				ValidatorAddress: valAddr.String(),
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				require.NoError(t, k.SetMetricsState(ctx, metricsState))
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types.QueryGetMetricsResponse) {
				require.NotNil(t, resp)
				require.NotNil(t, resp.MetricsState)
				require.Equal(t, metricsState.ValidatorAddress, resp.MetricsState.ValidatorAddress)
				require.Equal(t, metricsState.ReportCount, resp.MetricsState.ReportCount)
				require.Equal(t, metricsState.Height, resp.MetricsState.Height)
				require.NotNil(t, resp.MetricsState.Metrics)
				require.Equal(t, metricsState.Metrics.VersionMajor, resp.MetricsState.Metrics.VersionMajor)
				require.Equal(t, metricsState.Metrics.VersionMinor, resp.MetricsState.Metrics.VersionMinor)
				require.Equal(t, metricsState.Metrics.VersionPatch, resp.MetricsState.Metrics.VersionPatch)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			stakingKeeper := supernodemocks.NewMockStakingKeeper(ctrl)
			slashingKeeper := supernodemocks.NewMockSlashingKeeper(ctrl)
			bankKeeper := supernodemocks.NewMockBankKeeper(ctrl)

			k, ctx := setupKeeperForTest(t, stakingKeeper, slashingKeeper, bankKeeper)
			q := keeper.NewQueryServerImpl(k)

			if tc.setupState != nil {
				tc.setupState(k, ctx)
			}

			resp, err := q.GetMetrics(ctx, tc.req)

			if tc.expectedErr != nil {
				require.Error(t, err)
				st, _ := status.FromError(err)
				exp, _ := status.FromError(tc.expectedErr)
				require.Equal(t, exp.Code(), st.Code())
			} else {
				require.NoError(t, err)
				if tc.checkResult != nil {
					tc.checkResult(t, resp)
				}
			}
		})
	}
}
