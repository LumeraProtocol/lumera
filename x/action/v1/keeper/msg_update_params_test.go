package keeper_test

import (
	types2 "github.com/LumeraProtocol/lumera/x/action/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (suite *MsgServerTestSuite) TestMsgUpdateParams() {
	params := types2.DefaultParams()
	suite.NoError(suite.keeper.SetParams(suite.ctx, params))
	wctx := sdk.UnwrapSDKContext(suite.ctx)

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
			name: "send enabled param",
			input: &types2.MsgUpdateParams{
				Authority: suite.keeper.GetAuthority(),
				Params:    types2.Params{},
			},
			expErr: false,
		},
		{
			name: "all good",
			input: &types2.MsgUpdateParams{
				Authority: suite.keeper.GetAuthority(),
				Params:    params,
			},
			expErr: false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			_, err := suite.msgServer.UpdateParams(wctx, tc.input)

			if tc.expErr {
				suite.Error(err)
				suite.Contains(err.Error(), tc.expErrMsg)
			} else {
				suite.NoError(err)
			}
		})
	}
}
