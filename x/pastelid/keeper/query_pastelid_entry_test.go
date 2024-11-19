package keeper_test

import (
	"strconv"
	"testing"

	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	keepertest "github.com/pastelnetwork/pastel/testutil/keeper"
	"github.com/pastelnetwork/pastel/testutil/nullify"
	"github.com/pastelnetwork/pastel/x/pastelid/types"
)

// Prevent strconv unused error
var _ = strconv.IntSize

func TestPastelidEntryQuerySingle(t *testing.T) {
	keeper, ctx := keepertest.PastelidKeeper(t)
	msgs := createNPastelidEntry(keeper, ctx, 2)
	tests := []struct {
		desc     string
		request  *types.QueryGetPastelidEntryRequest
		response *types.QueryGetPastelidEntryResponse
		err      error
	}{
		{
			desc: "First",
			request: &types.QueryGetPastelidEntryRequest{
				Address: msgs[0].Address,
			},
			response: &types.QueryGetPastelidEntryResponse{PastelidEntry: msgs[0]},
		},
		{
			desc: "Second",
			request: &types.QueryGetPastelidEntryRequest{
				Address: msgs[1].Address,
			},
			response: &types.QueryGetPastelidEntryResponse{PastelidEntry: msgs[1]},
		},
		{
			desc: "KeyNotFound",
			request: &types.QueryGetPastelidEntryRequest{
				Address: strconv.Itoa(100000),
			},
			err: status.Error(codes.NotFound, "not found"),
		},
		{
			desc: "InvalidRequest",
			err:  status.Error(codes.InvalidArgument, "invalid request"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			response, err := keeper.PastelidEntry(ctx, tc.request)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			} else {
				require.NoError(t, err)
				require.Equal(t,
					nullify.Fill(tc.response),
					nullify.Fill(response),
				)
			}
		})
	}
}

func TestPastelidEntryQueryPaginated(t *testing.T) {
	keeper, ctx := keepertest.PastelidKeeper(t)
	msgs := createNPastelidEntry(keeper, ctx, 5)

	request := func(next []byte, offset, limit uint64, total bool) *types.QueryAllPastelidEntryRequest {
		return &types.QueryAllPastelidEntryRequest{
			Pagination: &query.PageRequest{
				Key:        next,
				Offset:     offset,
				Limit:      limit,
				CountTotal: total,
			},
		}
	}
	t.Run("ByOffset", func(t *testing.T) {
		step := 2
		for i := 0; i < len(msgs); i += step {
			resp, err := keeper.PastelidEntryAll(ctx, request(nil, uint64(i), uint64(step), false))
			require.NoError(t, err)
			require.LessOrEqual(t, len(resp.PastelidEntry), step)
			require.Subset(t,
				nullify.Fill(msgs),
				nullify.Fill(resp.PastelidEntry),
			)
		}
	})
	t.Run("ByKey", func(t *testing.T) {
		step := 2
		var next []byte
		for i := 0; i < len(msgs); i += step {
			resp, err := keeper.PastelidEntryAll(ctx, request(next, 0, uint64(step), false))
			require.NoError(t, err)
			require.LessOrEqual(t, len(resp.PastelidEntry), step)
			require.Subset(t,
				nullify.Fill(msgs),
				nullify.Fill(resp.PastelidEntry),
			)
			next = resp.Pagination.NextKey
		}
	})
	t.Run("Total", func(t *testing.T) {
		resp, err := keeper.PastelidEntryAll(ctx, request(nil, 0, 0, true))
		require.NoError(t, err)
		require.Equal(t, len(msgs), int(resp.Pagination.Total))
		require.ElementsMatch(t,
			nullify.Fill(msgs),
			nullify.Fill(resp.PastelidEntry),
		)
	})
	t.Run("InvalidRequest", func(t *testing.T) {
		_, err := keeper.PastelidEntryAll(ctx, nil)
		require.ErrorIs(t, err, status.Error(codes.InvalidArgument, "invalid request"))
	})
}
