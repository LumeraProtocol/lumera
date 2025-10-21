package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	"github.com/LumeraProtocol/lumera/x/claim/keeper"	
	"github.com/LumeraProtocol/lumera/x/claim/types"
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
			k, ctx := keepertest.ClaimKeeper(t, "")

			// Setup
			if tc.setupRecord != nil {
				err := k.SetClaimRecord(ctx, *tc.setupRecord)
				require.NoError(t, err)
				tc.expectRecord = *tc.setupRecord
			}

			// Test
			record, found, err := k.GetClaimRecord(ctx, tc.queryAddress)

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
			k, ctx := keepertest.ClaimKeeper(t, "")

			// Setup
			if tc.initialRecord != nil {
				err := k.SetClaimRecord(ctx, *tc.initialRecord)
				require.NoError(t, err)
			}

			// Test
			err := k.SetClaimRecord(ctx, tc.recordToSet)
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Verify
			record, found, err := k.GetClaimRecord(ctx, tc.recordToSet.OldAddress)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, tc.recordToSet, record)
		})
	}
}

func TestListClaimed(t *testing.T) {
	testCases := []struct {
		name           string
		setupRecords   []types.ClaimRecord
		vestedTerm     uint32
		expectClaimed  int
		expectErr      bool
	}{
		{
			name:           "empty state",
			setupRecords:   nil,
			vestedTerm:     1,
			expectClaimed:  0,
			expectErr:      false,
		},
		{
			name: "no claimed records",
			setupRecords: []types.ClaimRecord{
				{
					OldAddress: "address1",
					Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(100))),
					Claimed:    false,
					VestedTier: 1,
				},
				{
					OldAddress: "address2",
					Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(200))),
					Claimed:    false,
					VestedTier: 2,
				},
			},
			vestedTerm:     1,
			expectClaimed:  0,
			expectErr:      false,
		},
		{
			name: "claimed records with different vested tiers",
			setupRecords: []types.ClaimRecord{
				{
					OldAddress: "address1",
					Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(100))),
					Claimed:    true,
					VestedTier: 1,
				},
				{
					OldAddress: "address2",
					Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(200))),
					Claimed:    true,
					VestedTier: 2,
				},
				{
					OldAddress: "address3",
					Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(300))),
					Claimed:    true,
					VestedTier: 1,
				},
			},
			vestedTerm:     1,
			expectClaimed:  2,
			expectErr:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx := keepertest.ClaimKeeper(t, "")
			q := keeper.NewQueryServerImpl(k)

			// Setup
			if tc.setupRecords != nil {
				for _, record := range tc.setupRecords {
					err := k.SetClaimRecord(ctx, record)
					require.NoError(t, err)
				}
			}

			// Test
			req := &types.QueryListClaimedRequest{
				VestedTerm: tc.vestedTerm,
			}
			resp, err := q.ListClaimed(ctx, req)

			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, resp)
			require.Equal(t, tc.expectClaimed, len(resp.Claims))

			// Verify all returned records are claimed and have the correct vested tier
			for _, claim := range resp.Claims {
				require.True(t, claim.Claimed)
				require.Equal(t, tc.vestedTerm, claim.VestedTier)
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
			k, ctx := keepertest.ClaimKeeper(t, "")

			// Setup
			if tc.setupRecords != nil {
				for _, record := range tc.setupRecords {
					err := k.SetClaimRecord(ctx, record)
					require.NoError(t, err)
				}
			}

			// Test
			count := k.GetClaimRecordCount(ctx)
			require.Equal(t, tc.expectCount, count)
		})
	}
}
