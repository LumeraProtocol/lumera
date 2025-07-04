package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/claim/types"
)

func TestMsgDelayedClaim(t *testing.T) {
	k, ms, ctx := setupMsgServer(t)

	// Define valid claim record with proper coin denomination
	validClaimRecord := types.ClaimRecord{
		OldAddress: "Ptko7ZkiQXyT9Db45GtsewpnhnRRwpHkHBc",
		Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(1000000))),
		Claimed:    false,
	}

	testCases := []struct {
		name      string
		msg       *types.MsgDelayedClaim
		setup     func()
		expErr    bool
		expErrMsg string
	}{
		{
			name: "valid claim",
			msg: &types.MsgDelayedClaim{
				OldAddress: "Ptko7ZkiQXyT9Db45GtsewpnhnRRwpHkHBc",
				NewAddress: "lumera1zvnc27832srgxa207y5hu2agy83wazfzurufyp",
				PubKey:     "038685010ec7ce724c1f83ba333564135feadf70eade12036546f20b95ce276a12",
				Signature:  "1f4926307e7d94f5290058f8836429963431b3f7fc091567f1621a510d25cbcb71240f9c4488d9cafa16c8c3457d07c263afcfe4c77e0081a4e75bb618e99e1cd3",
				Tier:       1,
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
				fee := sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(1000)))
				ctx = sdkCtx.WithValue(types.ClaimTxFee, fee)
			},
			expErr: false,
		},
		{
			name: "claims disabled",
			msg: &types.MsgDelayedClaim{
				OldAddress: "Ptko7ZkiQXyT9Db45GtsewpnhnRRwpHkHBc",
				NewAddress: "lumera1zvnc27832srgxa207y5hu2agy83wazfzurufyp",
				PubKey:     "038685010ec7ce724c1f83ba333564135feadf70eade12036546f20b95ce276a12",
				Signature:  "1f4926307e7d94f5290058f8836429963431b3f7fc091567f1621a510d25cbcb71240f9c4488d9cafa16c8c3457d07c263afcfe4c77e0081a4e75bb618e99e1cd3",
				Tier:       1,
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
			msg: &types.MsgDelayedClaim{
				OldAddress: "Ptko7ZkiQXyT9Db45GtsewpnhnRRwpHkHBc",
				NewAddress: "lumera1zvnc27832srgxa207y5hu2agy83wazfzurufyp",
				PubKey:     "038685010ec7ce724c1f83ba333564135feadf70eade12036546f20b95ce276a12",
				Signature:  "1f4926307e7d94f5290058f8836429963431b3f7fc091567f1621a510d25cbcb71240f9c4488d9cafa16c8c3457d07c263afcfe4c77e0081a4e75bb618e99e1cd3",
				Tier:       1,
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
			msg: &types.MsgDelayedClaim{
				OldAddress: "Ptko7ZkiQXyT9Db45GtsewpnhnRRwpHkHBc",
				NewAddress: "lumera1zvnc27832srgxa207y5hu2agy83wazfzurufyp",
				PubKey:     "038685010ec7ce724c1f83ba333564135feadf70eade12036546f20b95ce276a12",
				Signature:  "1f4926307e7d94f5290058f8836429963431b3f7fc091567f1621a510d25cbcb71240f9c4488d9cafa16c8c3457d07c263afcfe4c77e0081a4e75bb618e99e1cd3",
				Tier:       1,
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

			resp, err := ms.DelayedClaim(ctx, tc.msg)

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
