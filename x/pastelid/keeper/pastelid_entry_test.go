package keeper_test

import (
	"context"
	"strconv"
	"testing"

	keepertest "github.com/pastelnetwork/pasteld/testutil/keeper"
	"github.com/pastelnetwork/pasteld/testutil/nullify"
	"github.com/pastelnetwork/pasteld/x/pastelid/keeper"
	"github.com/pastelnetwork/pasteld/x/pastelid/types"
	"github.com/stretchr/testify/require"
)

// Prevent strconv unused error
var _ = strconv.IntSize

func createNPastelidEntry(keeper keeper.Keeper, ctx context.Context, n int) []types.PastelidEntry {
	items := make([]types.PastelidEntry, n)
	for i := range items {
		items[i].Address = strconv.Itoa(i)

		keeper.SetPastelidEntry(ctx, items[i])
	}
	return items
}

func TestPastelidEntryGet(t *testing.T) {
	keeper, ctx := keepertest.PastelidKeeper(t)
	items := createNPastelidEntry(keeper, ctx, 10)
	for _, item := range items {
		rst, found := keeper.GetPastelidEntry(ctx,
			item.Address,
		)
		require.True(t, found)
		require.Equal(t,
			nullify.Fill(&item),
			nullify.Fill(&rst),
		)
	}
}
func TestPastelidEntryRemove(t *testing.T) {
	keeper, ctx := keepertest.PastelidKeeper(t)
	items := createNPastelidEntry(keeper, ctx, 10)
	for _, item := range items {
		keeper.RemovePastelidEntry(ctx,
			item.Address,
		)
		_, found := keeper.GetPastelidEntry(ctx,
			item.Address,
		)
		require.False(t, found)
	}
}

func TestPastelidEntryGetAll(t *testing.T) {
	keeper, ctx := keepertest.PastelidKeeper(t)
	items := createNPastelidEntry(keeper, ctx, 10)
	require.ElementsMatch(t,
		nullify.Fill(items),
		nullify.Fill(keeper.GetAllPastelidEntry(ctx)),
	)
}
