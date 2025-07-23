package keeper_test

import (
	"strings"

	"github.com/LumeraProtocol/lumera/x/action/v1/types"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (suite *MsgServerTestSuite) TestMsgFinalizeActionCascade() {
	suite.ctx = suite.ctx.WithEventManager(sdk.NewEventManager())

	suite.setupExpectationsGetAllTopSNs(1)

	actionID := suite.registerCascadeAction()
	suite.finalizeCascadeAction(actionID)

	// Verify events were emitted
	events := suite.ctx.EventManager().Events()
	foundFinalizeEvent := false
	for _, event := range events {
		if event.Type == types.EventTypeActionFinalized {
			foundFinalizeEvent = true
			break
		}
	}
	suite.True(foundFinalizeEvent, "action_finalized event not found")
}

func (suite *MsgServerTestSuite) TestMsgFinalizeActionSense() {
	suite.ctx = suite.ctx.WithEventManager(sdk.NewEventManager())

	suite.setupExpectationsGetAllTopSNs(1)

	actionID := suite.registerSenseAction()
	suite.finalizeSenseAction(actionID, suite.supernodes[0].SupernodeAccount, actiontypes.ActionStateDone)

	// Verify events were emitted
	events := suite.ctx.EventManager().Events()
	foundFinalizeEvent := false
	for _, event := range events {
		if event.Type == types.EventTypeActionFinalized {
			foundFinalizeEvent = true
			break
		}
		// Check attributes
		for _, attr := range event.Attributes {
			if string(attr.Key) == types.AttributeKeySuperNodes {
				superNodes := strings.Split(attr.Value, ",")
				suite.Equal(1, len(superNodes))
				suite.Equal(suite.supernodes[0].SupernodeAccount, superNodes[0])
			}
		}
	}

	suite.True(foundFinalizeEvent, "action_finalized event not found")
}

func (suite *MsgServerTestSuite) TestMsgFinalizeActionCascadeErrors() {
	suite.setupExpectationsGetAllTopSNs(1)
	actionID := suite.registerCascadeAction()

	testCases := []struct {
		name          string
		actionId      string
		actionType    string
		superNode     string
		badMetadata   string
		badIDsOti     bool
		badIDs        bool
		errorContains string
	}{
		{
			name:          "Non-existent action ID",
			actionId:      "non_existent_id",
			actionType:    "CASCADE",
			superNode:     suite.supernodes[0].SupernodeAccount,
			badMetadata:   "",
			badIDsOti:     false,
			badIDs:        false,
			errorContains: "not found",
		},
		{
			name:          "Wrong supernode",
			actionId:      actionID,
			actionType:    "CASCADE",
			superNode:     suite.badSupernode.SupernodeAccount,
			badMetadata:   "",
			badIDsOti:     false,
			badIDs:        false,
			errorContains: "unauthorized supernode",
		},
		{
			name:          "Invalid metadata JSON",
			actionId:      actionID,
			actionType:    "CASCADE",
			superNode:     suite.supernodes[0].SupernodeAccount,
			badMetadata:   "{invalid_json",
			badIDsOti:     false,
			badIDs:        false,
			errorContains: "invalid metadata",
		},
		{
			name:          "Missing required metadata fields - IDs",
			actionId:      actionID,
			actionType:    "CASCADE",
			superNode:     suite.supernodes[0].SupernodeAccount,
			badMetadata:   "",
			badIDsOti:     false,
			badIDs:        true,
			errorContains: "invalid metadata",
		},
		{
			name:          "Wrong Action Type",
			actionId:      actionID,
			actionType:    "SENSE",
			superNode:     suite.supernodes[0].SupernodeAccount,
			badMetadata:   "",
			badIDsOti:     false,
			badIDs:        false,
			errorContains: "invalid metadata",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {		
			msg := suite.makeFinalizeCascadeActionMessage(tc.actionId, tc.actionType, tc.superNode, tc.badMetadata, tc.badIDsOti, tc.badIDs)
			_, err := suite.msgServer.FinalizeAction(suite.ctx, &msg)
			suite.Error(err)
			suite.Contains(err.Error(), tc.errorContains)
		})
	}
}
