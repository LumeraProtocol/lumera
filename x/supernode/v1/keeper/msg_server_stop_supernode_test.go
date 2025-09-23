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

func TestMsgServer_StopSupernode(t *testing.T) {
	valAddr := sdk.ValAddress([]byte("validator"))
	creatorAddr := sdk.AccAddress(valAddr)

	otherValAddr := sdk.ValAddress([]byte("other-validator"))
	otherCreatorAddr := sdk.AccAddress(otherValAddr)

	existingSupernode := types2.SuperNode{
		SupernodeAccount: otherCreatorAddr.String(),
		ValidatorAddress: valAddr.String(),
		Note:             "1.0.0",
		PrevIpAddresses: []*types2.IPAddressHistory{
			{
				Address: "192.145.1.1",
			},
		},
		P2PPort: "26657",
	}

	testCases := []struct {
		name          string
		msg           *types2.MsgStopSupernode
		setupMock     func(sk *supernodemocks.MockStakingKeeper, slk *supernodemocks.MockSlashingKeeper, bk *supernodemocks.MockBankKeeper)
		setupState    func(k keeper2.Keeper, ctx sdk.Context)
		expectedError error
		checkResult   func(t *testing.T, k keeper2.Keeper, ctx sdk.Context)
	}{
		{
			name: "successful stop",
			msg: &types2.MsgStopSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				Reason:           "maintenance",
			},
			setupState: func(k keeper2.Keeper, ctx sdk.Context) {
				existingSupernode.States = []*types2.SuperNodeStateRecord{
					{
						State:  types2.SuperNodeStateActive,
						Height: 1,
					},
				}
				require.NoError(t, k.SetSuperNode(ctx, existingSupernode))

			},
			expectedError: nil,
        checkResult: func(t *testing.T, k keeper2.Keeper, ctx sdk.Context) {
            _, found := k.QuerySuperNode(ctx, valAddr)
            require.True(t, found)

            // Verify event attributes
            evs := ctx.EventManager().Events()
            foundEvt := false
            for _, e := range evs {
                if e.Type != types2.EventTypeSupernodeStopped {
                    continue
                }
                kv := map[string]string{}
                for _, a := range e.Attributes {
                    kv[string(a.Key)] = string(a.Value)
                }
                if kv[types2.AttributeKeyValidatorAddress] == valAddr.String() &&
                    kv[types2.AttributeKeyReason] == "maintenance" &&
                    kv[types2.AttributeKeyOldState] == types2.SuperNodeStateActive.String() &&
                    kv[types2.AttributeKeyHeight] != "" {
                    foundEvt = true
                    break
                }
            }
            require.True(t, foundEvt, "stop event with expected attributes not found")
        },
		},
		{
			name: "invalid validator address",
			msg: &types2.MsgStopSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: "invalid",
				Reason:           "maintenance",
			},
			expectedError: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "supernode not found",
			msg: &types2.MsgStopSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				Reason:           "node down",
			},
			expectedError: sdkerrors.ErrNotFound,
		},
		{
			name: "unauthorized",
			msg: &types2.MsgStopSupernode{
				Creator:          otherCreatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				Reason:           "other reason",
			},
			setupState: func(k keeper2.Keeper, ctx sdk.Context) {
				require.NoError(t, k.SetSuperNode(ctx, existingSupernode))
			},
			expectedError: sdkerrors.ErrUnauthorized,
		},
		{
			name: "supernode already stopped",
			msg: &types2.MsgStopSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				Reason:           "maintenance",
			},
			setupState: func(k keeper2.Keeper, ctx sdk.Context) {
				existingSupernode.States = []*types2.SuperNodeStateRecord{
					{
						State:  types2.SuperNodeStateActive,
						Height: 1,
					},
					{
						State:  types2.SuperNodeStateStopped,
						Height: 2,
					},
				}
				require.NoError(t, k.SetSuperNode(ctx, existingSupernode))

			},
			expectedError: sdkerrors.ErrInvalidRequest,
			checkResult: func(t *testing.T, k keeper2.Keeper, ctx sdk.Context) {
				_, found := k.QuerySuperNode(ctx, valAddr)
				require.True(t, found)
			},
		},
		{
			name: "supernode disabled",
			msg: &types2.MsgStopSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				Reason:           "maintenance",
			},
			setupState: func(k keeper2.Keeper, ctx sdk.Context) {
				existingSupernode.States = []*types2.SuperNodeStateRecord{
					{
						State:  types2.SuperNodeStateActive,
						Height: 1,
					},
					{
						State:  types2.SuperNodeStateDisabled,
						Height: 2,
					},
				}
				require.NoError(t, k.SetSuperNode(ctx, existingSupernode))

			},
			expectedError: sdkerrors.ErrInvalidRequest,
			checkResult: func(t *testing.T, k keeper2.Keeper, ctx sdk.Context) {
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

			msgServer := keeper2.NewMsgServerImpl(k)
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
