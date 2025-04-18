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

func TestMsgServer_UpdateSupernode(t *testing.T) {
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
	}

	testCases := []struct {
		name          string
		msg           *types.MsgUpdateSupernode
		setupMock     func(sk *supernodemocks.MockStakingKeeper, slk *supernodemocks.MockSlashingKeeper, bk *supernodemocks.MockBankKeeper)
		setupState    func(k keeper.Keeper, ctx sdk.Context)
		expectedError error
		checkResult   func(t *testing.T, k keeper.Keeper, ctx sdk.Context)
	}{
		{
			name: "successful update with no changes",
			msg: &types.MsgUpdateSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				IpAddress:        "",
				Version:          "",
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				require.NoError(t, k.SetSuperNode(ctx, existingSupernode))
			},
			expectedError: nil,
			checkResult: func(t *testing.T, k keeper.Keeper, ctx sdk.Context) {
				sn, found := k.QuerySuperNode(ctx, valAddr)
				require.True(t, found)
				require.Equal(t, "1.0.0", sn.Version)
			},
		},
		{
			name: "successful update with IP change and version change",
			msg: &types.MsgUpdateSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				IpAddress:        "192.168.1.1",
				Version:          "1.1.0",
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				newSupernode := types.SuperNode{
					SupernodeAccount: otherCreatorAddr.String(),
					ValidatorAddress: valAddr.String(),
					Version:          "1.0.0",
					PrevIpAddresses: []*types.IPAddressHistory{
						{
							Address: "10.0.1.1",
							Height:  1,
						},
					},
					States: []*types.SuperNodeStateRecord{
						{
							State:  types.SuperNodeStateActive,
							Height: 1,
						},
					},
				}
				require.NoError(t, k.SetSuperNode(ctx, newSupernode))
			},
			expectedError: nil,
			checkResult: func(t *testing.T, k keeper.Keeper, ctx sdk.Context) {
				sn, found := k.QuerySuperNode(ctx, valAddr)
				require.True(t, found)
				require.Equal(t, "1.1.0", sn.Version)
				require.Len(t, sn.PrevIpAddresses, 2)
				require.Equal(t, "10.0.1.1", sn.PrevIpAddresses[0].Address)
				require.Equal(t, "192.168.1.1", sn.PrevIpAddresses[1].Address)
			},
		},
		{
			name: "successful update with supernode account change",
			msg: &types.MsgUpdateSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				IpAddress:        "192.168.1.1",
				Version:          "1.1.0",
				SupernodeAccount: creatorAddr.String(),
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				newSupernode := types.SuperNode{
					SupernodeAccount: otherCreatorAddr.String(),
					ValidatorAddress: valAddr.String(),
					Version:          "1.0.0",
					PrevIpAddresses: []*types.IPAddressHistory{
						{
							Address: "10.0.1.1",
							Height:  1,
						},
					},
					States: []*types.SuperNodeStateRecord{
						{
							State:  types.SuperNodeStateActive,
							Height: 1,
						},
					},
				}
				require.NoError(t, k.SetSuperNode(ctx, newSupernode))
			},
			expectedError: nil,
			checkResult: func(t *testing.T, k keeper.Keeper, ctx sdk.Context) {
				sn, found := k.QuerySuperNode(ctx, valAddr)
				require.True(t, found)
				require.Equal(t, "1.1.0", sn.Version)
				require.Len(t, sn.PrevIpAddresses, 2)
				require.Equal(t, "10.0.1.1", sn.PrevIpAddresses[0].Address)
				require.Equal(t, "192.168.1.1", sn.PrevIpAddresses[1].Address)
				require.Equal(t, sn.SupernodeAccount, creatorAddr.String())

			},
		},
		{
			name: "invalid validator address",
			msg: &types.MsgUpdateSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: "invalid",
			},
			expectedError: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "supernode not found",
			msg: &types.MsgUpdateSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
			},
			expectedError: sdkerrors.ErrNotFound,
		},
		{
			name: "unauthorized updater",
			msg: &types.MsgUpdateSupernode{
				Creator:          otherCreatorAddr.String(),
				ValidatorAddress: valAddr.String(),
			},
			setupState: func(k keeper.Keeper, ctx sdk.Context) {
				require.NoError(t, k.SetSuperNode(ctx, existingSupernode))
			},
			expectedError: sdkerrors.ErrUnauthorized,
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
			_, err := msgServer.UpdateSupernode(sdk.WrapSDKContext(ctx), tc.msg)

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
