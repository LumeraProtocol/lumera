package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	supernodemocks "github.com/LumeraProtocol/lumera/x/supernode/v1/mocks"
	types2 "github.com/LumeraProtocol/lumera/x/supernode/v1/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestKeeper_ListSuperNodes(t *testing.T) {
	valAddr := sdk.ValAddress([]byte("validator"))
	creatorAddr := sdk.AccAddress(valAddr)
	// Create action supernodes
	sn1 := types2.SuperNode{
		ValidatorAddress: valAddr.String(),
		SupernodeAccount: creatorAddr.String(),
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
	sn2 := types2.SuperNode{
		ValidatorAddress: sdk.ValAddress([]byte("val2")).String(),
		SupernodeAccount: creatorAddr.String(),
		Version:          "2.0.0",
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
		req         *types2.QueryListSuperNodesRequest
		setupState  func(k keeper.Keeper, ctx sdk.Context)
		expectedErr error
		checkResult func(t *testing.T, resp *types2.QueryListSuperNodesResponse)
	}{
		{
			name:        "invalid request (nil)",
			req:         nil,
			expectedErr: status.Error(codes.InvalidArgument, "invalid request"),
		},
		{
			name: "no supernodes in store",
			req: &types2.QueryListSuperNodesRequest{
				Pagination: &query.PageRequest{Limit: 10},
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				// no state set, empty store
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types2.QueryListSuperNodesResponse) {
				require.Empty(t, resp.Supernodes)
				require.Nil(t, resp.Pagination.NextKey)
			},
		},
		{
			name: "multiple supernodes, no pagination",
			req: &types2.QueryListSuperNodesRequest{
				Pagination: &query.PageRequest{Limit: 10},
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				sn1.States = []*types2.SuperNodeStateRecord{
					{
						State:  types2.SuperNodeStateActive,
						Height: 1,
					},
					{
						State:  types2.SuperNodeStateDisabled,
						Height: 2,
					},
				}
				sn2.States = []*types2.SuperNodeStateRecord{
					{
						State:  types2.SuperNodeStateActive,
						Height: 1,
					},
					{
						State:  types2.SuperNodeStatePenalized,
						Height: 2,
					},
				}
				require.NoError(t, k.SetSuperNode(ctx, sn1))
				require.NoError(t, k.SetSuperNode(ctx, sn2))
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types2.QueryListSuperNodesResponse) {
				require.Len(t, resp.Supernodes, 2)
				// Just check that both were returned
				addrSet := make(map[string]bool)
				for _, sn := range resp.Supernodes {
					addrSet[sn.ValidatorAddress] = true
				}
				require.True(t, addrSet[sn1.ValidatorAddress])
				require.True(t, addrSet[sn2.ValidatorAddress])
			},
		},
		{
			name: "pagination with fewer results",
			req: &types2.QueryListSuperNodesRequest{
				Pagination: &query.PageRequest{Limit: 1},
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				require.NoError(t, k.SetSuperNode(ctx, sn1))
				require.NoError(t, k.SetSuperNode(ctx, sn2))
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types2.QueryListSuperNodesResponse) {
				require.Len(t, resp.Supernodes, 1)
				require.NotNil(t, resp.Pagination.NextKey)
				// The test only checks first page. Additional pages would be tested similarly in other tests.
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

			resp, err := k.ListSuperNodes(ctx, tc.req)

			if tc.expectedErr != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
				if tc.checkResult != nil {
					tc.checkResult(t, resp)
				}
			}
		})
	}
}
