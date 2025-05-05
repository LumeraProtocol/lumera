package keeper_test

import (
	"cosmossdk.io/math"
	"github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	types2 "github.com/LumeraProtocol/lumera/x/action/v1/types"
	"testing"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestKeeper_GetActionFee(t *testing.T) {
	testCases := []struct {
		name        string
		req         *types2.QueryGetActionFeeRequest
		setupParams func(k keeper.Keeper, ctx sdk.Context)
		expectedFee string
		expectedErr error
	}{
		{
			name:        "nil request",
			req:         nil,
			expectedErr: status.Error(codes.InvalidArgument, "invalid request"),
		},
		{
			name:        "invalid data size",
			req:         &types2.QueryGetActionFeeRequest{DataSize: "invalid"},
			expectedErr: status.Errorf(codes.InvalidArgument, "invalid data_size: strconv.ParseInt: parsing \"invalid\": invalid syntax"),
		},
		{
			name: "valid request with zero data size",
			req:  &types2.QueryGetActionFeeRequest{DataSize: "0"},
			setupParams: func(k keeper.Keeper, ctx sdk.Context) {
				params := types2.DefaultParams()
				params.BaseActionFee = sdk.NewCoin("ulume", math.NewInt(10000))
				params.FeePerByte = sdk.NewCoin("ulume", math.NewInt(100))
				k.SetParams(ctx, params)
			},
			expectedFee: "10000",
		},
		{
			name: "valid request with data size 200",
			req:  &types2.QueryGetActionFeeRequest{DataSize: "200"},
			setupParams: func(k keeper.Keeper, ctx sdk.Context) {
				params := types2.DefaultParams()
				params.BaseActionFee = sdk.NewCoin("ulume", math.NewInt(10000))
				params.FeePerByte = sdk.NewCoin("ulume", math.NewInt(100))
				k.SetParams(ctx, params)
			},
			expectedFee: "30000", // 100 * 200 + 10000
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			k, ctx := keepertest.ActionKeeper(t)
			goCtx := sdk.WrapSDKContext(ctx)

			if tc.setupParams != nil {
				tc.setupParams(k, ctx)
			}

			resp, err := k.GetActionFee(goCtx, tc.req)

			if tc.expectedErr != nil {
				require.Error(t, err)
				st, _ := status.FromError(err)
				expectedStatus, _ := status.FromError(tc.expectedErr)
				require.Equal(t, expectedStatus.Code(), st.Code())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedFee, resp.Amount)
			}
		})
	}
}
