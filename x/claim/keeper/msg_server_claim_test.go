package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/pastelnetwork/pastel/x/claim/types"
)

func TestMsgClaim(t *testing.T) {
	k, ms, ctx := setupMsgServer(t)

	// Define valid claim record with proper coin denomination
	validClaimRecord := types.ClaimRecord{
		OldAddress: "PtqHAEacynVd3V821NPhgxu9K4Ab6kAguHi",
		Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultDenom, math.NewInt(1000000))),
		Claimed:    false,
	}

	testCases := []struct {
		name      string
		msg       *types.MsgClaim
		setup     func()
		expErr    bool
		expErrMsg string
	}{
		{
			name: "valid claim",
			msg: &types.MsgClaim{
				OldAddress: "PtqHAEacynVd3V821NPhgxu9K4Ab6kAguHi",
				NewAddress: "pastel139k6camfq63u9gtc4pq8yjw4j7tmwmqeggr4p0",
				PubKey:     "0309331fc3d23ca17d91eec40ee7711efcd56facf949d46cbfa6393d43f2747e90",
				Signature:  "1f46b3a2129047a0d7a6bf91e2879e940ed3db06a2cafaaaabacc337141146f43e4932d357b435bbf2c48227f5c2f738df23a2ebc221dd11cb14ed4b83bd2a95c7",
			},
			setup: func() {
				sdkCtx := sdk.UnwrapSDKContext(ctx)
				params := types.DefaultParams()
				params.EnableClaims = true
				params.MaxClaimsPerBlock = 10
				params.ClaimEndTime = sdkCtx.BlockTime().Unix() + 3600 // 1 hour from now
				require.NoError(t, k.SetParams(sdkCtx, params))

				// Set fresh claim record
				require.NoError(t, k.SetClaimRecord(sdkCtx, validClaimRecord))

				// Set fee in context
				fee := sdk.NewCoins(sdk.NewCoin(types.DefaultDenom, math.NewInt(1000)))
				ctx = sdkCtx.WithValue(types.ClaimTxFee, fee)
			},
			expErr: false,
		},
		{
			name: "claims disabled",
			msg: &types.MsgClaim{
				OldAddress: "PtqHAEacynVd3V821NPhgxu9K4Ab6kAguHi",
				NewAddress: "pastel139k6camfq63u9gtc4pq8yjw4j7tmwmqeggr4p0",
				PubKey:     "0309331fc3d23ca17d91eec40ee7711efcd56facf949d46cbfa6393d43f2747e90",
				Signature:  "1f46b3a2129047a0d7a6bf91e2879e940ed3db06a2cafaaaabacc337141146f43e4932d357b435bbf2c48227f5c2f738df23a2ebc221dd11cb14ed4b83bd2a95c7",
			},
			setup: func() {
				sdkCtx := sdk.UnwrapSDKContext(ctx)
				params := types.DefaultParams()
				params.EnableClaims = false
				require.NoError(t, k.SetParams(sdkCtx, params))
				require.NoError(t, k.SetClaimRecord(sdkCtx, validClaimRecord))
			},
			expErr:    true,
			expErrMsg: types.ErrClaimDisabled.Error(),
		},
		{
			name: "claim period expired",
			msg: &types.MsgClaim{
				OldAddress: "PtqHAEacynVd3V821NPhgxu9K4Ab6kAguHi",
				NewAddress: "pastel139k6camfq63u9gtc4pq8yjw4j7tmwmqeggr4p0",
				PubKey:     "0309331fc3d23ca17d91eec40ee7711efcd56facf949d46cbfa6393d43f2747e90",
				Signature:  "1f46b3a2129047a0d7a6bf91e2879e940ed3db06a2cafaaaabacc337141146f43e4932d357b435bbf2c48227f5c2f738df23a2ebc221dd11cb14ed4b83bd2a95c7",
			},
			setup: func() {
				sdkCtx := sdk.UnwrapSDKContext(ctx)
				params := types.DefaultParams()
				params.EnableClaims = true
				params.ClaimEndTime = sdkCtx.BlockTime().Unix() - 3600 // 1 hour ago
				require.NoError(t, k.SetParams(sdkCtx, params))
				require.NoError(t, k.SetClaimRecord(sdkCtx, validClaimRecord))
			},
			expErr:    true,
			expErrMsg: types.ErrClaimPeriodExpired.Error(),
		},
		{
			name: "already claimed",
			msg: &types.MsgClaim{
				OldAddress: "PtqHAEacynVd3V821NPhgxu9K4Ab6kAguHi",
				NewAddress: "pastel139k6camfq63u9gtc4pq8yjw4j7tmwmqeggr4p0",
				PubKey:     "0309331fc3d23ca17d91eec40ee7711efcd56facf949d46cbfa6393d43f2747e90",
				Signature:  "1f46b3a2129047a0d7a6bf91e2879e940ed3db06a2cafaaaabacc337141146f43e4932d357b435bbf2c48227f5c2f738df23a2ebc221dd11cb14ed4b83bd2a95c7",
			},
			setup: func() {
				sdkCtx := sdk.UnwrapSDKContext(ctx)
				params := types.DefaultParams()
				params.EnableClaims = true
				require.NoError(t, k.SetParams(sdkCtx, params))

				claimedRecord := validClaimRecord
				claimedRecord.Claimed = true
				require.NoError(t, k.SetClaimRecord(sdkCtx, claimedRecord))
			},
			expErr:    true,
			expErrMsg: types.ErrClaimAlreadyClaimed.Error(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup()
			}

			resp, err := ms.Claim(ctx, tc.msg)

			if tc.expErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expErrMsg)
				require.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)

				// Check claim record was updated
				sdkCtx := sdk.UnwrapSDKContext(ctx)
				record, found, err := k.GetClaimRecord(sdkCtx, tc.msg.OldAddress)
				require.NoError(t, err)
				require.True(t, found)
				require.True(t, record.Claimed)
				require.NotZero(t, record.ClaimTime)
			}
		})
	}
}
