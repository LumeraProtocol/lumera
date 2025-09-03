package keeper_test

import (
	"testing"

	keeper2 "github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	types2 "github.com/LumeraProtocol/lumera/x/action/v1/types"

	"github.com/stretchr/testify/suite"

	"google.golang.org/protobuf/encoding/protojson"

	actionapi "github.com/LumeraProtocol/lumera/api/lumera/action"
	sdk "github.com/cosmos/cosmos-sdk/types"
	v1beta1 "cosmossdk.io/api/cosmos/base/v1beta1"
)

type MsgServerTestSuite struct {
	suite.Suite
	KeeperTestSuiteConfig
	msgServer types2.MsgServer
}

func TestMsgServerTestSuite(t *testing.T) {
	suite.Run(t, new(MsgServerTestSuite))
}

func (suite *MsgServerTestSuite) SetupTest() {
	suite.SetupTestSuite(&suite.Suite)
	suite.msgServer = keeper2.NewMsgServerImpl(suite.keeper)
}

func (suite *MsgServerTestSuite) TestMsgServer() {
	suite.NotNil(suite.msgServer)
	suite.NotNil(suite.ctx)
	suite.NotEmpty(suite.keeper)
}

// TestMsgServerDirectly tests basic keeper functionality
func (suite *MsgServerTestSuite) TestMsgServerDirectly() {
	// Test setting params
	params := types2.DefaultParams()
	err := suite.keeper.SetParams(suite.ctx, params)
	suite.NoError(err)

	// Test getting params from the keeper
	gotParams := suite.keeper.GetParams(suite.ctx)
	suite.Equal(params, gotParams)
}

// TestValidateContractsImplementation verifies the keeper implements required interfaces
func (suite *MsgServerTestSuite) TestValidateContractsImplementation() {
	// Verify the keeper implements the ActionRegistrar interface
	var _ keeper2.ActionRegistrar = &suite.keeper

	// Verify the keeper implements the ActionFinalizer interface
	var _ keeper2.ActionFinalizer = &suite.keeper

	// Verify the keeper implements the ActionApprover interface
	var _ keeper2.ActionApprover = &suite.keeper
}

// TestKeeperEventEmission tests event emission by the keeper
func (suite *MsgServerTestSuite) TestKeeperEventEmission() {

	// Create a context with event manager
	suite.ctx = suite.ctx.WithEventManager(sdk.NewEventManager())

	// Create sense metadata
	senseMetadata := &actionapi.SenseMetadata{
		DataHash:             "test_hash",
		DdAndFingerprintsMax: 10,
		DdAndFingerprintsIc:  5,
	}

	// Marshal metadata to bytes
	metadataBytes, err := suite.keeper.GetCodec().Marshal(senseMetadata)
	suite.NoError(err)

	// Create a test action with embedded metadata
	action := &actionapi.Action{
		Creator:     suite.creatorAddress.String(),
		ActionType:  actionapi.ActionType_ACTION_TYPE_SENSE,
		Price:       &v1beta1.Coin{Denom: "ulume", Amount: "100000"},
		BlockHeight: suite.ctx.BlockHeight(),
		State:       actionapi.ActionState_ACTION_STATE_PENDING,
		Metadata:    metadataBytes,
	}

	// Register the action
	_, err = suite.keeper.RegisterAction(suite.ctx, action)
	suite.NoError(err)

	// Get events
	events := suite.ctx.EventManager().Events()

	// Verify event was emitted
	found := false
	for _, event := range events {
		if event.Type == types2.EventTypeActionRegistered {
			found = true
			break
		}
	}
	suite.True(found, "action_registered event not found")
}

// TestMsgRequestAction tests the RequestAction message handler
func (suite *MsgServerTestSuite) TestMsgRequestAction() {
	suite.registerCascadeAction()
}

func (suite *MsgServerTestSuite) registerCascadeAction() string {
	cascadeMetadata := actionapi.CascadeMetadata{
		DataHash:   "test_hash",
		FileName:   "test_file",
		RqIdsIc:    20,
		Signatures: suite.signatureCascade,
	}

	// Convert to JSON using encoding/json
	cascadeMetadataBytes, err := protojson.Marshal(&cascadeMetadata)
	suite.NoError(err)

	// Create a RequestAction message
	msg := types2.MsgRequestAction{
		Creator:    suite.creatorAddress.String(),
		ActionType: "CASCADE",
		Price:      "100000ulume",
		Metadata:   string(cascadeMetadataBytes),
	}

	// Execute the message
	res, err := suite.msgServer.RequestAction(suite.ctx, &msg)
	suite.NoError(err)
	suite.NotNil(res)

	action, found := suite.keeper.GetActionByID(suite.ctx, res.ActionId)
	suite.True(found, "action not found")

	// Verify the response
	suite.Equal(res.ActionId, action.ActionID, "Action ID in response does not match the registered action")
	// Check the status of the action
	suite.Equal(actionapi.ActionState_ACTION_STATE_PENDING.String(), res.Status,
		"Expected action status to be PENDING, got %s", res.Status)
	// Verify the action's creator
	suite.Equal(suite.creatorAddress.String(), action.Creator,
		"Expected action creator to be %s, got %s", suite.creatorAddress.String(), action.Creator)
	// Verify the action type
	suite.Equal(actionapi.ActionType_ACTION_TYPE_CASCADE, action.ActionType,
		"Expected action type to be %s, got %s", actionapi.ActionType_ACTION_TYPE_CASCADE, action.ActionType)

	return res.ActionId
}
func (suite *MsgServerTestSuite) registerSenseAction() string {
	senseMetadata := actionapi.SenseMetadata{
		DataHash:            "test_hash",
		DdAndFingerprintsIc: suite.ic,
	}

	// Convert to JSON using encoding/json
	senseMetadataBytes, err := protojson.Marshal(&senseMetadata)
	suite.NoError(err)

	// Create a RequestAction message
	msg := types2.MsgRequestAction{
		Creator:    suite.creatorAddress.String(),
		ActionType: "SENSE",
		Price:      "100000ulume",
		Metadata:   string(senseMetadataBytes),
	}

	// Execute the message
	res, err := suite.msgServer.RequestAction(suite.ctx, &msg)
	suite.NoError(err)
	suite.NotNil(res)

	action, found := suite.keeper.GetActionByID(suite.ctx, res.ActionId)
	suite.True(found, "action not found")

	// Verify the response
	suite.Equal(res.ActionId, action.ActionID, "Action ID in response does not match the registered action")
	// Check the status of the action
	suite.Equal(actionapi.ActionState_ACTION_STATE_PENDING.String(), res.Status,
		"Expected action status to be PENDING, got %s", res.Status)
	// Verify the action's creator
	suite.Equal(suite.creatorAddress.String(), action.Creator,
		"Expected action creator to be %s, got %s", suite.creatorAddress.String(), action.Creator)
	// Verify the action type
	suite.Equal(actionapi.ActionType_ACTION_TYPE_SENSE, action.ActionType,
		"Expected action type to be %s, got %s", actionapi.ActionType_ACTION_TYPE_SENSE, action.ActionType)

	return res.ActionId
}

// TestMsgFinalizeAction tests the FinalizeAction message handler
func (suite *MsgServerTestSuite) TestMsgFinalizeAction() {
	actionID := suite.registerCascadeAction()
	suite.finalizeCascadeAction(actionID)
}

func (suite *MsgServerTestSuite) finalizeCascadeAction(actionID string) {
	var validIDs []string
	for i := suite.ic; i < suite.ic+50; i++ { // 50 is default value for MaxDdAndFingerprints
		id, err := keeper2.CreateKademliaID(suite.signatureCascade, i)
		suite.Require().NoError(err)
		validIDs = append(validIDs, id)
	}

	finalizeMetadata := actionapi.CascadeMetadata{
		RqIdsIds: validIDs,
	}

	finalizeMetadataBytes, err := protojson.Marshal(&finalizeMetadata)
	suite.NoError(err)

	// Create FinalizeAction message
	msgFinal := types2.MsgFinalizeAction{
		Creator:    suite.supernodes[0].SupernodeAccount,
		ActionId:   actionID,
		ActionType: "CASCADE",
		Metadata:   string(finalizeMetadataBytes),
	}

	resFinal, err := suite.msgServer.FinalizeAction(suite.ctx, &msgFinal)
	suite.NoError(err)
	suite.NotNil(resFinal)

	actionFinal, foundFinal := suite.keeper.GetActionByID(suite.ctx, actionID)
	suite.True(foundFinal, "action not found")

	suite.Equal(actionFinal.ActionID, actionID)
	suite.Equal(actionFinal.State, actionapi.ActionState_ACTION_STATE_DONE)
}
func (suite *MsgServerTestSuite) finalizeSenseAction(actionID string, superNode string, actionState actionapi.ActionState) {
	var validIDs []string
	for i := suite.ic; i < suite.ic+50; i++ { // 50 is default value for MaxDdAndFingerprints
		id, err := keeper2.CreateKademliaID(suite.signatureSense, i)
		suite.Require().NoError(err)
		validIDs = append(validIDs, id)
	}

	finalizeMetadata := actionapi.SenseMetadata{
		DdAndFingerprintsIds: validIDs,
		Signatures:           suite.signatureSense,
	}

	finalizeMetadataBytes, err := protojson.Marshal(&finalizeMetadata)
	suite.NoError(err)

	// Create FinalizeAction message
	msgFinal := types2.MsgFinalizeAction{
		Creator:    superNode,
		ActionId:   actionID,
		ActionType: "SENSE",
		Metadata:   string(finalizeMetadataBytes),
	}

	resFinal, err := suite.msgServer.FinalizeAction(suite.ctx, &msgFinal)
	suite.NoError(err)
	suite.NotNil(resFinal)

	actionFinal, foundFinal := suite.keeper.GetActionByID(suite.ctx, actionID)
	suite.True(foundFinal, "action not found")

	suite.Equal(actionFinal.ActionID, actionID)
	suite.Equal(actionFinal.State, actionState)
}

func (suite *MsgServerTestSuite) makeFinalizeCascadeActionMessage(actionID string, actionType string, superNode string, badMetadata string, rqIdsOtiBad bool, rqIdsBad bool) types2.MsgFinalizeAction {
	var validIDs []string
	if !rqIdsBad {
		for i := suite.ic; i < suite.ic+50; i++ { // 50 is default value for MaxDdAndFingerprints
			id, err := keeper2.CreateKademliaID(suite.signatureCascade, i)
			suite.Require().NoError(err)
			validIDs = append(validIDs, id)
		}
	}

	finalizeMetadata := actionapi.CascadeMetadata{
		RqIdsIds: validIDs,
	}

	finalizeMetadataBytes, err := protojson.Marshal(&finalizeMetadata)
	suite.NoError(err)

	var metadata string
	if badMetadata != "" {
		metadata = badMetadata
	} else {
		metadata = string(finalizeMetadataBytes)
	}

	return types2.MsgFinalizeAction{
		Creator:    superNode,
		ActionId:   actionID,
		ActionType: actionType,
		Metadata:   metadata,
	}

}

// TestMsgApproveAction tests the ApproveAction message handler
func (suite *MsgServerTestSuite) TestMsgApproveAction() {
	actionID := suite.registerCascadeAction()
	suite.finalizeCascadeAction(actionID)
	suite.approveAction(actionID, suite.creatorAddress.String())
}

func (suite *MsgServerTestSuite) approveActionNoCheck(actionID string, creator string) (*types2.MsgApproveActionResponse, error) {
	msg := types2.MsgApproveAction{
		Creator:  creator,
		ActionId: actionID,
	}

	// Execute the message
	res, err := suite.msgServer.ApproveAction(suite.ctx, &msg)
	if err != nil {
		return nil, err
	}
	return res, err
}

func (suite *MsgServerTestSuite) approveAction(actionID string, creator string) {
	res, err := suite.approveActionNoCheck(actionID, creator)
	suite.NoError(err)
	suite.NotNil(res)

	// Verify the action is now in APPROVED state
	updatedAction, found := suite.keeper.GetActionByID(suite.ctx, actionID)
	suite.True(found)
	suite.Equal(actionapi.ActionState_ACTION_STATE_APPROVED, updatedAction.State)
}
