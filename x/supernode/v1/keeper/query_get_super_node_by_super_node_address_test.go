package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	supernodemocks "github.com/LumeraProtocol/lumera/x/supernode/v1/mocks"
	types2 "github.com/LumeraProtocol/lumera/x/supernode/v1/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestKeeper_GetSuperNodeBySuperNodeAddress(t *testing.T) {
	valAddr := sdk.ValAddress([]byte("validator"))
	creatorAddr := sdk.AccAddress(valAddr)

	sn := types2.SuperNode{
		SupernodeAccount: string(creatorAddr.String()),
		ValidatorAddress: valAddr.String(),
		Version:          "1.0.0",
		PrevIpAddresses: []*types2.IPAddressHistory{
			{
				Address: "1022.145.1.1",
				Height:  1,
			},
		},
		States: []*types2.SuperNodeStateRecord{
			{
				State:  types2.SuperNodeStateActive,
				Height: 1,
			},
		},
		P2PPort: "26657",
	}

	testCases := []struct {
		name        string
		req         *types2.QueryGetSuperNodeBySuperNodeAddressRequest
		setupState  func(k keeper.Keeper, ctx sdk.Context)
		expectedErr error
		checkResult func(t *testing.T, resp *types2.QueryGetSuperNodeBySuperNodeAddressResponse)
	}{
		{
			name:        "invalid request (nil)",
			req:         nil,
			expectedErr: status.Error(codes.InvalidArgument, "invalid request"),
		},
		{
			name: "supernode not found",
			req: &types2.QueryGetSuperNodeBySuperNodeAddressRequest{
				SupernodeAddress: "non-existent-address",
			},
			expectedErr: status.Error(codes.NotFound, "supernode not found"),
		},
		{
			name: "supernode found",
			req: &types2.QueryGetSuperNodeBySuperNodeAddressRequest{
				SupernodeAddress: creatorAddr.String(),
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				require.NoError(t, k.SetSuperNode(ctx, sn))
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types2.QueryGetSuperNodeBySuperNodeAddressResponse) {
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

			resp, err := k.GetSuperNodeBySuperNodeAddress(ctx, tc.req)

			if tc.expectedErr != nil {
				require.Error(t, err)
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
