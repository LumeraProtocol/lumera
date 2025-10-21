package keeper_test

import (
	"github.com/LumeraProtocol/lumera/x/action/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (suite *MsgServerTestSuite) TestMsgUpdateParams() {
	params := types.DefaultParams()
	suite.NoError(suite.keeper.SetParams(suite.ctx, params))
	wctx := sdk.UnwrapSDKContext(suite.ctx)
	authority, err := suite.keeper.GetAddressCodec().BytesToString(suite.keeper.GetAuthority())
	suite.NoError(err)

	// default params
	testCases := []struct {
		name      string
		input     *types.MsgUpdateParams
		expErr    bool
		expErrMsg string
	}{
		{
			name: "invalid authority",
			input: &types.MsgUpdateParams{
				Authority: "invalid",
				Params:    params,
			},
			expErr:    true,
			expErrMsg: "invalid authority",
		},
		{
			name: "send enabled param",
			input: &types.MsgUpdateParams{
				Authority: authority,
				Params:    types.Params{},
			},
			expErr: false,
		},
		{
			name: "all good",
			input: &types.MsgUpdateParams{
				Authority: authority,
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
