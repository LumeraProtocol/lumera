package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	supernodemocks "github.com/LumeraProtocol/lumera/x/supernode/v1/mocks"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"go.uber.org/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestKeeper_GetSuperNodeBySuperNodeAddress(t *testing.T) {
	valAddr := sdk.ValAddress([]byte("validator"))
	creatorAddr := sdk.AccAddress(valAddr)

	sn := types.SuperNode{
		SupernodeAccount: string(creatorAddr.String()),
		ValidatorAddress: valAddr.String(),
		Note:             "1.0.0",
		PrevIpAddresses: []*types.IPAddressHistory{
			{
				Address: "1022.145.1.1",
				Height:  1,
			},
		},
		States: []*types.SuperNodeStateRecord{
			{
				State:  types.SuperNodeStateActive,
				Height: 1,
			},
		},
		P2PPort: "4445",
	}

	testCases := []struct {
		name        string
		req         *types.QueryGetSuperNodeBySuperNodeAddressRequest
		setupState  func(k keeper.Keeper, ctx sdk.Context)
		expectedErr error
		checkResult func(t *testing.T, resp *types.QueryGetSuperNodeBySuperNodeAddressResponse)
	}{
		{
			name:        "invalid request (nil)",
			req:         nil,
			expectedErr: status.Error(codes.InvalidArgument, "invalid request"),
		},
		{
			name: "supernode not found",
			req: &types.QueryGetSuperNodeBySuperNodeAddressRequest{
				SupernodeAddress: "non-existent-address",
			},
			expectedErr: status.Error(codes.NotFound, "supernode not found"),
		},
		{
			name: "supernode found",
			req: &types.QueryGetSuperNodeBySuperNodeAddressRequest{
				SupernodeAddress: creatorAddr.String(),
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				require.NoError(t, k.SetSuperNode(ctx, sn))
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types.QueryGetSuperNodeBySuperNodeAddressResponse) {
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
			q := keeper.NewQueryServerImpl(k)

			if tc.setupState != nil {
				tc.setupState(k, ctx)
			}

			resp, err := q.GetSuperNodeBySuperNodeAddress(ctx, tc.req)

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
