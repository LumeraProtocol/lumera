package keeper_test

import (
	actionapi "github.com/LumeraProtocol/lumera/api/lumera/action"
	"github.com/LumeraProtocol/lumera/x/action/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (suite *MsgServerTestSuite) TestMsgApproveActionCascade() {
	suite.ctx = suite.ctx.WithEventManager(sdk.NewEventManager())
	actionID := suite.registerCascadeAction()
	suite.finalizeCascadeAction(actionID)
	suite.approveAction(actionID, suite.creatorAddress.String())

	// Verify action state has changed to APPROVED
	updatedAction, found := suite.keeper.GetActionByID(suite.ctx, actionID)
	suite.True(found)
	suite.Equal(actionapi.ActionState_ACTION_STATE_APPROVED, updatedAction.State)

	// Verify events were emitted
	events := suite.ctx.EventManager().Events()
	foundApproveEvent := false

	for _, event := range events {
		if event.Type == types.EventTypeActionApproved {
			foundApproveEvent = true
		}
	}
	suite.True(foundApproveEvent, "action_approved event not found")
}

func (suite *MsgServerTestSuite) TestMsgApproveActionErrors() {
	suite.ctx = suite.ctx.WithEventManager(sdk.NewEventManager())

	actionIDApproved := suite.registerCascadeAction()
	suite.finalizeCascadeAction(actionIDApproved)
	suite.approveAction(actionIDApproved, suite.creatorAddress.String())

	actionIDDone := suite.registerCascadeAction()
	suite.finalizeCascadeAction(actionIDDone)

	actionIDPending := suite.registerCascadeAction()

	actionIDProcessing := suite.registerSenseAction()
	suite.finalizeSenseAction(actionIDProcessing, suite.supernodes[0].SupernodeAccount, actionapi.ActionState_ACTION_STATE_PROCESSING)

	testCases := []struct {
		name          string
		creator       string
		actionId      string
		signature     string
		errorContains string
	}{
		{
			name:          "Non-existent action ID",
			creator:       suite.creatorAddress.String(),
			actionId:      "non_existent_id",
			errorContains: "not found",
		},
		{
			name:          "Different creator than action",
			creator:       suite.imposterAddress.String(),
			actionId:      actionIDDone,
			errorContains: "only the creator",
		},
		{
			name:          "Action not in DONE state - Pending",
			creator:       suite.creatorAddress.String(),
			actionId:      actionIDPending,
			errorContains: "cannot be approved",
		},
		{
			name:          "Action not in DONE state - Processing",
			creator:       suite.creatorAddress.String(),
			actionId:      actionIDProcessing,
			errorContains: "cannot be approved",
		},
		{
			name:          "Already approved action",
			creator:       suite.creatorAddress.String(),
			actionId:      actionIDApproved,
			errorContains: "cannot be approved",
		},
		{
			name:          "Valid approval",
			creator:       suite.creatorAddress.String(),
			actionId:      actionIDDone,
			errorContains: "", // No error expected
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			res, err := suite.approveActionNoCheck(tc.actionId, tc.creator)

			if tc.errorContains != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tc.errorContains)
				suite.Nil(res)
			} else {
				suite.NoError(err)
				suite.NotNil(res)
			}
		})
	}
}
