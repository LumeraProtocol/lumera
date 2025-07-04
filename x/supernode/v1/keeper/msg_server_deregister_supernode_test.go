package keeper_test

import (
	"testing"

	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	supernodemocks "github.com/LumeraProtocol/lumera/x/supernode/v1/mocks"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

func TestMsgServer_DeRegisterSupernode(t *testing.T) {
	valAddr := sdk.ValAddress([]byte("validator"))
	creatorAddr := sdk.AccAddress(valAddr)

	otherValAddr := sdk.ValAddress([]byte("other-validator"))
	otherCreatorAddr := sdk.AccAddress(otherValAddr)

	testCases := []struct {
		name          string
		msg           *types.MsgDeregisterSupernode
		currentState  types.SuperNodeState
		expectedError error
	}{
		{
			name: "successful deregistration",
			msg: &types.MsgDeregisterSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
			},
			currentState:  types.SuperNodeStateActive,
			expectedError: nil,
		},
		{
			name: "invalid validator address",
			msg: &types.MsgDeregisterSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: "invalid",
			},
			currentState:  types.SuperNodeStateActive,
			expectedError: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "validator not found",
			msg: &types.MsgDeregisterSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
			},

			expectedError: sdkerrors.ErrNotFound,
		},
		{
			name: "unauthorized",
			msg: &types.MsgDeregisterSupernode{
				Creator:          otherCreatorAddr.String(),
				ValidatorAddress: valAddr.String(),
			},
			currentState: types.SuperNodeStateActive,

			expectedError: sdkerrors.ErrUnauthorized,
		},
		{
			name: "supernode already deregistered",
			msg: &types.MsgDeregisterSupernode{
				Creator:          otherCreatorAddr.String(),
				ValidatorAddress: valAddr.String(),
			},
			currentState: types.SuperNodeStateDisabled,

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

			k, ctx := setupKeeperForTest(t, stakingKeeper, slashingKeeper, bankKeeper)
			if tc.expectedError != sdkerrors.ErrNotFound {

				k.SetSuperNode(ctx, types.SuperNode{
					SupernodeAccount: creatorAddr.String(),
					ValidatorAddress: valAddr.String(),
					Version:          "1.0.0",
					States: []*types.SuperNodeStateRecord{
						{
							State:  types.SuperNodeStateActive,
							Height: ctx.BlockHeight(),
						},

						{
							State:  tc.currentState,
							Height: ctx.BlockHeight(),
						},
					},
					PrevIpAddresses: []*types.IPAddressHistory{
						{
							Address: "1022.145.1.1",
							Height:  1,
						},
					},
					P2PPort: "26657",
				})
			}

			msgServer := keeper.NewMsgServerImpl(k)

			_, err := msgServer.DeregisterSupernode(ctx, tc.msg)
			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
