package keeper_test

import (
	"time"

	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
)

func (suite *KeeperTestSuite) TestEndBlocker_ChecksExpiredActions() {
	expiredAction := &actiontypes.Action{
		ActionID:       "expired-1",
		State:          actiontypes.ActionStatePending,
		ExpirationTime: suite.ctx.BlockTime().Add(-1 * time.Hour).Unix(), // already expired
		Creator:        suite.creatorAddress.String(),
		ActionType:     actiontypes.ActionTypeSense,
	}
	err := suite.keeper.SetAction(suite.ctx, expiredAction)
	suite.NoError(err)

	err = suite.keeper.EndBlocker(suite.ctx)
	suite.NoError(err)

	updatedAction, found := suite.keeper.GetActionByID(suite.ctx, expiredAction.ActionID)
	suite.True(found)
	suite.Equal(actiontypes.ActionStateExpired, updatedAction.State)
}
