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
	// Only 2 of the below cases reach keeper.FinalizeAction (and thus query top supernodes).
	suite.setupExpectationsGetAllTopSNs(2)
	actionID := suite.registerCascadeAction()

	testCases := []struct {
		name          string
		actionId      string
		actionType    string
		superNode     string
		badMetadata   string
		badIDsOti     bool
		badIDs        bool
		expectErr     bool
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
			expectErr:     true,
			errorContains: "not found",
		},
		{
			name:          "Wrong supernode (rejected, evidence, no error)",
			actionId:      actionID,
			actionType:    "CASCADE",
			superNode:     suite.badSupernode.SupernodeAccount,
			badMetadata:   "",
			badIDsOti:     false,
			badIDs:        false,
			expectErr:     false,
			errorContains: "",
		},
		{
			name:          "Invalid rq_ids_ids values (rejected, evidence, no error)",
			actionId:      actionID,
			actionType:    "CASCADE",
			superNode:     suite.supernodes[0].SupernodeAccount,
			badMetadata:   "",
			badIDsOti:     true,
			badIDs:        false,
			expectErr:     false,
			errorContains: "",
		},
		{
			name:          "Invalid metadata JSON",
			actionId:      actionID,
			actionType:    "CASCADE",
			superNode:     suite.supernodes[0].SupernodeAccount,
			badMetadata:   "{invalid_json",
			badIDsOti:     false,
			badIDs:        false,
			expectErr:     true,
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
			expectErr:     true,
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
			expectErr:     true,
			errorContains: "invalid metadata",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.ctx = suite.ctx.WithEventManager(sdk.NewEventManager())
			msg := suite.makeFinalizeCascadeActionMessage(tc.actionId, tc.actionType, tc.superNode, tc.badMetadata, tc.badIDsOti, tc.badIDs)
			_, err := suite.msgServer.FinalizeAction(suite.ctx, &msg)

			if tc.expectErr {
				suite.Error(err)
				suite.Contains(err.Error(), tc.errorContains)
				return
			}

			suite.NoError(err)

			action, found := suite.keeper.GetActionByID(suite.ctx, tc.actionId)
			suite.True(found)
			suite.Equal(actiontypes.ActionStatePending, action.State)

			events := suite.ctx.EventManager().Events()
			foundRejected := false
			for _, event := range events {
				if event.Type == types.EventTypeActionFinalizationRejected {
					foundRejected = true
					break
				}
			}
			suite.True(foundRejected, "action_finalization_rejected event not found")
		})
	}
}
