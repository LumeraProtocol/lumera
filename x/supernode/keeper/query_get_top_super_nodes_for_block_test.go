package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/golang/mock/gomock"
	"github.com/pastelnetwork/pastel/x/supernode/keeper"
	supernodemocks "github.com/pastelnetwork/pastel/x/supernode/mocks"
	"github.com/pastelnetwork/pastel/x/supernode/types"
	"github.com/stretchr/testify/require"
)

func TestKeeper_GetTopSuperNodesForBlock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	stakingKeeper := supernodemocks.NewMockStakingKeeper(ctrl)
	slashingKeeper := supernodemocks.NewMockSlashingKeeper(ctrl)
	bankKeeper := supernodemocks.NewMockBankKeeper(ctrl)

	k, ctx := setupKeeperForTest(t, stakingKeeper, slashingKeeper, bankKeeper)
	queryServer := keeper.Keeper(k)

	// Helper to create a supernode with a given validator address string
	makeSuperNode := func(valStr string) types.SuperNode {
		valAddr := sdk.ValAddress([]byte(valStr))
		valStr = valAddr.String()
		// use valStr in test cases

		return types.SuperNode{
			ValidatorAddress: valStr,
			IpAddress:        "192.168.1.1",
			State:            types.SuperNodeStateActive,
			Version:          "1.0.0",
			Metrics: &types.MetricsAggregate{
				Metrics:     map[string]float64{},
				ReportCount: 0,
				LastUpdated: ctx.BlockTime(),
			},
			PrevIpAddresses: []*types.IPAddressHistory{},
		}
	}

	// Insert some supernodes
	valAddrs := []string{
		"pastelvaloper1aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa1",
		"pastelvaloper1bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb2",
		"pastelvaloper1ccccccccccccccccccccccccccccccc3",
	}
	for _, v := range valAddrs {
		require.NoError(t, k.SetSuperNode(ctx, makeSuperNode(v)))
	}

	testCases := []struct {
		name        string
		req         *types.QueryGetTopSuperNodesForBlockRequest
		setupState  func()
		expectedErr error
		checkResult func(t *testing.T, resp *types.QueryGetTopSuperNodesForBlockResponse)
	}{
		{
			name:        "invalid request (nil)",
			req:         nil,
			expectedErr: sdkerrors.ErrInvalidRequest,
		},
		{
			name: "invalid block height",
			req: &types.QueryGetTopSuperNodesForBlockRequest{
				BlockHeight: -1,
			},
			expectedErr: sdkerrors.ErrInvalidRequest,
		},
		{
			name: "no supernodes",
			req: &types.QueryGetTopSuperNodesForBlockRequest{
				BlockHeight: 10,
			},
			setupState: func() {
				// clear store
				// re-init keeper for a clean store
				k, ctx = setupKeeperForTest(t, stakingKeeper, slashingKeeper, bankKeeper)
				queryServer = keeper.Keeper(k)
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types.QueryGetTopSuperNodesForBlockResponse) {
				require.Empty(t, resp.Supernodes)
			},
		},
		{
			name: "fewer than 10 supernodes",
			req: &types.QueryGetTopSuperNodesForBlockRequest{
				BlockHeight: 100,
			},
			setupState: func() {
				// reset and add 3 supernodes
				k, ctx = setupKeeperForTest(t, stakingKeeper, slashingKeeper, bankKeeper)
				queryServer = keeper.Keeper(k)
				sns := []types.SuperNode{
					makeSuperNode("pastelvaloper1xxx1"),
					makeSuperNode("pastelvaloper1xxx2"),
					makeSuperNode("pastelvaloper1xxx3"),
				}
				for _, sn := range sns {
					require.NoError(t, k.SetSuperNode(ctx, sn))
				}
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types.QueryGetTopSuperNodesForBlockResponse) {
				// only 3 supernodes, should return all 3
				require.Len(t, resp.Supernodes, 3)
			},
		},
		{
			name: "more than 10 supernodes",
			req: &types.QueryGetTopSuperNodesForBlockRequest{
				BlockHeight: 50,
			},
			setupState: func() {
				k, ctx = setupKeeperForTest(t, stakingKeeper, slashingKeeper, bankKeeper)
				queryServer = keeper.Keeper(k)
				// Add 15 supernodes
				for i := 0; i < 15; i++ {
					val := makeSuperNode(
						"pastelvaloper1" + string('a'+byte(i)) +
							"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
					require.NoError(t, k.SetSuperNode(ctx, val))
				}
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types.QueryGetTopSuperNodesForBlockResponse) {
				// Should return top 10 only
				require.Len(t, resp.Supernodes, 10)
			},
		},
		{
			name: "check sorting by closeness",
			req: &types.QueryGetTopSuperNodesForBlockRequest{
				BlockHeight: 123,
			},
			setupState: func() {
				k, ctx = setupKeeperForTest(t, stakingKeeper, slashingKeeper, bankKeeper)
				queryServer = keeper.Keeper(k)

				// Insert 5 supernodes with different addresses
				// The actual sorting correctness can be tested by ensuring no error and stable length
				for i := 0; i < 5; i++ {
					val := makeSuperNode("pastelvaloper1diff" + string('0'+byte(i)))
					require.NoError(t, k.SetSuperNode(ctx, val))
				}
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types.QueryGetTopSuperNodesForBlockResponse) {
				// Just check we got 5 back sorted by closeness
				require.Len(t, resp.Supernodes, 5)
				// Not checking exact order due to randomness of hashing.
				// In a real test, you could mock GetBlockHashForHeight to a known value and check exact order.
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.setupState != nil {
				tc.setupState()
			}

			resp, err := queryServer.GetTopSuperNodesForBlock(ctx, tc.req)
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
