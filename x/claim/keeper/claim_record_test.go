package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	keepertest "github.com/pastelnetwork/pastel/testutil/keeper"
	"github.com/pastelnetwork/pastel/x/claim/types"
	"github.com/stretchr/testify/require"
)

func TestGetClaimRecord(t *testing.T) {
	testCases := []struct {
		name         string
		setupRecord  *types.ClaimRecord
		queryAddress string
		expectFound  bool
		expectErr    bool
		expectRecord types.ClaimRecord
	}{
		{
			name:         "non-existent record",
			setupRecord:  nil,
			queryAddress: "test_address",
			expectFound:  false,
			expectErr:    false,
			expectRecord: types.ClaimRecord{},
		},
		{
			name: "existing record",
			setupRecord: &types.ClaimRecord{
				OldAddress: "test_address",
				Balance:    sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, math.NewInt(100))),
				Claimed:    false,
			},
			queryAddress: "test_address",
			expectFound:  true,
			expectErr:    false,
		},
		{
			name: "query wrong address",
			setupRecord: &types.ClaimRecord{
				OldAddress: "test_address",
				Balance:    sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, math.NewInt(100))),
				Claimed:    false,
			},
			queryAddress: "wrong_address",
			expectFound:  false,
			expectErr:    false,
			expectRecord: types.ClaimRecord{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			keeper, ctx := keepertest.ClaimKeeper(t)

			// Setup
			if tc.setupRecord != nil {
				err := keeper.SetClaimRecord(ctx, *tc.setupRecord)
				require.NoError(t, err)
				tc.expectRecord = *tc.setupRecord
			}

			// Test
			record, found, err := keeper.GetClaimRecord(ctx, tc.queryAddress)

			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expectFound, found)
			if tc.expectFound {
				require.Equal(t, tc.expectRecord, record)
			}
		})
	}
}

func TestSetClaimRecord(t *testing.T) {
	testCases := []struct {
		name          string
		initialRecord *types.ClaimRecord
		recordToSet   types.ClaimRecord
		expectErr     bool
	}{
		{
			name:          "set new record",
			initialRecord: nil,
			recordToSet: types.ClaimRecord{
				OldAddress: "test_address",
				Balance:    sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, math.NewInt(100))),
				Claimed:    false,
			},
			expectErr: false,
		},
		{
			name: "update existing record",
			initialRecord: &types.ClaimRecord{
				OldAddress: "test_address",
				Balance:    sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, math.NewInt(100))),
				Claimed:    false,
			},
			recordToSet: types.ClaimRecord{
				OldAddress: "test_address",
				Balance:    sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, math.NewInt(100))),
				Claimed:    true,
			},
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			keeper, ctx := keepertest.ClaimKeeper(t)

			// Setup
			if tc.initialRecord != nil {
				err := keeper.SetClaimRecord(ctx, *tc.initialRecord)
				require.NoError(t, err)
			}

			// Test
			err := keeper.SetClaimRecord(ctx, tc.recordToSet)
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Verify
			record, found, err := keeper.GetClaimRecord(ctx, tc.recordToSet.OldAddress)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, tc.recordToSet, record)
		})
	}
}

func TestListClaimRecords(t *testing.T) {
	testCases := []struct {
		name         string
		setupRecords []types.ClaimRecord
		expectErr    bool
		expectCount  int
	}{
		{
			name:         "empty list",
			setupRecords: nil,
			expectErr:    false,
			expectCount:  0,
		},
		{
			name: "multiple records",
			setupRecords: []types.ClaimRecord{
				{
					OldAddress: "address1",
					Balance:    sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, math.NewInt(100))),
					Claimed:    false,
				},
				{
					OldAddress: "address2",
					Balance:    sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, math.NewInt(200))),
					Claimed:    true,
				},
			},
			expectErr:   false,
			expectCount: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			keeper, ctx := keepertest.ClaimKeeper(t)

			// Setup
			if tc.setupRecords != nil {
				for _, record := range tc.setupRecords {
					err := keeper.SetClaimRecord(ctx, record)
					require.NoError(t, err)
				}
			}

			// Test
			records, err := keeper.ListClaimRecords(ctx)
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, records, tc.expectCount)

			if tc.setupRecords != nil {
				require.ElementsMatch(t, tc.setupRecords, records)
			}
		})
	}
}

func TestGetClaimRecordCount(t *testing.T) {
	testCases := []struct {
		name         string
		setupRecords []types.ClaimRecord
		expectCount  uint64
	}{
		{
			name:         "initial empty state",
			setupRecords: nil,
			expectCount:  0,
		},
		{
			name: "multiple records",
			setupRecords: []types.ClaimRecord{
				{
					OldAddress: "address1",
					Balance:    sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, math.NewInt(100))),
					Claimed:    false,
				},
				{
					OldAddress: "address2",
					Balance:    sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, math.NewInt(200))),
					Claimed:    false,
				},
			},
			expectCount: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			keeper, ctx := keepertest.ClaimKeeper(t)

			// Setup
			if tc.setupRecords != nil {
				for _, record := range tc.setupRecords {
					err := keeper.SetClaimRecord(ctx, record)
					require.NoError(t, err)
				}
			}

			// Test
			count := keeper.GetClaimRecordCount(ctx)
			require.Equal(t, tc.expectCount, count)
		})
	}
}
