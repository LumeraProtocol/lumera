package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	supernodemocks "github.com/LumeraProtocol/lumera/x/supernode/v1/mocks"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

func TestHandleMetricsStaleness_PostponesWhenBalanceBelowOneLume(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	stakingKeeper := supernodemocks.NewMockStakingKeeper(ctrl)
	slashingKeeper := supernodemocks.NewMockSlashingKeeper(ctrl)
	bankKeeper := supernodemocks.NewMockBankKeeper(ctrl)

	k, ctx := setupKeeperForTest(t, stakingKeeper, slashingKeeper, bankKeeper)
	ctx = ctx.WithBlockHeight(10)

	valAddr := sdk.ValAddress([]byte("validator"))
	supernodeAccount := sdk.AccAddress([]byte("supernode1")).String()
	supernodeAccAddr, err := sdk.AccAddressFromBech32(supernodeAccount)
	require.NoError(t, err)

	sn := types.SuperNode{
		ValidatorAddress: valAddr.String(),
		SupernodeAccount: supernodeAccount,
		Note:             "1.0.0",
		PrevIpAddresses: []*types.IPAddressHistory{
			{Address: "102.145.1.1", Height: 1},
		},
		States: []*types.SuperNodeStateRecord{
			{State: types.SuperNodeStateActive, Height: 1},
		},
		P2PPort: "4445",
	}
	require.NoError(t, k.SetSuperNode(ctx, sn))

	bankKeeper.EXPECT().
		SpendableCoins(gomock.Any(), supernodeAccAddr).
		Return(sdk.NewCoins(sdk.NewInt64Coin("ulume", 999_999)))

	require.NoError(t, k.HandleMetricsStaleness(ctx))

	stored, found := k.QuerySuperNode(ctx, valAddr)
	require.True(t, found)
	require.Equal(t, types.SuperNodeStatePostponed, stored.States[len(stored.States)-1].State)
}

func TestHandleMetricsStaleness_DoesNotPostponeWhenBalanceAtLeastOneLume(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	stakingKeeper := supernodemocks.NewMockStakingKeeper(ctrl)
	slashingKeeper := supernodemocks.NewMockSlashingKeeper(ctrl)
	bankKeeper := supernodemocks.NewMockBankKeeper(ctrl)

	k, ctx := setupKeeperForTest(t, stakingKeeper, slashingKeeper, bankKeeper)
	ctx = ctx.WithBlockHeight(10)

	valAddr := sdk.ValAddress([]byte("validator"))
	supernodeAccount := sdk.AccAddress([]byte("supernode1")).String()
	supernodeAccAddr, err := sdk.AccAddressFromBech32(supernodeAccount)
	require.NoError(t, err)

	sn := types.SuperNode{
		ValidatorAddress: valAddr.String(),
		SupernodeAccount: supernodeAccount,
		Note:             "1.0.0",
		PrevIpAddresses: []*types.IPAddressHistory{
			{Address: "102.145.1.1", Height: 1},
		},
		States: []*types.SuperNodeStateRecord{
			{State: types.SuperNodeStateActive, Height: 1},
		},
		P2PPort: "4445",
	}
	require.NoError(t, k.SetSuperNode(ctx, sn))

	bankKeeper.EXPECT().
		SpendableCoins(gomock.Any(), supernodeAccAddr).
		Return(sdk.NewCoins(sdk.NewInt64Coin("ulume", 1_000_000)))

	require.NoError(t, k.HandleMetricsStaleness(ctx))

	stored, found := k.QuerySuperNode(ctx, valAddr)
	require.True(t, found)
	require.Equal(t, types.SuperNodeStateActive, stored.States[len(stored.States)-1].State)
}
