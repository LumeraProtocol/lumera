package keeper_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/action/v1/types"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/jsonpb"
)

type MsgServerTestSuite struct {
	KeeperTestSuite
	msgServer types.MsgServer
}

func TestMsgServerTestSuite(t *testing.T) {
	suite.Run(t, new(MsgServerTestSuite))
}

func (suite *MsgServerTestSuite) SetupTest() {
	suite.KeeperTestSuite.SetupTest()
	suite.msgServer = keeper.NewMsgServerImpl(suite.keeper)
}

func (suite *MsgServerTestSuite) TestMsgServer() {
	suite.NotNil(suite.msgServer)
	suite.NotNil(suite.ctx)
	suite.NotEmpty(suite.keeper)
}

// TestMsgServerDirectly tests basic keeper functionality
func (suite *MsgServerTestSuite) TestMsgServerDirectly() {
	// Test setting params
	params := types.DefaultParams()
	err := suite.keeper.SetParams(suite.ctx, params)
	suite.NoError(err)

	// Test getting params from the keeper
	gotParams := suite.keeper.GetParams(suite.ctx)
	suite.Equal(params, gotParams)
}

// TestValidateContractsImplementation verifies the keeper implements required interfaces
func (suite *MsgServerTestSuite) TestValidateContractsImplementation() {
	// Verify the keeper implements the ActionRegistrar interface
	var _ keeper.ActionRegistrar = &suite.keeper

	// Verify the keeper implements the ActionFinalizer interface
	var _ keeper.ActionFinalizer = &suite.keeper

	// Verify the keeper implements the ActionApprover interface
	var _ keeper.ActionApprover = &suite.keeper
}

// TestKeeperEventEmission tests event emission by the keeper
func (suite *MsgServerTestSuite) TestKeeperEventEmission() {

	// Create a context with event manager
	suite.ctx = suite.ctx.WithEventManager(sdk.NewEventManager())

	// Create sense metadata
	senseMetadata := &actiontypes.SenseMetadata{
		DataHash:             "test_hash",
		DdAndFingerprintsMax: 10,
		DdAndFingerprintsIc:  5,
	}

	// Marshal metadata to bytes
	metadataBytes, err := suite.keeper.GetCodec().Marshal(senseMetadata)
	suite.NoError(err)

	testPrice := sdk.NewInt64Coin("ulume", 100000)
	// Create a test action with embedded metadata
	action := &actiontypes.Action{
		Creator:     suite.creatorAddress.String(),
		ActionType:  types.ActionTypeSense,
		Price:       testPrice.String(),
		BlockHeight: suite.ctx.BlockHeight(),
		State:       actiontypes.ActionStatePending,
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
		if event.Type == types.EventTypeActionRegistered {
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
	cascadeMetadata := types.CascadeMetadata{
		DataHash:   "test_hash",
		FileName:   "test_file",
		RqIdsIc:    20,
		Signatures: suite.signatureCascade,
	}

	// Convert to JSON using jsonpb
	var cascadeMetadataBytes bytes.Buffer
	marshaler := &jsonpb.Marshaler{}
	err := marshaler.Marshal(&cascadeMetadataBytes, &cascadeMetadata)
	suite.NoError(err)

	// Create a RequestAction message
	msg := types.MsgRequestAction{
		Creator:    suite.creatorAddress.String(),
		ActionType: "CASCADE",
		Price:      "100000ulume",
		Metadata:   cascadeMetadataBytes.String(),
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
	suite.Equal(actiontypes.ActionStatePending.String(), res.Status,
		"Expected action status to be PENDING, got %s", res.Status)
	// Verify the action's creator
	suite.Equal(suite.creatorAddress.String(), action.Creator,
		"Expected action creator to be %s, got %s", suite.creatorAddress.String(), action.Creator)
	// Verify the action type
	suite.Equal(types.ActionTypeCascade, action.ActionType,
		"Expected action type to be %s, got %s", types.ActionTypeCascade, action.ActionType)

	return res.ActionId
}

func (suite *MsgServerTestSuite) registerSenseAction() string {
	senseMetadata := types.SenseMetadata{
		DataHash:            "test_hash",
		DdAndFingerprintsIc: suite.ic,
	}

	// Convert to JSON using jsonpb
	marshaller := &jsonpb.Marshaler{}
	var senseMetadataBytes bytes.Buffer
	err := marshaller.Marshal(&senseMetadataBytes, &senseMetadata)
	suite.NoError(err)

	// Create a RequestAction message
	msg := types.MsgRequestAction{
		Creator:    suite.creatorAddress.String(),
		ActionType: "SENSE",
		Price:      "100000ulume",
		Metadata:   senseMetadataBytes.String(),
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
	suite.Equal(actiontypes.ActionStatePending.String(), res.Status,
		"Expected action status to be PENDING, got %s", res.Status)
	// Verify the action's creator
	suite.Equal(suite.creatorAddress.String(), action.Creator,
		"Expected action creator to be %s, got %s", suite.creatorAddress.String(), action.Creator)
	// Verify the action type
	suite.Equal(actiontypes.ActionTypeSense, action.ActionType,
		"Expected action type to be %s, got %s", actiontypes.ActionTypeSense, action.ActionType)

	return res.ActionId
}

// TestMsgFinalizeAction tests the FinalizeAction message handler
func (suite *MsgServerTestSuite) TestMsgFinalizeAction() {
	suite.setupExpectationsGetAllTopSNs(1)

	actionID := suite.registerCascadeAction()
	suite.finalizeCascadeAction(actionID)
}

func (suite *MsgServerTestSuite) finalizeCascadeAction(actionID string) {
	var validIDs []string
	for i := suite.ic; i < suite.ic+50; i++ { // 50 is default value for MaxDdAndFingerprints
		id, err := keeper.CreateKademliaID(suite.signatureCascade, i)
		suite.Require().NoError(err)
		validIDs = append(validIDs, id)
	}

	finalizeMetadata := actiontypes.CascadeMetadata{
		RqIdsIds: validIDs,
	}

	// Convert to JSON using jsonpb
	marshaller := &jsonpb.Marshaler{}
	var finalizeMetadataBytes bytes.Buffer
	err := marshaller.Marshal(&finalizeMetadataBytes, &finalizeMetadata)
	suite.NoError(err)

	// Create FinalizeAction message
	msgFinal := types.MsgFinalizeAction{
		Creator:    suite.supernodes[0].SupernodeAccount,
		ActionId:   actionID,
		ActionType: "CASCADE",
		Metadata:   finalizeMetadataBytes.String(),
	}

	resFinal, err := suite.msgServer.FinalizeAction(suite.ctx, &msgFinal)
	suite.NoError(err)
	suite.NotNil(resFinal)

	actionFinal, foundFinal := suite.keeper.GetActionByID(suite.ctx, actionID)
	suite.True(foundFinal, "action not found")

	suite.Equal(actionFinal.ActionID, actionID)
	suite.Equal(actionFinal.State, actiontypes.ActionStateDone)
}

func (suite *MsgServerTestSuite) finalizeSenseAction(actionID string, superNode string, actionState actiontypes.ActionState) {
	var validIDs []string
	for i := suite.ic; i < suite.ic+50; i++ { // 50 is default value for MaxDdAndFingerprints
		id, err := keeper.CreateKademliaID(suite.signatureSense, i)
		suite.Require().NoError(err)
		validIDs = append(validIDs, id)
	}

	finalizeMetadata := actiontypes.SenseMetadata{
		DdAndFingerprintsIds: validIDs,
		Signatures:           suite.signatureSense,
	}

	// Convert to JSON using jsonpb
	marshaller := &jsonpb.Marshaler{}
	var finalizeMetadataBytes bytes.Buffer
	err := marshaller.Marshal(&finalizeMetadataBytes, &finalizeMetadata)
	suite.NoError(err)

	// Create FinalizeAction message
	msgFinal := types.MsgFinalizeAction{
		Creator:    superNode,
		ActionId:   actionID,
		ActionType: "SENSE",
		Metadata:   finalizeMetadataBytes.String(),
	}

	resFinal, err := suite.msgServer.FinalizeAction(suite.ctx, &msgFinal)
	suite.NoError(err)
	suite.NotNil(resFinal)

	actionFinal, foundFinal := suite.keeper.GetActionByID(suite.ctx, actionID)
	suite.True(foundFinal, "action not found")

	suite.Equal(actionFinal.ActionID, actionID)
	suite.Equal(actionFinal.State, actionState)
}

func (suite *MsgServerTestSuite) makeFinalizeCascadeActionMessage(actionID string, actionType string, superNode string, badMetadata string, _ bool, rqIdsBad bool) types.MsgFinalizeAction {
	var validIDs []string
	if !rqIdsBad {
		for i := suite.ic; i < suite.ic+50; i++ { // 50 is default value for MaxDdAndFingerprints
			id, err := keeper.CreateKademliaID(suite.signatureCascade, i)
			suite.Require().NoError(err)
			validIDs = append(validIDs, id)
		}
	}

	finalizeMetadata := actiontypes.CascadeMetadata{
		RqIdsIds: validIDs,
	}

	// Convert to JSON using jsonpb
	marshaller := &jsonpb.Marshaler{}
	var finalizeMetadataBytes bytes.Buffer
	err := marshaller.Marshal(&finalizeMetadataBytes, &finalizeMetadata)
	suite.NoError(err)

	var metadata string
	if badMetadata != "" {
		metadata = badMetadata
	} else {
		metadata = finalizeMetadataBytes.String()
	}

	return types.MsgFinalizeAction{
		Creator:    superNode,
		ActionId:   actionID,
		ActionType: actionType,
		Metadata:   metadata,
	}

}

// TestMsgApproveAction tests the ApproveAction message handler
func (suite *MsgServerTestSuite) TestMsgApproveAction() {
	suite.setupExpectationsGetAllTopSNs(1)
	actionID := suite.registerCascadeAction()
	suite.finalizeCascadeAction(actionID)
	suite.approveAction(actionID, suite.creatorAddress.String())
}

func (suite *MsgServerTestSuite) approveActionNoCheck(actionID string, creator string) (*types.MsgApproveActionResponse, error) {
	msg := types.MsgApproveAction{
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
	suite.Equal(actiontypes.ActionStateApproved, updatedAction.State)
}
