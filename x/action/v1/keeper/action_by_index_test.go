package keeper_test

import (
	"testing"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	"github.com/LumeraProtocol/lumera/testutil/sample"
	"github.com/LumeraProtocol/lumera/x/action/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func actionIDSet(actions []*types.Action) map[string]struct{} {
	set := make(map[string]struct{}, len(actions))
	for _, a := range actions {
		set[a.ActionID] = struct{}{}
	}
	return set
}

func TestKeeper_GetActionsByCreator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	k, ctx := keepertest.ActionKeeper(t, ctrl)

	creatorA := sample.AccAddress()
	creatorB := sample.AccAddress()
	snX := sample.AccAddress()
	price := sdk.NewInt64Coin("ulume", 100).String()

	// creatorA owns actions 1 and 2; creatorB owns action 3.
	require.NoError(t, k.SetAction(ctx, &types.Action{Creator: creatorA, ActionID: "1", ActionType: types.ActionTypeCascade, Price: price, State: types.ActionStatePending, BlockHeight: 10, SuperNodes: []string{snX}}))
	require.NoError(t, k.SetAction(ctx, &types.Action{Creator: creatorA, ActionID: "2", ActionType: types.ActionTypeSense, Price: price, State: types.ActionStatePending, BlockHeight: 11, SuperNodes: []string{snX}}))
	require.NoError(t, k.SetAction(ctx, &types.Action{Creator: creatorB, ActionID: "3", ActionType: types.ActionTypeCascade, Price: price, State: types.ActionStatePending, BlockHeight: 12, SuperNodes: []string{snX}}))

	got, err := k.GetActionsByCreator(ctx, creatorA)
	require.NoError(t, err)
	require.Len(t, got, 2)
	ids := actionIDSet(got)
	require.Contains(t, ids, "1")
	require.Contains(t, ids, "2")
	require.NotContains(t, ids, "3")

	// Address that created nothing yields an empty (non-error) result.
	empty, err := k.GetActionsByCreator(ctx, sample.AccAddress())
	require.NoError(t, err)
	require.Len(t, empty, 0)
}

func TestKeeper_GetActionsBySuperNode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	k, ctx := keepertest.ActionKeeper(t, ctrl)

	creator := sample.AccAddress()
	snX := sample.AccAddress()
	snY := sample.AccAddress()
	price := sdk.NewInt64Coin("ulume", 100).String()

	// snX serves actions 1 and 3; snY serves action 2.
	require.NoError(t, k.SetAction(ctx, &types.Action{Creator: creator, ActionID: "1", ActionType: types.ActionTypeCascade, Price: price, State: types.ActionStatePending, BlockHeight: 10, SuperNodes: []string{snX}}))
	require.NoError(t, k.SetAction(ctx, &types.Action{Creator: creator, ActionID: "2", ActionType: types.ActionTypeSense, Price: price, State: types.ActionStatePending, BlockHeight: 11, SuperNodes: []string{snY}}))
	require.NoError(t, k.SetAction(ctx, &types.Action{Creator: creator, ActionID: "3", ActionType: types.ActionTypeCascade, Price: price, State: types.ActionStatePending, BlockHeight: 12, SuperNodes: []string{snX, snY}}))

	got, err := k.GetActionsBySuperNode(ctx, snX)
	require.NoError(t, err)
	require.Len(t, got, 2)
	ids := actionIDSet(got)
	require.Contains(t, ids, "1")
	require.Contains(t, ids, "3")
	require.NotContains(t, ids, "2")

	// Address serving nothing yields an empty (non-error) result.
	empty, err := k.GetActionsBySuperNode(ctx, sample.AccAddress())
	require.NoError(t, err)
	require.Len(t, empty, 0)
}
