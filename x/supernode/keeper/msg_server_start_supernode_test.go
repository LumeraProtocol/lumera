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

func TestMsgServer_StartSupernode(t *testing.T) {
	valAddr := sdk.ValAddress([]byte("validator"))
	creatorAddr := sdk.AccAddress(valAddr)

	otherValAddr := sdk.ValAddress([]byte("other-validator"))
	otherCreatorAddr := sdk.AccAddress(otherValAddr)

	existingSupernode := types.SuperNode{
		SupernodeAccount: creatorAddr.String(),
		ValidatorAddress: valAddr.String(),
		Version:          "1.0.0",
		States: []*types.SuperNodeStateRecord{
			{
				State:  types.SuperNodeStateActive,
				Height: 1,
			},
			{
				State:  types.SuperNodeStateStopped,
				Height: 1,
			},
		},
	}

	testCases := []struct {
		name          string
		msg           *types.MsgStartSupernode
		setupMock     func(sk *supernodemocks.MockStakingKeeper, slk *supernodemocks.MockSlashingKeeper, bk *supernodemocks.MockBankKeeper)
		setupState    func(k keeper.Keeper, ctx sdk.Context)
		expectedError error
		checkResult   func(t *testing.T, k keeper.Keeper, ctx sdk.Context)
	}{
		{
			name: "successful start",
			msg: &types.MsgStartSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
			},
			setupMock: nil,
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				existingSupernode.PrevIpAddresses = []*types.IPAddressHistory{
					{
						Address: "192.168.1.1",
					},
				}
				require.NoError(t, k.SetSuperNode(ctx, existingSupernode))
			},
			expectedError: nil,
			checkResult: func(t *testing.T, k keeper.Keeper, ctx sdk.Context) {
				sn, found := k.QuerySuperNode(ctx, valAddr)
				require.True(t, found)
				require.Len(t, sn.PrevIpAddresses, 1)
				require.Equal(t, "192.168.1.1", sn.PrevIpAddresses[0].Address)
			},
		},
		{
			name: "invalid validator address",
			msg: &types.MsgStartSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: "invalid",
			},
			expectedError: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "supernode not found",
			msg: &types.MsgStartSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
			},
			expectedError: sdkerrors.ErrNotFound,
		},
		{
			name: "unauthorized",
			msg: &types.MsgStartSupernode{
				Creator:          otherCreatorAddr.String(),
				ValidatorAddress: valAddr.String(),
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				require.NoError(t, k.SetSuperNode(ctx, existingSupernode))
			},
			expectedError: sdkerrors.ErrUnauthorized,
		},
		{
			name: "supernode already active",
			msg: &types.MsgStartSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
			},
			setupMock: nil,
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				existingSupernode.PrevIpAddresses = []*types.IPAddressHistory{
					{
						Address: "192.168.1.1",
					},
				}
				existingSupernode.States = []*types.SuperNodeStateRecord{
					{
						State:  types.SuperNodeStateActive,
						Height: 1,
					},
				}
				require.NoError(t, k.SetSuperNode(ctx, existingSupernode))
			},
			expectedError: sdkerrors.ErrInvalidRequest,
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
			_, err := msgServer.StartSupernode(ctx, tc.msg)

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
