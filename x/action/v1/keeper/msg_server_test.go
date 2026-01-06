package keeper_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/action/v1/types"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/gogoproto/jsonpb"
	icatypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/types"
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

func (suite *MsgServerTestSuite) TestMsgRequestActionStoresAppPubkey() {
	base := authtypes.NewBaseAccountWithAddress(suite.creatorAddress)
	ica := icatypes.NewInterchainAccount(base, "owner")
	suite.keeper.GetAuthKeeper().SetAccount(suite.ctx, ica)

	cascadeMetadata := types.CascadeMetadata{
		DataHash:   "test_hash",
		FileName:   "test_file",
		RqIdsIc:    20,
		Signatures: suite.signatureCascade,
	}

	var cascadeMetadataBytes bytes.Buffer
	marshaler := &jsonpb.Marshaler{}
	err := marshaler.Marshal(&cascadeMetadataBytes, &cascadeMetadata)
	suite.NoError(err)

	appPubkey := suite.accountPairs[3].PubKey.Bytes()
	msg := types.MsgRequestAction{
		Creator:     suite.creatorAddress.String(),
		ActionType:  "CASCADE",
		Price:       "100000ulume",
		Metadata:    cascadeMetadataBytes.String(),
		FileSizeKbs: "123",
		AppPubkey:   appPubkey,
	}

	res, err := suite.msgServer.RequestAction(suite.ctx, &msg)
	suite.Require().NoError(err)
	suite.Require().NotNil(res)

	action, found := suite.keeper.GetActionByID(suite.ctx, res.ActionId)
	suite.True(found, "action not found")
	suite.Equal(appPubkey, action.AppPubkey)
}

func (suite *MsgServerTestSuite) TestMsgRequestActionRejectsAppPubkeyForNonICA() {
	cascadeMetadata := types.CascadeMetadata{
		DataHash:   "test_hash",
		FileName:   "test_file",
		RqIdsIc:    20,
		Signatures: suite.signatureCascade,
	}

	var cascadeMetadataBytes bytes.Buffer
	marshaler := &jsonpb.Marshaler{}
	err := marshaler.Marshal(&cascadeMetadataBytes, &cascadeMetadata)
	suite.NoError(err)

	msg := types.MsgRequestAction{
		Creator:     suite.creatorAddress.String(),
		ActionType:  "CASCADE",
		Price:       "100000ulume",
		Metadata:    cascadeMetadataBytes.String(),
		FileSizeKbs: "123",
		AppPubkey:   []byte{1, 2, 3},
	}

	res, err := suite.msgServer.RequestAction(suite.ctx, &msg)
	suite.Error(err)
	suite.Nil(res)
	suite.ErrorIs(err, types.ErrInvalidAppPubKey)
}

func (suite *MsgServerTestSuite) TestMsgRequestActionICARequiresAppPubkey() {
	base := authtypes.NewBaseAccountWithAddress(suite.creatorAddress)
	ica := icatypes.NewInterchainAccount(base, "owner")
	suite.keeper.GetAuthKeeper().SetAccount(suite.ctx, ica)

	cascadeMetadata := types.CascadeMetadata{
		DataHash:   "test_hash",
		FileName:   "test_file",
		RqIdsIc:    20,
		Signatures: suite.signatureCascade,
	}

	var cascadeMetadataBytes bytes.Buffer
	marshaler := &jsonpb.Marshaler{}
	err := marshaler.Marshal(&cascadeMetadataBytes, &cascadeMetadata)
	suite.NoError(err)

	msg := types.MsgRequestAction{
		Creator:     suite.creatorAddress.String(),
		ActionType:  "CASCADE",
		Price:       "100000ulume",
		Metadata:    cascadeMetadataBytes.String(),
		FileSizeKbs: "123",
	}

	res, err := suite.msgServer.RequestAction(suite.ctx, &msg)
	suite.Error(err)
	suite.Nil(res)
	suite.ErrorIs(err, types.ErrInvalidAppPubKey)
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
		Creator:     suite.creatorAddress.String(),
		ActionType:  "CASCADE",
		Price:       "100000ulume",
		Metadata:    cascadeMetadataBytes.String(),
		FileSizeKbs: "123",
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
	// Verify file size
	suite.Equal(int64(123), action.FileSizeKbs)

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
		Creator:     suite.creatorAddress.String(),
		ActionType:  "SENSE",
		Price:       "100000ulume",
		Metadata:    senseMetadataBytes.String(),
		FileSizeKbs: "456",
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
	// Verify file size
	suite.Equal(int64(456), action.FileSizeKbs)

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

func (suite *MsgServerTestSuite) TestMsgApproveActionTxMsgDataContainsResponseFields() {
	suite.setupExpectationsGetAllTopSNs(1)
	actionID := suite.registerCascadeAction()
	suite.finalizeCascadeAction(actionID)

	suite.ctx = suite.ctx.WithEventManager(sdk.NewEventManager())

	msg := types.MsgApproveAction{
		Creator:  suite.creatorAddress.String(),
		ActionId: actionID,
	}

	res, err := suite.msgServer.ApproveAction(suite.ctx, &msg)
	suite.Require().NoError(err)
	suite.Require().NotNil(res)

	result, err := sdk.WrapServiceResult(suite.ctx, res, nil)
	suite.Require().NoError(err)

	txMsgData := sdk.TxMsgData{MsgResponses: result.MsgResponses}
	bz, err := suite.keeper.GetCodec().Marshal(&txMsgData)
	suite.Require().NoError(err)

	var decoded sdk.TxMsgData
	err = suite.keeper.GetCodec().Unmarshal(bz, &decoded)
	suite.Require().NoError(err)
	suite.Require().Len(decoded.MsgResponses, 1)

	var approveRes types.MsgApproveActionResponse
	err = suite.keeper.GetCodec().Unmarshal(decoded.MsgResponses[0].Value, &approveRes)
	suite.Require().NoError(err)
	suite.Require().Equal(actionID, approveRes.ActionId)
	suite.Require().Equal(actiontypes.ActionStateApproved.String(), approveRes.Status)
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
	suite.Equal(actionID, res.ActionId, "Expected response to return action ID")
	suite.Equal(actiontypes.ActionStateApproved.String(), res.Status, "Expected response to return approved status")

	// Verify the action is now in APPROVED state
	updatedAction, found := suite.keeper.GetActionByID(suite.ctx, actionID)
	suite.True(found)
	suite.Equal(actiontypes.ActionStateApproved, updatedAction.State)
}
