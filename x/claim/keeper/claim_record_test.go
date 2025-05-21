package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	"github.com/LumeraProtocol/lumera/x/claim/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
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
				Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(100))),
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
				Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(100))),
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
				Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(100))),
				Claimed:    false,
			},
			expectErr: false,
		},
		{
			name: "update existing record",
			initialRecord: &types.ClaimRecord{
				OldAddress: "test_address",
				Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(100))),
				Claimed:    false,
			},
			recordToSet: types.ClaimRecord{
				OldAddress: "test_address",
				Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(100))),
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
					Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(100))),
					Claimed:    false,
				},
				{
					OldAddress: "address2",
					Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(200))),
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

			// Test without filter (should return all records)
			records, err := keeper.ListClaimRecords(ctx, nil)
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

func TestListClaimRecordsWithFilter(t *testing.T) {
	testCases := []struct {
		name         string
		setupRecords []types.ClaimRecord
		filter       func(*types.ClaimRecord) bool
		expectErr    bool
		expectCount  int
	}{
		{
			name: "filter by claimed = true",
			setupRecords: []types.ClaimRecord{
				{
					OldAddress: "address1",
					Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(100))),
					Claimed:    true,
				},
				{
					OldAddress: "address2",
					Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(200))),
					Claimed:    false,
				},
				{
					OldAddress: "address3",
					Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(300))),
					Claimed:    true,
				},
			},
			filter: func(record *types.ClaimRecord) bool {
				return record.Claimed
			},
			expectErr:   false,
			expectCount: 2, // Only the records with Claimed = true
		},
		{
			name: "filter by claimed = false",
			setupRecords: []types.ClaimRecord{
				{
					OldAddress: "address1",
					Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(100))),
					Claimed:    true,
				},
				{
					OldAddress: "address2",
					Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(200))),
					Claimed:    false,
				},
				{
					OldAddress: "address3",
					Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(300))),
					Claimed:    true,
				},
			},
			filter: func(record *types.ClaimRecord) bool {
				return !record.Claimed
			},
			expectErr:   false,
			expectCount: 1, // Only the record with Claimed = false
		},
		{
			name: "complex filter with multiple conditions",
			setupRecords: []types.ClaimRecord{
				{
					OldAddress: "address1",
					Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(100))),
					Claimed:    true,
					ClaimTime:  1000,
				},
				{
					OldAddress: "address2",
					Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(200))),
					Claimed:    true,
					ClaimTime:  2000,
				},
				{
					OldAddress: "address3",
					Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(300))),
					Claimed:    false,
					ClaimTime:  0,
				},
			},
			filter: func(record *types.ClaimRecord) bool {
				return record.Claimed && record.ClaimTime > 1500
			},
			expectErr:   false,
			expectCount: 1, // Only the record with Claimed = true and ClaimTime > 1500
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

			// Test with filter
			records, err := keeper.ListClaimRecords(ctx, tc.filter)
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, records, tc.expectCount)

			// Verify that all returned records satisfy the filter
			for _, record := range records {
				require.True(t, tc.filter(record))
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
					Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(100))),
					Claimed:    false,
				},
				{
					OldAddress: "address2",
					Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(200))),
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
