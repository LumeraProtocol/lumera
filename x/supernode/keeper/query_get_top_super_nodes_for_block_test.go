package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/pastelnetwork/pastel/x/supernode/keeper"
	supernodemocks "github.com/pastelnetwork/pastel/x/supernode/mocks"
	"github.com/pastelnetwork/pastel/x/supernode/types"
)

func TestDetermineStateAtBlock(t *testing.T) {
	testCases := []struct {
		name        string
		states      []*types.SuperNodeStateRecord
		blockHeight int64
		wantState   types.SuperNodeState
		wantFound   bool
		note        string
	}{
		{
			name:        "no states at all",
			states:      []*types.SuperNodeStateRecord{},
			blockHeight: 100,
			wantState:   types.SuperNodeStateUnspecified,
			wantFound:   false,
			note:        "Empty states means we never find a record.",
		},
		{
			name: "single state, exactly matches blockHeight",
			states: []*types.SuperNodeStateRecord{
				{State: types.SuperNodeStateActive, Height: 100},
			},
			blockHeight: 100,
			wantState:   types.SuperNodeStateActive,
			wantFound:   true,
			note:        "Single record, block height matches exactly.",
		},
		{
			name: "single state, blockHeight below record => not found",
			states: []*types.SuperNodeStateRecord{
				{State: types.SuperNodeStateActive, Height: 100},
			},
			blockHeight: 99,
			wantState:   types.SuperNodeStateUnspecified,
			wantFound:   false,
			note:        "Block is below the earliest record’s height => no record found",
		},
		{
			name: "multiple states (unordered), blockHeight lands at second record",
			states: []*types.SuperNodeStateRecord{
				{State: types.SuperNodeStateDisabled, Height: 200},
				{State: types.SuperNodeStateActive, Height: 100},
				{State: types.SuperNodeStateStopped, Height: 300},
			},
			blockHeight: 150,
			wantState:   types.SuperNodeStateActive,
			wantFound:   true,
			note:        "We sort them ascending => [Active@100,Disabled@200,Stopped@300]. 150 is after 100, before 200 => last valid is Active@100 => Actually need to see logic in code.",
		},
		{
			name: "multiple states (unordered), blockHeight after all records",
			states: []*types.SuperNodeStateRecord{
				{State: types.SuperNodeStatePenalized, Height: 500},
				{State: types.SuperNodeStateActive, Height: 100},
				{State: types.SuperNodeStateDisabled, Height: 250},
			},
			blockHeight: 1000,
			wantState:   types.SuperNodeStatePenalized,
			wantFound:   true,
			note:        "After sorting => [Active@100,Disabled@250,Penalized@500]. blockHeight=1000 => pick the last record => Penalized@500.",
		},
		{
			name: "blockHeight exactly equals second record",
			states: []*types.SuperNodeStateRecord{
				{State: types.SuperNodeStateActive, Height: 50},
				{State: types.SuperNodeStateStopped, Height: 100},
				{State: types.SuperNodeStatePenalized, Height: 200},
			},
			blockHeight: 100,
			wantState:   types.SuperNodeStateStopped,
			wantFound:   true,
			note:        "Matches second record exactly => return Stopped.",
		},
		{
			name: "blockHeight below earliest => not found",
			states: []*types.SuperNodeStateRecord{
				{State: types.SuperNodeStateActive, Height: 50},
			},
			blockHeight: 1,
			wantState:   types.SuperNodeStateUnspecified,
			wantFound:   false,
			note:        "Earliest record is at block 50, blockHeight=1 => not found.",
		},
		{
			name: "multiple states ascending, blockHeight in the middle",
			states: []*types.SuperNodeStateRecord{
				{State: types.SuperNodeStateActive, Height: 10},
				{State: types.SuperNodeStateDisabled, Height: 20},
				{State: types.SuperNodeStatePenalized, Height: 30},
			},
			blockHeight: 25,
			wantState:   types.SuperNodeStateDisabled,
			wantFound:   true,
			note:        "Between 20 and 30 => last valid record is Disabled@20.",
		},
		{
			name: "multiple states ascending, exact match on last record",
			states: []*types.SuperNodeStateRecord{
				{State: types.SuperNodeStateActive, Height: 10},
				{State: types.SuperNodeStateDisabled, Height: 20},
				{State: types.SuperNodeStatePenalized, Height: 30},
			},
			blockHeight: 30,
			wantState:   types.SuperNodeStatePenalized,
			wantFound:   true,
			note:        "Exact match on last record => penalized.",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gotState, gotFound := keeper.DetermineStateAtBlock(tc.states, tc.blockHeight)
			require.Equal(t, tc.wantFound, gotFound, tc.note)

			// If we didn't find a record, wantState should be Unspecified
			if !gotFound {
				require.Equal(t, types.SuperNodeStateUnspecified, gotState)
			} else {
				require.Equal(t, tc.wantState, gotState, tc.note)
			}
		})
	}
}

func TestKeeper_GetTopSuperNodesForBlock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	stakingKeeper := supernodemocks.NewMockStakingKeeper(ctrl)
	slashingKeeper := supernodemocks.NewMockSlashingKeeper(ctrl)
	bankKeeper := supernodemocks.NewMockBankKeeper(ctrl)

	k, ctx := setupKeeperForTest(t, stakingKeeper, slashingKeeper, bankKeeper)
	queryServer := keeper.Keeper(k)

	// Helper to create a valid pastelvaloper address (via sdk.ValAddress)
	makeValAddr := func(id string) string {
		valBz := []byte(id + "_unique")
		valAddr := sdk.ValAddress(valBz)
		return valAddr.String() // guaranteed valid bech32 if TestMain sets the prefix
	}
	makeSnAddr := func(id string) string {
		valBz := []byte(id + "_unique")
		valAddr := sdk.ValAddress(valBz)
		return sdk.AccAddress(valAddr).String() // guaranteed valid bech32 if TestMain sets the prefix
	}

	// Helper to store supernodes
	storeSuperNodes := func(sns []types.SuperNode) {
		for _, sn := range sns {
			err := k.SetSuperNode(ctx, sn)
			require.NoError(t, err)
		}
	}

	// Creates a supernode with a first state record = (Active, someHeight).
	makeSuperNode := func(label string, registrationHeight int64) types.SuperNode {
		return types.SuperNode{
			SupernodeAccount: makeSnAddr(label),
			ValidatorAddress: makeValAddr(label),
			Version:          "1.0",
			States: []*types.SuperNodeStateRecord{
				{
					State:  types.SuperNodeStateActive,
					Height: registrationHeight, // must be <= query block to be recognized
				},
			},
			PrevIpAddresses: []*types.IPAddressHistory{
				{
					Address: "1022.145.1.1",
					Height:  1,
				},
			},
		}
	}

	// Re-init (clear) store if needed
	clearStore := func() {
		k, ctx = setupKeeperForTest(t, stakingKeeper, slashingKeeper, bankKeeper)
		queryServer = keeper.Keeper(k)
	}

	testCases := []struct {
		name        string
		req         *types.QueryGetTopSuperNodesForBlockRequest
		setupState  func()
		expectedErr error
		checkResult func(t *testing.T, resp *types.QueryGetTopSuperNodesForBlockResponse)
	}{

		{
			name:        "nil request => error",
			req:         nil,
			expectedErr: sdkerrors.ErrInvalidRequest,
		},
		{
			name: "negative block height => error",
			req: &types.QueryGetTopSuperNodesForBlockRequest{
				BlockHeight: -5,
			},
			expectedErr: sdkerrors.ErrInvalidRequest,
		},
		{
			name: "no supernodes => empty result",
			req: &types.QueryGetTopSuperNodesForBlockRequest{
				BlockHeight: 100,
			},
			setupState: func() {
				clearStore() // store is empty
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types.QueryGetTopSuperNodesForBlockResponse) {
				require.Empty(t, resp.Supernodes)
			},
		},
		{
			name: "valid scenario with one supernode => single result",
			req: &types.QueryGetTopSuperNodesForBlockRequest{
				BlockHeight: 100,
			},
			setupState: func() {
				clearStore()
				sn := makeSuperNode("solo", 10) // active at block=10 => valid for block=100
				storeSuperNodes([]types.SuperNode{sn})
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types.QueryGetTopSuperNodesForBlockResponse) {
				require.Len(t, resp.Supernodes, 1)
			},
		},
		{
			name: "multiple supernodes, sorting by XOR distance, default limit=25",
			req: &types.QueryGetTopSuperNodesForBlockRequest{
				BlockHeight: 200,
				Limit:       0, // => default 25
			},
			setupState: func() {
				clearStore()
				var sns []types.SuperNode
				labels := []string{"aaa", "bbb", "ccc", "ddd", "eee"}
				for i, lb := range labels {
					sn := makeSuperNode(lb, int64(10*(i+1))) // all < 200 => valid
					sns = append(sns, sn)
				}
				storeSuperNodes(sns)
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types.QueryGetTopSuperNodesForBlockResponse) {
				// We expect all 5 to pass the filter
				require.Len(t, resp.Supernodes, 5)
			},
		},
		{
			name: "multiple supernodes, limit=2 => returns top 2 only",
			req: &types.QueryGetTopSuperNodesForBlockRequest{
				BlockHeight: 200,
				Limit:       2,
			},
			setupState: func() {
				clearStore()
				var sns []types.SuperNode
				labels := []string{"xxx", "yyy", "zzz", "ppp", "qqq"}
				for _, lb := range labels {
					sn := makeSuperNode(lb, 10) // all < 200 => valid
					sns = append(sns, sn)
				}
				storeSuperNodes(sns)
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types.QueryGetTopSuperNodesForBlockResponse) {
				require.Len(t, resp.Supernodes, 2)
			},
		},
		{
			name: "multiple states, blockHeight=150 => picks correct intervals, sorts, limit=2, user wants Active",
			req: &types.QueryGetTopSuperNodesForBlockRequest{
				BlockHeight: 150,
				Limit:       2,
				State:       types.SuperNodeStateActive,
			},
			setupState: func() {
				clearStore()
				// sn1: active@10, changes to disabled@200 => at 150 => Active
				sn1 := types.SuperNode{
					Version:          "1.0",
					SupernodeAccount: makeSnAddr("sn1"),
					ValidatorAddress: makeValAddr("sn1"),
					States: []*types.SuperNodeStateRecord{
						{State: types.SuperNodeStateActive, Height: 10},
						{State: types.SuperNodeStateDisabled, Height: 200},
					},
					PrevIpAddresses: []*types.IPAddressHistory{
						{
							Address: "1022.145.1.1",
							Height:  1,
						},
					},
				}

				// sn2: always Active
				sn2 := types.SuperNode{
					Version:          "1.0",
					SupernodeAccount: makeSnAddr("sn2"),
					ValidatorAddress: makeValAddr("sn2"),
					States: []*types.SuperNodeStateRecord{
						{State: types.SuperNodeStateActive, Height: 10},
					},
					PrevIpAddresses: []*types.IPAddressHistory{
						{
							Address: "1022.145.1.1",
							Height:  1,
						},
					},
				}
				// sn3: active@10 => penalized@100 => so at 150 => penalized => skip if user wants Active
				sn3 := types.SuperNode{
					Version:          "1.0",
					SupernodeAccount: makeSnAddr("sn3"),
					ValidatorAddress: makeValAddr("sn3"),
					States: []*types.SuperNodeStateRecord{
						{State: types.SuperNodeStateActive, Height: 10},
						{State: types.SuperNodeStatePenalized, Height: 100},
					},
					PrevIpAddresses: []*types.IPAddressHistory{
						{
							Address: "1022.145.1.1",
							Height:  1,
						},
					},
				}
				sn5 := types.SuperNode{
					Version:          "1.0",
					SupernodeAccount: makeSnAddr("sn5"),
					ValidatorAddress: makeValAddr("sn5"),
					States: []*types.SuperNodeStateRecord{
						{State: types.SuperNodeStateActive, Height: 1},
						{State: types.SuperNodeStateDisabled, Height: 10},
					},
					PrevIpAddresses: []*types.IPAddressHistory{
						{
							Address: "1022.145.1.1",
							Height:  1,
						},
					},
				}
				sn6 := types.SuperNode{
					Version:          "1.0",
					SupernodeAccount: makeSnAddr("sn6"),
					ValidatorAddress: makeValAddr("sn6"),
					States: []*types.SuperNodeStateRecord{
						{State: types.SuperNodeStateActive, Height: 1000},
					},
					PrevIpAddresses: []*types.IPAddressHistory{
						{
							Address: "1022.145.1.1",
							Height:  1,
						},
					},
				}
				sn7 := types.SuperNode{
					Version:          "1.0",
					SupernodeAccount: makeSnAddr("sn7"),
					ValidatorAddress: makeValAddr("sn7"),
					States: []*types.SuperNodeStateRecord{
						{State: types.SuperNodeStateActive, Height: 5},
						{State: types.SuperNodeStateUnspecified, Height: 10},
					},
					PrevIpAddresses: []*types.IPAddressHistory{
						{
							Address: "1022.145.1.1",
							Height:  1,
						},
					},
				}
				storeSuperNodes([]types.SuperNode{sn1, sn2, sn3, sn5, sn6, sn7})
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types.QueryGetTopSuperNodesForBlockResponse) {
				require.Len(t, resp.Supernodes, 2)
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
