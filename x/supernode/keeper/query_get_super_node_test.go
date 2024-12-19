package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/pastelnetwork/pastel/x/supernode/keeper"
	supernodemocks "github.com/pastelnetwork/pastel/x/supernode/mocks"
	"github.com/pastelnetwork/pastel/x/supernode/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestKeeper_GetSuperNode(t *testing.T) {
	valAddr := sdk.ValAddress([]byte("validator"))
	anotherValAddr := sdk.ValAddress([]byte("another-validator"))

	sn := types.SuperNode{
		ValidatorAddress: valAddr.String(),
		IpAddress:        "192.168.1.1",
		State:            types.SuperNodeStateActive,
		Version:          "1.0.0",
	}

	testCases := []struct {
		name        string
		req         *types.QueryGetSuperNodeRequest
		setupState  func(k keeper.Keeper, ctx sdk.Context)
		expectedErr error
		checkResult func(t *testing.T, resp *types.QueryGetSuperNodeResponse)
	}{
		{
			name:        "invalid request (nil)",
			req:         nil,
			expectedErr: status.Error(codes.InvalidArgument, "invalid request"),
		},
		{
			name: "invalid validator address",
			req: &types.QueryGetSuperNodeRequest{
				ValidatorAddress: "invalid",
			},
			expectedErr: status.Error(codes.InvalidArgument, "invalid validator address"),
		},
		{
			name: "supernode not found",
			req: &types.QueryGetSuperNodeRequest{
				ValidatorAddress: anotherValAddr.String(),
			},
			expectedErr: status.Error(codes.NotFound, "no supernode found"),
		},
		{
			name: "supernode found",
			req: &types.QueryGetSuperNodeRequest{
				ValidatorAddress: valAddr.String(),
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				require.NoError(t, k.SetSuperNode(ctx, sn))
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types.QueryGetSuperNodeResponse) {
				require.NotNil(t, resp.Supernode)
				require.Equal(t, sn, *resp.Supernode)
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

			if tc.setupState != nil {
				tc.setupState(k, ctx)
			}

			resp, err := k.GetSuperNode(sdk.WrapSDKContext(ctx), tc.req)

			if tc.expectedErr != nil {
				require.Error(t, err)
				// Since the error might contain additional text, use Contains to verify the code
				st, _ := status.FromError(err)
				expectedStatus, _ := status.FromError(tc.expectedErr)
				require.Equal(t, expectedStatus.Code(), st.Code())
			} else {
				require.NoError(t, err)
				if tc.checkResult != nil {
					tc.checkResult(t, resp)
				}
			}
		})
	}
}
