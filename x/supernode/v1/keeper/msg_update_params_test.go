package keeper_test

import (
	types2 "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestMsgUpdateParams(t *testing.T) {
	k, ms, ctx := setupMsgServer(t)
	params := types2.DefaultParams()
	wctx := sdk.UnwrapSDKContext(ctx)

	// Use wctx instead of sdk.Context{}
	require.NoError(t, k.SetParams(wctx, params))

	// default params
	testCases := []struct {
		name      string
		input     *types2.MsgUpdateParams
		expErr    bool
		expErrMsg string
	}{
		{
			name: "invalid authority",
			input: &types2.MsgUpdateParams{
				Authority: "invalid",
				Params:    params,
			},
			expErr:    true,
			expErrMsg: "invalid authority",
		},
		{
			name: "empty params but valid authority",
			input: &types2.MsgUpdateParams{
				Authority: k.GetAuthority(),
				Params:    types2.Params{},
			},
			expErr: false,
		},
		{
			name: "all good",
			input: &types2.MsgUpdateParams{
				Authority: k.GetAuthority(),
				Params:    params,
			},
			expErr: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := ms.UpdateParams(ctx, tc.input)

			if tc.expErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expErrMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
