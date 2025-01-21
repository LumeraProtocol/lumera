package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/supernode/keeper"
	supernodemocks "github.com/LumeraProtocol/lumera/x/supernode/mocks"
	"github.com/LumeraProtocol/lumera/x/supernode/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestMsgServer_StopSupernode(t *testing.T) {
	valAddr := sdk.ValAddress([]byte("validator"))
	creatorAddr := sdk.AccAddress(valAddr)

	otherValAddr := sdk.ValAddress([]byte("other-validator"))
	otherCreatorAddr := sdk.AccAddress(otherValAddr)

	existingSupernode := types.SuperNode{
		SupernodeAccount: otherCreatorAddr.String(),
		ValidatorAddress: valAddr.String(),
		Version:          "1.0.0",
		PrevIpAddresses: []*types.IPAddressHistory{
			{
				Address: "192.145.1.1",
			},
		},
	}

	testCases := []struct {
		name          string
		msg           *types.MsgStopSupernode
		setupMock     func(sk *supernodemocks.MockStakingKeeper, slk *supernodemocks.MockSlashingKeeper, bk *supernodemocks.MockBankKeeper)
		setupState    func(k keeper.Keeper, ctx sdk.Context)
		expectedError error
		checkResult   func(t *testing.T, k keeper.Keeper, ctx sdk.Context)
	}{
		{
			name: "successful stop",
			msg: &types.MsgStopSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				Reason:           "maintenance",
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				existingSupernode.States = []*types.SuperNodeStateRecord{
					{
						State:  types.SuperNodeStateActive,
						Height: 1,
					},
				}
				require.NoError(t, k.SetSuperNode(ctx, existingSupernode))

			},
			expectedError: nil,
			checkResult: func(t *testing.T, k keeper.Keeper, ctx sdk.Context) {
				_, found := k.QuerySuperNode(ctx, valAddr)
				require.True(t, found)
			},
		},
		{
			name: "invalid validator address",
			msg: &types.MsgStopSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: "invalid",
				Reason:           "maintenance",
			},
			expectedError: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "supernode not found",
			msg: &types.MsgStopSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				Reason:           "node down",
			},
			expectedError: sdkerrors.ErrNotFound,
		},
		{
			name: "unauthorized",
			msg: &types.MsgStopSupernode{
				Creator:          otherCreatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				Reason:           "other reason",
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				require.NoError(t, k.SetSuperNode(ctx, existingSupernode))
			},
			expectedError: sdkerrors.ErrUnauthorized,
		},
		{
			name: "supernode already stopped",
			msg: &types.MsgStopSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				Reason:           "maintenance",
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				existingSupernode.States = []*types.SuperNodeStateRecord{
					{
						State:  types.SuperNodeStateActive,
						Height: 1,
					},
					{
						State:  types.SuperNodeStateStopped,
						Height: 2,
					},
				}
				require.NoError(t, k.SetSuperNode(ctx, existingSupernode))

			},
			expectedError: sdkerrors.ErrInvalidRequest,
			checkResult: func(t *testing.T, k keeper.Keeper, ctx sdk.Context) {
				_, found := k.QuerySuperNode(ctx, valAddr)
				require.True(t, found)
			},
		},
		{
			name: "supernode disabled",
			msg: &types.MsgStopSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				Reason:           "maintenance",
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				existingSupernode.States = []*types.SuperNodeStateRecord{
					{
						State:  types.SuperNodeStateActive,
						Height: 1,
					},
					{
						State:  types.SuperNodeStateDisabled,
						Height: 2,
					},
				}
				require.NoError(t, k.SetSuperNode(ctx, existingSupernode))

			},
			expectedError: sdkerrors.ErrInvalidRequest,
			checkResult: func(t *testing.T, k keeper.Keeper, ctx sdk.Context) {
				_, found := k.QuerySuperNode(ctx, valAddr)
				require.True(t, found)
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

			if tc.setupMock != nil {
				tc.setupMock(stakingKeeper, slashingKeeper, bankKeeper)
			}

			k, ctx := setupKeeperForTest(t, stakingKeeper, slashingKeeper, bankKeeper)
			if tc.setupState != nil {
				tc.setupState(k, ctx)
			}

			msgServer := keeper.NewMsgServerImpl(k)
			_, err := msgServer.StopSupernode(ctx, tc.msg)

			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
			} else {
				require.NoError(t, err)
			}

			if tc.checkResult != nil {
				tc.checkResult(t, k, ctx)
			}
		})
	}
}
