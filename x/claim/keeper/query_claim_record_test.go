package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	keepertest "github.com/pastelnetwork/pastel/testutil/keeper"
	"github.com/pastelnetwork/pastel/x/claim/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestClaimRecordQuery(t *testing.T) {
	keeper, ctx := keepertest.ClaimKeeper(t)

	// Define a valid claim record using real Pastel network values
	validRecord := types.ClaimRecord{
		OldAddress: "PtqHAEacynVd3V821NPhgxu9K4Ab6kAguHi",
		Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(1000000))),
		Claimed:    false,
	}

	testCases := []struct {
		name      string
		request   *types.QueryClaimRecordRequest
		setup     func()
		expErr    bool
		expCode   codes.Code
		expRecord *types.ClaimRecord
	}{
		{
			name:      "empty request",
			request:   nil,
			setup:     func() {},
			expErr:    true,
			expCode:   codes.InvalidArgument,
			expRecord: nil,
		},
		{
			name: "query non-existent record",
			request: &types.QueryClaimRecordRequest{
				Address: "PtqInvalidAddressXXXXXXXXXXXXXXXXXXXX",
			},
			setup:     func() {},
			expErr:    true,
			expCode:   codes.NotFound,
			expRecord: nil,
		},
		{
			name: "query existing record",
			request: &types.QueryClaimRecordRequest{
				Address: validRecord.OldAddress,
			},
			setup: func() {
				err := keeper.SetClaimRecord(ctx, validRecord)
				require.NoError(t, err)
			},
			expErr:    false,
			expRecord: &validRecord,
		},
		{
			name: "query claimed record",
			request: &types.QueryClaimRecordRequest{
				Address: "PtqHAEacynVd3V821NPhgxu9K4Ab6kAguHi",
			},
			setup: func() {
				claimedRecord := validRecord
				claimedRecord.Claimed = true
				err := keeper.SetClaimRecord(ctx, claimedRecord)
				require.NoError(t, err)
			},
			expErr: false,
			expRecord: &types.ClaimRecord{
				OldAddress: "PtqHAEacynVd3V821NPhgxu9K4Ab6kAguHi",
				Balance:    validRecord.Balance,
				Claimed:    true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup test state
			tc.setup()

			// Execute query
			response, err := keeper.ClaimRecord(ctx, tc.request)

			// Verify results
			if tc.expErr {
				require.Error(t, err)
				statusErr, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, tc.expCode, statusErr.Code())
				require.Nil(t, response)
			} else {
				require.NoError(t, err)
				require.NotNil(t, response)
				require.Equal(t, tc.expRecord.OldAddress, response.Record.OldAddress)
				require.Equal(t, tc.expRecord.Balance, response.Record.Balance)
				require.Equal(t, tc.expRecord.Claimed, response.Record.Claimed)
			}
		})
	}
}
