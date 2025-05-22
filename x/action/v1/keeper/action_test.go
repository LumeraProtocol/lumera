package keeper_test

import (
	"fmt"

	"github.com/LumeraProtocol/lumera/x/action/v1/common"
	types2 "github.com/LumeraProtocol/lumera/x/action/v1/types"

	sdk "github.com/cosmos/cosmos-sdk/types"

	actionapi "github.com/LumeraProtocol/lumera/api/lumera/action"
)

func (suite *KeeperTestSuite) TestRegisterAction() {
	// Test cases for RegisterAction
	testCases := []struct {
		name      string
		creator   string
		action    *actionapi.Action
		expErr    error
		setupFunc func()
	}{
		{
			name:    "Register Cascade Action - Success",
			creator: suite.creatorAddress.String(),
			action:  suite.prepareCascadeActionForRegistration(suite.creatorAddress.String(), MetadataFieldToMissNone),
			expErr:  nil,
		},
		{
			name:    "Register Sense Action - Success",
			creator: suite.creatorAddress.String(),
			action:  suite.prepareSenseActionForRegistration(suite.creatorAddress.String(), MetadataFieldToMissNone),
			expErr:  nil,
		},
		{
			name:    "Register Cascade Action - Missing MetadataID",
			creator: suite.creatorAddress.String(),
			action: &actionapi.Action{
				Creator:    suite.creatorAddress.String(),
				ActionType: actionapi.ActionType_ACTION_TYPE_CASCADE,
				Price:      "100000ulume",
				Metadata:   nil, // Missing metadata
			},
			expErr: types2.ErrInvalidMetadata,
		},
		{
			name:    "Register Sense Action - Missing MetadataID",
			creator: suite.creatorAddress.String(),
			action: &actionapi.Action{
				Creator:    suite.creatorAddress.String(),
				ActionType: actionapi.ActionType_ACTION_TYPE_SENSE,
				Price:      "100000ulume",
				Metadata:   nil, // Missing metadata
			},
			expErr: types2.ErrInvalidMetadata,
		},
		{
			name:    "Register Cascade Action - Missing Signatures in MetadataID",
			creator: suite.creatorAddress.String(),
			action:  suite.prepareCascadeActionForRegistration(suite.creatorAddress.String(), MetadataFieldToMissSignatures),
			expErr:  types2.ErrInvalidMetadata,
		},
		{
			name:    "Register Action - Invalid State",
			creator: suite.creatorAddress.String(),
			action: func() *actionapi.Action {
				// Create sense metadata
				senseMetadata := &actionapi.SenseMetadata{
					DataHash:             "hash123",
					DdAndFingerprintsMax: 10,
				}

				// In actual test, we need to account for the possibility of panics during setup
				var metadataBytes []byte
				var err error
				metadataBytes, err = suite.keeper.GetCodec().Marshal(senseMetadata)
				if err != nil {
					return &actionapi.Action{
						Creator:    suite.creatorAddress.String(),
						ActionType: actionapi.ActionType_ACTION_TYPE_SENSE,
						Price:      "100000ulume",
						State:      actionapi.ActionState_ACTION_STATE_DONE, // Should start as UNSPECIFIED
						Metadata:   nil,                                     // Empty metadata ID
					}
				}

				// Create action with invalid state but with embedded metadata
				return &actionapi.Action{
					Creator:    suite.creatorAddress.String(),
					ActionType: actionapi.ActionType_ACTION_TYPE_SENSE,
					Price:      "100000ulume",
					State:      actionapi.ActionState_ACTION_STATE_DONE, // Should start as UNSPECIFIED
					Metadata:   metadataBytes,
				}
			}(),
			expErr: types2.ErrInvalidActionState,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Setup if needed
			if tc.setupFunc != nil {
				tc.setupFunc()
			}

			// Reset the suite's context to a clean state
			ctx := suite.ctx.WithBlockHeight(1)

			// Execute the function under test
			_, err := suite.keeper.RegisterAction(ctx, tc.action)

			// Check the result
			if tc.expErr != nil {
				suite.ErrorContains(err, tc.expErr.Error())
			} else {
				suite.NoError(err)
				suite.NotEmpty(tc.action.ActionID, "Action ID should not be empty")

				// Verify the action was correctly stored
				storedAction, found := suite.keeper.GetActionByID(ctx, tc.action.ActionID)
				suite.True(found, "Action should be found in store")
				suite.Equal(tc.action.Creator, storedAction.Creator, "Creator should match")
				suite.Equal(tc.action.ActionType, storedAction.ActionType, "ActionType should match")
				suite.Equal(tc.action.Price, storedAction.Price, "Price should match")
				suite.Equal(tc.action.BlockHeight, storedAction.BlockHeight, "BlockHeight should match")
				suite.Equal(actionapi.ActionState_ACTION_STATE_PENDING, storedAction.State, "State should be PENDING")
			}
		})
	}
}

func (suite *KeeperTestSuite) TestGetActionByID_NotFound() {
	// Test getting a non-existent action
	action, found := suite.keeper.GetActionByID(suite.ctx, "non-existent-id")
	suite.False(found, "Action should not be found")
	suite.Nil(action, "Action should be nil")
}

func (suite *KeeperTestSuite) TestIterateActions() {
	creator := suite.creatorAddress.String()

	// Create several actions
	actions := []*actionapi.Action{
		suite.prepareSenseActionForRegistration(creator, MetadataFieldToMissNone),
		suite.prepareCascadeActionForRegistration(creator, MetadataFieldToMissNone),
		suite.prepareSenseActionForRegistration(creator, MetadataFieldToMissNone),
	}

	// Store the actions
	var ids []string
	for _, action := range actions {
		_, err := suite.keeper.RegisterAction(suite.ctx, action)
		suite.NoError(err)
		ids = append(ids, action.ActionID)
	}

	// Count actions using iterator
	count := 0
	err := suite.keeper.IterateActions(suite.ctx, func(action *actionapi.Action) bool {
		count++
		return false // Continue iteration
	})
	suite.NoError(err)
	suite.Equal(len(actions), count, "Should iterate over all actions")

	// Verify individual actions can be found
	for _, id := range ids {
		actionFound := false
		err := suite.keeper.IterateActions(suite.ctx, func(action *actionapi.Action) bool {
			if action.ActionID == id {
				actionFound = true
				return true // Stop iteration
			}
			return false // Continue iteration
		})
		suite.NoError(err)
		suite.True(actionFound, fmt.Sprintf("Action with ID %s should be found", id))
	}
}

func (suite *KeeperTestSuite) TestFinalizeAction() {
	testCases := []struct {
		name             string
		creator          string
		action           *actionapi.Action
		finalizeMetadata []byte
		superNode        string
		state            actionapi.ActionState
		expErr           error
		setupFunc        func()
	}{
		{
			name:             "Finalizing Cascade Action - Success",
			creator:          suite.creatorAddress.String(),
			superNode:        suite.supernodes[0].SupernodeAccount,
			action:           suite.prepareCascadeActionForRegistration(suite.creatorAddress.String(), MetadataFieldToMissNone),
			finalizeMetadata: suite.generateCascadeFinalizationMetadata(MetadataFieldToMissNone),
			state:            actionapi.ActionState_ACTION_STATE_DONE,
			expErr:           nil,
		},
		{
			name:             "Finalizing Sense Action - Success",
			creator:          suite.creatorAddress.String(),
			superNode:        suite.supernodes[0].SupernodeAccount,
			action:           suite.prepareSenseActionForRegistration(suite.creatorAddress.String(), MetadataFieldToMissNone),
			finalizeMetadata: suite.generateSenseFinalizationMetadata(suite.signatureSense, MetadataFieldToMissNone),
			state:            actionapi.ActionState_ACTION_STATE_DONE,
			expErr:           nil,
		},
		{
			name:             "Finalizing Cascade Action - Wrong SN",
			creator:          suite.creatorAddress.String(),
			superNode:        suite.badSupernode.SupernodeAccount,
			action:           suite.prepareCascadeActionForRegistration(suite.creatorAddress.String(), MetadataFieldToMissNone),
			finalizeMetadata: suite.generateCascadeFinalizationMetadata(MetadataFieldToMissNone),
			expErr:           types2.ErrUnauthorizedSN,
		},
		{
			name:             "Finalizing Sense Action - Wrong SN",
			creator:          suite.creatorAddress.String(),
			superNode:        suite.badSupernode.SupernodeAccount,
			action:           suite.prepareSenseActionForRegistration(suite.creatorAddress.String(), MetadataFieldToMissNone),
			finalizeMetadata: suite.generateSenseFinalizationMetadata(suite.signatureSense, MetadataFieldToMissNone),
			expErr:           types2.ErrUnauthorizedSN,
		},
	}
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Setup if needed
			if tc.setupFunc != nil {
				tc.setupFunc()
			}

			_, err := suite.keeper.RegisterAction(suite.ctx, tc.action)
			suite.NoError(err)

			// Finalize the action
			err = suite.keeper.FinalizeAction(suite.ctx, tc.action.ActionID, tc.superNode, tc.finalizeMetadata)
			if tc.expErr != nil {
				suite.ErrorContains(err, tc.expErr.Error())
			} else {
				suite.NoError(err)

				// Verify the action was finalized
				updated, found := suite.keeper.GetActionByID(suite.ctx, tc.action.ActionID)
				suite.True(found)
				suite.Equal(tc.state, updated.State)
				suite.Equal(1, len(updated.SuperNodes))
				suite.Equal(tc.superNode, updated.SuperNodes[0])
			}
		})
	}
}

func (suite *KeeperTestSuite) TestFinalizeAction_Sense_SingleSN() {
	testCases := []struct {
		name             string
		creator          string
		superNode        string
		action           *actionapi.Action
		finalizeMetadata []byte
		expectedState    actionapi.ActionState
		expectedError    error
		verifySuperNodes bool
	}{
		{
			name:             "Single supernode finalizes successfully",
			creator:          suite.creatorAddress.String(),
			superNode:        suite.supernodes[0].SupernodeAccount,
			action:           suite.prepareSenseActionForRegistration(suite.creatorAddress.String(), MetadataFieldToMissNone),
			finalizeMetadata: suite.generateSenseFinalizationMetadata(suite.signatureSense, MetadataFieldToMissNone),
			expectedState:    actionapi.ActionState_ACTION_STATE_DONE,
			expectedError:    nil,
			verifySuperNodes: true,
		},
		{
			name:             "Invalid signature - finalization fails",
			creator:          suite.creatorAddress.String(),
			superNode:        suite.supernodes[0].SupernodeAccount,
			action:           suite.prepareSenseActionForRegistration(suite.creatorAddress.String(), MetadataFieldToMissNone),
			finalizeMetadata: suite.generateSenseFinalizationMetadata("bad-signature", MetadataFieldToMissNone),
			expectedState:    actionapi.ActionState_ACTION_STATE_PENDING,
			expectedError:    types2.ErrInvalidSignature,
			verifySuperNodes: false,
		},
		{
			name:             "Finalization called twice - second should be ignored",
			creator:          suite.creatorAddress.String(),
			superNode:        suite.supernodes[0].SupernodeAccount,
			action:           suite.prepareSenseActionForRegistration(suite.creatorAddress.String(), MetadataFieldToMissNone),
			finalizeMetadata: suite.generateSenseFinalizationMetadata(suite.signatureSense, MetadataFieldToMissNone),
			expectedState:    actionapi.ActionState_ACTION_STATE_DONE,
			expectedError:    nil,
			verifySuperNodes: true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Register action
			_, err := suite.keeper.RegisterAction(suite.ctx, tc.action)
			suite.NoError(err)

			// First finalize
			err = suite.keeper.FinalizeAction(suite.ctx, tc.action.ActionID, tc.superNode, tc.finalizeMetadata)

			if tc.expectedError != nil {
				suite.ErrorContains(err, tc.expectedError.Error())
			} else {
				suite.NoError(err)
			}

			// Get updated action
			updated, found := suite.keeper.GetActionByID(suite.ctx, tc.action.ActionID)
			suite.True(found)
			suite.Equal(tc.expectedState, updated.State)

			if tc.verifySuperNodes {
				suite.Contains(updated.SuperNodes, tc.superNode)
			} else {
				suite.NotContains(updated.SuperNodes, tc.superNode)
			}
		})
	}
}

func (suite *KeeperTestSuite) TestFinalizeAction_NotFound() {
	superNode := suite.supernodes[0].SupernodeAccount
	cascadeMetadata := suite.generateCascadeFinalizationMetadata(MetadataFieldToMissNone)
	senseMetadata := suite.generateSenseFinalizationMetadata(suite.signatureSense, MetadataFieldToMissNone)

	// Attempt to finalize non-existent action
	err := suite.keeper.FinalizeAction(suite.ctx, "non-existent-id", superNode, cascadeMetadata)
	suite.ErrorContains(err, "not found")
	err = suite.keeper.FinalizeAction(suite.ctx, "non-existent-id", superNode, senseMetadata)
	suite.ErrorContains(err, "not found")
}

func (suite *KeeperTestSuite) TestFinalizeAction_Again_Cascade() {
	creator := suite.creatorAddress.String()
	superNode := suite.supernodes[0].SupernodeAccount

	// Create an action
	action := suite.prepareCascadeActionForRegistration(creator, MetadataFieldToMissNone)
	_, err := suite.keeper.RegisterAction(suite.ctx, action)
	suite.NoError(err)

	// Prepare finalization metadata
	metadata := suite.generateCascadeFinalizationMetadata(MetadataFieldToMissNone)

	// Finalize the action
	err = suite.keeper.FinalizeAction(suite.ctx, action.ActionID, superNode, []byte(metadata))
	suite.NoError(err)

	// Try to finalize again
	err = suite.keeper.FinalizeAction(suite.ctx, action.ActionID, superNode, []byte(metadata))
	suite.ErrorContains(err, "cannot be finalized")
}

func (suite *KeeperTestSuite) TestFinalizeAction_Again_Sense() {
	creator := suite.creatorAddress.String()
	superNode := suite.supernodes[0].SupernodeAccount

	// Create an action
	action := suite.prepareSenseActionForRegistration(creator, MetadataFieldToMissNone)
	_, err := suite.keeper.RegisterAction(suite.ctx, action)
	suite.NoError(err)

	// Prepare finalization metadata
	metadata := suite.generateSenseFinalizationMetadata(suite.signatureSense, MetadataFieldToMissNone)

	// Finalize the action
	err = suite.keeper.FinalizeAction(suite.ctx, action.ActionID, superNode, []byte(metadata))
	suite.NoError(err)

	// Try to finalize again
	err = suite.keeper.FinalizeAction(suite.ctx, action.ActionID, superNode, []byte(metadata))
	suite.ErrorContains(err, "action 1 cannot be finalized: current state ACTION_STATE_DONE is not one of pending or processing")
}

func (suite *KeeperTestSuite) TestApproveAction() {
	creator := suite.creatorAddress.String()
	superNode := suite.supernodes[0].SupernodeAccount

	// Create an action
	action := suite.prepareCascadeActionForRegistration(creator, MetadataFieldToMissNone)
	_, err := suite.keeper.RegisterAction(suite.ctx, action)
	suite.NoError(err)

	// Finalize the action first
	metadata := suite.generateCascadeFinalizationMetadata(MetadataFieldToMissNone)
	err = suite.keeper.FinalizeAction(suite.ctx, action.ActionID, superNode, metadata)
	suite.NoError(err)

	// Approve the action
	err = suite.keeper.ApproveAction(suite.ctx, action.ActionID, creator)
	suite.NoError(err)

	// Verify the action was approved
	updated, found := suite.keeper.GetActionByID(suite.ctx, action.ActionID)
	suite.True(found)
	suite.Equal(actionapi.ActionState_ACTION_STATE_APPROVED, updated.State)
}

func (suite *KeeperTestSuite) TestApproveAction_NotFound() {
	creator := suite.creatorAddress.String()

	// Attempt to approve non-existent action
	err := suite.keeper.ApproveAction(suite.ctx, "non-existent-id", creator)
	suite.ErrorContains(err, "not found")
}

func (suite *KeeperTestSuite) TestApproveAction_InvalidState() {
	creator := suite.creatorAddress.String()

	// Create an action
	action := suite.prepareCascadeActionForRegistration(creator, MetadataFieldToMissNone)
	_, err := suite.keeper.RegisterAction(suite.ctx, action)
	suite.NoError(err)

	// Try to approve without finalization first
	err = suite.keeper.ApproveAction(suite.ctx, action.ActionID, creator)
	suite.ErrorContains(err, "cannot be approved")
}

func (suite *KeeperTestSuite) TestApproveAction_UnauthorizedCreator() {
	creator := suite.creatorAddress.String()
	imposter := suite.imposterAddress.String()
	superNode := suite.supernodes[0].SupernodeAccount

	// Create an action
	action := suite.prepareCascadeActionForRegistration(creator, MetadataFieldToMissNone)
	_, err := suite.keeper.RegisterAction(suite.ctx, action)
	suite.NoError(err)

	// Finalize the action first
	metadata := suite.generateCascadeFinalizationMetadata(MetadataFieldToMissNone)
	err = suite.keeper.FinalizeAction(suite.ctx, action.ActionID, superNode, metadata)
	suite.NoError(err)

	// Try to approve with wrong creator
	err = suite.keeper.ApproveAction(suite.ctx, action.ActionID, imposter)
	suite.ErrorContains(err, "only the creator")
}

func (suite *KeeperTestSuite) TestValidateMetadata_Cascade() {
	actionHandler, err := suite.keeper.GetActionRegistry().GetHandler(actionapi.ActionType_ACTION_TYPE_CASCADE)
	suite.NoError(err)

	params := types2.DefaultParams()

	invalidCascadeAction := suite.prepareCascadeActionForRegistration(suite.creatorAddress.String(), MetadataFieldToMissDataHash)
	_, err = actionHandler.Process(invalidCascadeAction.Metadata, common.MsgRequestAction, &params)
	suite.Error(err, "data_hash is required for cascade metadata")

	invalidCascadeAction = suite.prepareCascadeActionForRegistration(suite.creatorAddress.String(), MetadataFieldToMissFileName)
	_, err = actionHandler.Process(invalidCascadeAction.Metadata, common.MsgRequestAction, &params)
	suite.Error(err, "file_name is required in existing metadata")

	invalidCascadeAction = suite.prepareCascadeActionForRegistration(suite.creatorAddress.String(), MetadataFieldToMissIdsIc)
	_, err = actionHandler.Process(invalidCascadeAction.Metadata, common.MsgRequestAction, &params)
	suite.Error(err, "rq_ids_ic is required in existing metadata")

	invalidCascadeAction = suite.prepareCascadeActionForRegistration(suite.creatorAddress.String(), MetadataFieldToMissSignatures)
	_, err = actionHandler.Process(invalidCascadeAction.Metadata, common.MsgRequestAction, &params)
	suite.Error(err, "signatures is required in existing metadata")

	cascadeMeta := suite.generateCascadeFinalizationMetadata(MetadataFieldToMissIds)
	_, err = actionHandler.Process(cascadeMeta, common.MsgFinalizeAction, &params)
	suite.Error(err, "rq_ids_ids is required for cascade metadata")

	cascadeMeta = suite.generateCascadeFinalizationMetadata(MetadataFieldToMissRqOti)
	_, err = actionHandler.Process(cascadeMeta, common.MsgFinalizeAction, &params)
	suite.Error(err, "rq_ids_oti is required for cascade metadata")
}

func (suite *KeeperTestSuite) TestValidateMetadata_Sense() {
	actionHandler, err := suite.keeper.GetActionRegistry().GetHandler(actionapi.ActionType_ACTION_TYPE_SENSE)
	suite.NoError(err)

	params := types2.DefaultParams()

	invalidSenseAction := suite.prepareSenseActionForRegistration(suite.creatorAddress.String(), MetadataFieldToMissDataHash)
	_, err = actionHandler.Process(invalidSenseAction.Metadata, common.MsgRequestAction, &params)
	suite.Error(err, "data_hash is required for sense metadata")

	invalidSenseAction = suite.prepareSenseActionForRegistration(suite.creatorAddress.String(), MetadataFieldToMissIdsIc)
	_, err = actionHandler.Process(invalidSenseAction.Metadata, common.MsgRequestAction, &params)
	suite.Error(err, "dd_and_fingerprints_ic is required in sense metadata")

	invalidSenseMeta := suite.generateSenseFinalizationMetadata(suite.signatureSense, MetadataFieldToMissIds)
	_, err = actionHandler.Process(invalidSenseMeta, common.MsgRequestAction, &params)
	suite.Error(err, "dd_and_fingerprints_ids is required in sense metadata")

	invalidSenseMeta = suite.generateSenseFinalizationMetadata(suite.signatureSense, MetadataFieldToMissSignatures)
	_, err = actionHandler.Process(invalidSenseMeta, common.MsgRequestAction, &params)
	suite.Error(err, "signatures is required in sense metadata")
}

func (suite *KeeperTestSuite) TestIterateActionsByState_Cascade() {
	creator := suite.creatorAddress.String()
	superNode := suite.supernodes[0].SupernodeAccount

	// Create actions in different states
	pendingAction := suite.prepareCascadeActionForRegistration(creator, MetadataFieldToMissNone)
	_, err := suite.keeper.RegisterAction(suite.ctx, pendingAction)
	suite.NoError(err)

	metadata := suite.generateCascadeFinalizationMetadata(MetadataFieldToMissNone)

	finalizingAction := suite.prepareCascadeActionForRegistration(creator, MetadataFieldToMissNone)
	_, err = suite.keeper.RegisterAction(suite.ctx, finalizingAction)
	suite.NoError(err)
	err = suite.keeper.FinalizeAction(suite.ctx, finalizingAction.ActionID, superNode, metadata)
	suite.NoError(err)

	approvedAction := suite.prepareCascadeActionForRegistration(creator, MetadataFieldToMissNone)
	_, err = suite.keeper.RegisterAction(suite.ctx, approvedAction)
	suite.NoError(err)
	err = suite.keeper.FinalizeAction(suite.ctx, approvedAction.ActionID, superNode, metadata)
	suite.NoError(err)
	err = suite.keeper.ApproveAction(suite.ctx, approvedAction.ActionID, creator)
	suite.NoError(err)

	// Test iterating over PENDING actions
	pendingCount := 0
	err = suite.keeper.IterateActionsByState(suite.ctx, actionapi.ActionState_ACTION_STATE_PENDING, func(action *actionapi.Action) bool {
		pendingCount++
		return false // Continue iteration
	})
	suite.NoError(err)
	suite.Equal(1, pendingCount, "Should have 1 PENDING action")

	// Test iterating over DONE actions
	doneCount := 0
	err = suite.keeper.IterateActionsByState(suite.ctx, actionapi.ActionState_ACTION_STATE_DONE, func(action *actionapi.Action) bool {
		doneCount++
		suite.Equal(finalizingAction.ActionID, action.ActionID, "Should be the finalized action")
		return false // Continue iteration
	})
	suite.NoError(err)
	suite.Equal(1, doneCount, "Should have 1 DONE action")

	// Test iterating over APPROVED actions
	approvedCount := 0
	err = suite.keeper.IterateActionsByState(suite.ctx, actionapi.ActionState_ACTION_STATE_APPROVED, func(action *actionapi.Action) bool {
		approvedCount++
		suite.Equal(approvedAction.ActionID, action.ActionID, "Should be the approved action")
		return false // Continue iteration
	})
	suite.NoError(err)
	suite.Equal(1, approvedCount, "Should have 1 APPROVED action")
}

func (suite *KeeperTestSuite) TestIterateActionsByState_Sense() {
	creator := suite.creatorAddress.String()
	superNode1 := suite.supernodes[0].SupernodeAccount

	// Create actions in different states
	pendingAction := suite.prepareSenseActionForRegistration(creator, MetadataFieldToMissNone)
	_, err := suite.keeper.RegisterAction(suite.ctx, pendingAction)
	suite.NoError(err)

	metadata := suite.generateSenseFinalizationMetadata(suite.signatureSense, MetadataFieldToMissNone)

	doneAction := suite.prepareSenseActionForRegistration(creator, MetadataFieldToMissNone)
	_, err = suite.keeper.RegisterAction(suite.ctx, doneAction)
	suite.NoError(err)
	err = suite.keeper.FinalizeAction(suite.ctx, doneAction.ActionID, superNode1, metadata)
	suite.NoError(err)

	approvedAction := suite.prepareSenseActionForRegistration(creator, MetadataFieldToMissNone)
	_, err = suite.keeper.RegisterAction(suite.ctx, approvedAction)
	suite.NoError(err)
	err = suite.keeper.FinalizeAction(suite.ctx, approvedAction.ActionID, superNode1, metadata)
	suite.NoError(err)
	suite.NoError(err)
	err = suite.keeper.ApproveAction(suite.ctx, approvedAction.ActionID, creator)
	suite.NoError(err)

	// Test iterating over PENDING actions
	pendingCount := 0
	err = suite.keeper.IterateActionsByState(suite.ctx, actionapi.ActionState_ACTION_STATE_PENDING, func(action *actionapi.Action) bool {
		pendingCount++
		return false // Continue iteration
	})
	suite.NoError(err)
	suite.Equal(1, pendingCount, "Should have 1 PENDING action")

	// Test iterating over PROCESSING actions
	processingCount := 0
	err = suite.keeper.IterateActionsByState(suite.ctx, actionapi.ActionState_ACTION_STATE_PROCESSING, func(action *actionapi.Action) bool {
		processingCount++
		return false // Continue iteration
	})
	suite.NoError(err)
	suite.Equal(0, processingCount, "Should have 0 PROCESSING action")

	// Test iterating over DONE actions
	doneCount := 0
	err = suite.keeper.IterateActionsByState(suite.ctx, actionapi.ActionState_ACTION_STATE_DONE, func(action *actionapi.Action) bool {
		doneCount++
		suite.Equal(doneAction.ActionID, action.ActionID, "Should be the finalized action")
		return false // Continue iteration
	})
	suite.NoError(err)
	suite.Equal(1, doneCount, "Should have 1 DONE action")

	// Test iterating over APPROVED actions
	approvedCount := 0
	err = suite.keeper.IterateActionsByState(suite.ctx, actionapi.ActionState_ACTION_STATE_APPROVED, func(action *actionapi.Action) bool {
		approvedCount++
		suite.Equal(approvedAction.ActionID, action.ActionID, "Should be the approved action")
		return false // Continue iteration
	})
	suite.NoError(err)
	suite.Equal(1, approvedCount, "Should have 1 APPROVED action")
}

func (suite *KeeperTestSuite) TestFeeDistribution() {
	creator := suite.creatorAddress.String()
	superNode := suite.supernodes[0].SupernodeAccount

	creatorAcc, err := sdk.AccAddressFromBech32(creator)
	suite.NoError(err)

	creatorBalanceBefore := suite.keeper.GetBankKeeper().GetBalance(suite.ctx, creatorAcc, "ulume")

	// Create an action with a fee
	action := suite.prepareCascadeActionForRegistration(creator, MetadataFieldToMissNone)
	action.Price = "100000ulume"
	_, err = suite.keeper.RegisterAction(suite.ctx, action)
	suite.NoError(err)

	creatorBalanceAfter := suite.keeper.GetBankKeeper().GetBalance(suite.ctx, creatorAcc, "ulume")
	shouldBe := creatorBalanceBefore.Amount.Int64() - 100000
	suite.Equal(shouldBe, creatorBalanceAfter.Amount.Int64(), "Supernode should receive the fee")

	snAcc, err := sdk.AccAddressFromBech32(superNode)
	suite.NoError(err)

	// Get balance on the supernode account before distribution
	balanceBefore := suite.keeper.GetBankKeeper().GetBalance(suite.ctx, snAcc, "ulume")

	// Finalize the action
	metadata := suite.generateCascadeFinalizationMetadata(MetadataFieldToMissNone)
	err = suite.keeper.FinalizeAction(suite.ctx, action.ActionID, superNode, metadata)
	suite.NoError(err)

	// Get balance on the supernode account before distribution
	balanceAfter := suite.keeper.GetBankKeeper().GetBalance(suite.ctx, snAcc, "ulume")
	shouldBe = balanceBefore.Amount.Int64() + (100000 * 0.9) // 90% of the fee, 10% to Community Pool
	suite.Equal(shouldBe, balanceAfter.Amount.Int64(), "Supernode should receive the fee")
}
