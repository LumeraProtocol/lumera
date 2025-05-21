package keeper_test

import (
	"testing"

	keeper2 "github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	supernodemocks "github.com/LumeraProtocol/lumera/x/supernode/v1/mocks"
	types2 "github.com/LumeraProtocol/lumera/x/supernode/v1/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestDetermineStateAtBlock(t *testing.T) {
	testCases := []struct {
		name        string
		states      []*types2.SuperNodeStateRecord
		blockHeight int64
		wantState   types2.SuperNodeState
		wantFound   bool
		note        string
	}{
		{
			name:        "no states at all",
			states:      []*types2.SuperNodeStateRecord{},
			blockHeight: 100,
			wantState:   types2.SuperNodeStateUnspecified,
			wantFound:   false,
			note:        "Empty states means we never find a record.",
		},
		{
			name: "single state, exactly matches blockHeight",
			states: []*types2.SuperNodeStateRecord{
				{State: types2.SuperNodeStateActive, Height: 100},
			},
			blockHeight: 100,
			wantState:   types2.SuperNodeStateActive,
			wantFound:   true,
			note:        "Single record, block height matches exactly.",
		},
		{
			name: "single state, blockHeight below record => not found",
			states: []*types2.SuperNodeStateRecord{
				{State: types2.SuperNodeStateActive, Height: 100},
			},
			blockHeight: 99,
			wantState:   types2.SuperNodeStateUnspecified,
			wantFound:   false,
			note:        "Block is below the earliest record's height => no record found",
		},
		{
			name: "multiple states (unordered), blockHeight lands at second record",
			states: []*types2.SuperNodeStateRecord{
				{State: types2.SuperNodeStateStopped, Height: 200},
				{State: types2.SuperNodeStateActive, Height: 100},
				{State: types2.SuperNodeStateStopped, Height: 300},
			},
			blockHeight: 150,
			wantState:   types2.SuperNodeStateActive,
			wantFound:   true,
			note:        "We sort them ascending => [Active@100,Stopped@200,Stopped@300]. 150 is after 100, before 200 => last valid is Active@100.",
		},
		{
			name: "multiple states (unordered), blockHeight after all records",
			states: []*types2.SuperNodeStateRecord{
				{State: types2.SuperNodeStateStopped, Height: 500},
				{State: types2.SuperNodeStateActive, Height: 100},
				{State: types2.SuperNodeStateStopped, Height: 250},
			},
			blockHeight: 1000,
			wantState:   types2.SuperNodeStateStopped,
			wantFound:   true,
			note:        "After sorting => [Active@100,Stopped@250,Stopped@500]. blockHeight=1000 => pick the last record => Stopped@500.",
		},
		{
			name: "blockHeight exactly equals second record",
			states: []*types2.SuperNodeStateRecord{
				{State: types2.SuperNodeStateActive, Height: 50},
				{State: types2.SuperNodeStateStopped, Height: 100},
				{State: types2.SuperNodeStateStopped, Height: 200},
			},
			blockHeight: 100,
			wantState:   types2.SuperNodeStateStopped,
			wantFound:   true,
			note:        "Matches second record exactly => return Stopped.",
		},
		{
			name: "blockHeight below earliest => not found",
			states: []*types2.SuperNodeStateRecord{
				{State: types2.SuperNodeStateActive, Height: 50},
			},
			blockHeight: 1,
			wantState:   types2.SuperNodeStateUnspecified,
			wantFound:   false,
			note:        "Earliest record is at block 50, blockHeight=1 => not found.",
		},
		{
			name: "multiple states ascending, blockHeight in the middle",
			states: []*types2.SuperNodeStateRecord{
				{State: types2.SuperNodeStateActive, Height: 10},
				{State: types2.SuperNodeStateStopped, Height: 20},
				{State: types2.SuperNodeStateStopped, Height: 30},
			},
			blockHeight: 25,
			wantState:   types2.SuperNodeStateStopped,
			wantFound:   true,
			note:        "Between 20 and 30 => last valid record is Stopped@20.",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gotState, gotFound := keeper2.DetermineStateAtBlock(tc.states, tc.blockHeight)
			require.Equal(t, tc.wantFound, gotFound, tc.note)

			// If we didn't find a record, wantState should be Unspecified
			if !gotFound {
				require.Equal(t, types2.SuperNodeStateUnspecified, gotState)
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
	queryServer := keeper2.Keeper(k)

	// Helper to create a valid lumeravaloper address (via sdk.ValAddress)
	makeValAddr := func(id string) string {
		valBz := []byte(id + "_unique")
		valAddr := sdk.ValAddress(valBz)
		return valAddr.String()
	}
	makeSnAddr := func(id string) string {
		valBz := []byte(id + "_unique")
		valAddr := sdk.ValAddress(valBz)
		return sdk.AccAddress(valAddr).String()
	}

	// Helper to store supernodes
	storeSuperNodes := func(sns []types2.SuperNode) {
		for _, sn := range sns {
			err := k.SetSuperNode(ctx, sn)
			require.NoError(t, err)
		}
	}

	clearStore := func() {
		k, ctx = setupKeeperForTest(t, stakingKeeper, slashingKeeper, bankKeeper)
		queryServer = keeper2.Keeper(k)
	}

	testCases := []struct {
		name        string
		req         *types2.QueryGetTopSuperNodesForBlockRequest
		setupState  func()
		expectedErr error
		checkResult func(t *testing.T, resp *types2.QueryGetTopSuperNodesForBlockResponse)
	}{
		{
			name:        "nil request => error",
			req:         nil,
			expectedErr: sdkerrors.ErrInvalidRequest,
		},
		{
			name: "negative block height => error",
			req: &types2.QueryGetTopSuperNodesForBlockRequest{
				BlockHeight: -5,
			},
			expectedErr: sdkerrors.ErrInvalidRequest,
		},
		{
			name: "no supernodes => empty result",
			req: &types2.QueryGetTopSuperNodesForBlockRequest{
				BlockHeight: 100,
			},
			setupState: func() {
				clearStore()
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types2.QueryGetTopSuperNodesForBlockResponse) {
				require.Empty(t, resp.Supernodes)
			},
		},
		{
			name: "multiple states, blockHeight=150, state=Stopped, limit=2",
			req: &types2.QueryGetTopSuperNodesForBlockRequest{
				BlockHeight: 150,
				Limit:       2,
				State:       "SUPERNODE_STATE_STOPPED",
			},
			setupState: func() {
				clearStore()
				sn1 := types2.SuperNode{
					Version:          "1.0",
					SupernodeAccount: makeSnAddr("sn1"),
					ValidatorAddress: makeValAddr("sn1"),
					States: []*types2.SuperNodeStateRecord{
						{State: types2.SuperNodeStateActive, Height: 10},
						{State: types2.SuperNodeStateStopped, Height: 100},
					},
					PrevIpAddresses: []*types2.IPAddressHistory{
						{
							Address: "192.168.1.1",
							Height:  1,
						},
					},
					P2PPort: "26657",
				}

				sn2 := types2.SuperNode{
					Version:          "1.0",
					SupernodeAccount: makeSnAddr("sn2"),
					ValidatorAddress: makeValAddr("sn2"),
					States: []*types2.SuperNodeStateRecord{
						{State: types2.SuperNodeStateStopped, Height: 10},
					},
					PrevIpAddresses: []*types2.IPAddressHistory{
						{
							Address: "192.168.1.1",
							Height:  1,
						},
					},
					P2PPort: "26657",
				}

				sn3 := types2.SuperNode{
					Version:          "1.0",
					SupernodeAccount: makeSnAddr("sn3"),
					ValidatorAddress: makeValAddr("sn3"),
					States: []*types2.SuperNodeStateRecord{
						{State: types2.SuperNodeStateActive, Height: 10},
						{State: types2.SuperNodeStateStopped, Height: 100},
					},
					PrevIpAddresses: []*types2.IPAddressHistory{
						{
							Address: "192.168.1.1",
							Height:  1,
						},
					},
					P2PPort: "26657",
				}
				storeSuperNodes([]types2.SuperNode{sn1, sn2, sn3})
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types2.QueryGetTopSuperNodesForBlockResponse) {
				require.Len(t, resp.Supernodes, 2)
				// Verify all returned nodes are in stopped state
				for _, node := range resp.Supernodes {
					lastState := node.States[len(node.States)-1]
					require.Equal(t, types2.SuperNodeStateStopped, lastState.State)
				}
			},
		},
		{
			name: "multiple states with large limit",
			req: &types2.QueryGetTopSuperNodesForBlockRequest{
				BlockHeight: 200,
				Limit:       100,
				State:       "SUPERNODE_STATE_STOPPED",
			},
			setupState: func() {
				clearStore()
				sn1 := types2.SuperNode{
					Version:          "1.0",
					SupernodeAccount: makeSnAddr("sn1"),
					ValidatorAddress: makeValAddr("sn1"),
					States: []*types2.SuperNodeStateRecord{
						{State: types2.SuperNodeStateStopped, Height: 10},
					},
					PrevIpAddresses: []*types2.IPAddressHistory{
						{
							Address: "192.168.1.1",
							Height:  1,
						},
					},
					P2PPort: "26657",
				}
				storeSuperNodes([]types2.SuperNode{sn1})
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types2.QueryGetTopSuperNodesForBlockResponse) {
				require.Len(t, resp.Supernodes, 1)
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
