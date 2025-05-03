package keeper_test

import (
	actionapi "github.com/LumeraProtocol/lumera/api/lumera/action"
	"time"
)

func (suite *KeeperTestSuite) TestEndBlocker_ChecksExpiredActions() {
	expiredAction := &actionapi.Action{
		ActionID:       "expired-1",
		State:          actionapi.ActionState_ACTION_STATE_PENDING,
		ExpirationTime: suite.ctx.BlockTime().Add(-1 * time.Hour).Unix(), // already expired
		Creator:        suite.creatorAddress.String(),
		ActionType:     actionapi.ActionType_ACTION_TYPE_SENSE,
	}
	err := suite.keeper.SetAction(suite.ctx, expiredAction)
	suite.NoError(err)

	err = suite.keeper.EndBlocker(suite.ctx)
	suite.NoError(err)

	updatedAction, found := suite.keeper.GetActionByID(suite.ctx, expiredAction.ActionID)
	suite.True(found)
	suite.Equal(actionapi.ActionState_ACTION_STATE_EXPIRED, updatedAction.State)
}
