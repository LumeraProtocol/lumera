package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"go.uber.org/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	supernodemocks "github.com/LumeraProtocol/lumera/x/supernode/v1/mocks"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
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
			note:        "Block is below the earliest record's height => no record found",
		},
		{
			name: "multiple states, blockHeight between records",
			states: []*types.SuperNodeStateRecord{
				{State: types.SuperNodeStateActive, Height: 100},
				{State: types.SuperNodeStateStopped, Height: 200},
				{State: types.SuperNodeStateStopped, Height: 300},
			},
			blockHeight: 150,
			wantState:   types.SuperNodeStateActive,
			wantFound:   true,
			note:        "Append-only history; 150 is after 100 and before 200 => Active@100.",
		},
		{
			name: "multiple states, blockHeight after all records",
			states: []*types.SuperNodeStateRecord{
				{State: types.SuperNodeStateActive, Height: 100},
				{State: types.SuperNodeStateStopped, Height: 250},
				{State: types.SuperNodeStateStopped, Height: 500},
			},
			blockHeight: 1000,
			wantState:   types.SuperNodeStateStopped,
			wantFound:   true,
			note:        "After all records => last record wins => Stopped@500.",
		},
		{
			name: "blockHeight exactly equals second record",
			states: []*types.SuperNodeStateRecord{
				{State: types.SuperNodeStateActive, Height: 50},
				{State: types.SuperNodeStateStopped, Height: 100},
				{State: types.SuperNodeStateStopped, Height: 200},
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
				{State: types.SuperNodeStateStopped, Height: 20},
				{State: types.SuperNodeStateStopped, Height: 30},
			},
			blockHeight: 25,
			wantState:   types.SuperNodeStateStopped,
			wantFound:   true,
			note:        "Between 20 and 30 => last valid record is Stopped@20.",
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
	q := keeper.NewQueryServerImpl(k)

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
	storeSuperNodes := func(sns []types.SuperNode) {
		for _, sn := range sns {
			err := k.SetSuperNode(ctx, sn)
			require.NoError(t, err)
		}
	}

	clearStore := func() {
		k, ctx = setupKeeperForTest(t, stakingKeeper, slashingKeeper, bankKeeper)
		q = keeper.NewQueryServerImpl(k)
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
				clearStore()
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types.QueryGetTopSuperNodesForBlockResponse) {
				require.Empty(t, resp.Supernodes)
			},
		},
		{
			name: "multiple states, blockHeight=150, state=Stopped, limit=2",
			req: &types.QueryGetTopSuperNodesForBlockRequest{
				BlockHeight: 150,
				Limit:       2,
				State:       "SUPERNODE_STATE_STOPPED",
			},
			setupState: func() {
				clearStore()
				sn1 := types.SuperNode{
					Note:             "1.0",
					SupernodeAccount: makeSnAddr("sn1"),
					ValidatorAddress: makeValAddr("sn1"),
					States: []*types.SuperNodeStateRecord{
						{State: types.SuperNodeStateActive, Height: 10},
						{State: types.SuperNodeStateStopped, Height: 100},
					},
					PrevIpAddresses: []*types.IPAddressHistory{
						{
							Address: "192.168.1.1",
							Height:  1,
						},
					},
					P2PPort: "26657",
				}

				sn2 := types.SuperNode{
					Note:             "1.0",
					SupernodeAccount: makeSnAddr("sn2"),
					ValidatorAddress: makeValAddr("sn2"),
					States: []*types.SuperNodeStateRecord{
						{State: types.SuperNodeStateStopped, Height: 10},
					},
					PrevIpAddresses: []*types.IPAddressHistory{
						{
							Address: "192.168.1.1",
							Height:  1,
						},
					},
					P2PPort: "26657",
				}

				sn3 := types.SuperNode{
					Note:             "1.0",
					SupernodeAccount: makeSnAddr("sn3"),
					ValidatorAddress: makeValAddr("sn3"),
					States: []*types.SuperNodeStateRecord{
						{State: types.SuperNodeStateActive, Height: 10},
						{State: types.SuperNodeStateStopped, Height: 100},
					},
					PrevIpAddresses: []*types.IPAddressHistory{
						{
							Address: "192.168.1.1",
							Height:  1,
						},
					},
					P2PPort: "26657",
				}
				storeSuperNodes([]types.SuperNode{sn1, sn2, sn3})
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types.QueryGetTopSuperNodesForBlockResponse) {
				require.Len(t, resp.Supernodes, 2)
				// Verify all returned nodes are in stopped state
				for _, node := range resp.Supernodes {
					lastState := node.States[len(node.States)-1]
					require.Equal(t, types.SuperNodeStateStopped, lastState.State)
				}
			},
		},
		{
			name: "multiple states with large limit",
			req: &types.QueryGetTopSuperNodesForBlockRequest{
				BlockHeight: 200,
				Limit:       100,
				State:       "SUPERNODE_STATE_STOPPED",
			},
			setupState: func() {
				clearStore()
				sn1 := types.SuperNode{
					Note:             "1.0",
					SupernodeAccount: makeSnAddr("sn1"),
					ValidatorAddress: makeValAddr("sn1"),
					States: []*types.SuperNodeStateRecord{
						{State: types.SuperNodeStateStopped, Height: 10},
					},
					PrevIpAddresses: []*types.IPAddressHistory{
						{
							Address: "192.168.1.1",
							Height:  1,
						},
					},
					P2PPort: "26657",
				}
				storeSuperNodes([]types.SuperNode{sn1})
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *types.QueryGetTopSuperNodesForBlockResponse) {
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

			resp, err := q.GetTopSuperNodesForBlock(ctx, tc.req)
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
