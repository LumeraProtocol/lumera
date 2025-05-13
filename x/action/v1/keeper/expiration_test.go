package keeper_test

import (
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"

	"github.com/stretchr/testify/suite"
	"github.com/golang/mock/gomock"

	"github.com/LumeraProtocol/lumera/testutil/cryptotestutils"
	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	"github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ExpirationTestSuite tests the expiration functionality
type ExpirationTestSuite struct {
	suite.Suite
	ctx         sdk.Context
	keeper      keeper.Keeper
	signature   string
	testAddr    sdk.AccAddress
	testValAddr sdk.ValAddress
	blockTime   time.Time
}

func (suite *ExpirationTestSuite) SetupTest() {
	ctrl := gomock.NewController(suite.T())
	defer ctrl.Finish()
	
	key, address := cryptotestutils.KeyAndAddress()
	pubKey := key.PubKey()
	pairs := []keepertest.AccountPair{{Address: address, PubKey: pubKey}}
	suite.keeper, suite.ctx = keepertest.ActionKeeperWithAddress(suite.T(), ctrl, pairs)

	var err error
	suite.signature, err = cryptotestutils.CreateSignatureString([]secp256k1.PrivKey{key}, 50)
	suite.Require().NoError(err)

	// Setup test address
	suite.testAddr = address
	suite.testValAddr = sdk.ValAddress(suite.testAddr)

	// Set a fixed block time for testing
	suite.blockTime = time.Now()
	suite.ctx = suite.ctx.WithBlockTime(suite.blockTime)
}

// TestCheckExpiration tests the expiration functionality
func (suite *ExpirationTestSuite) TestCheckExpiration() {
	// Create test actions with different expiration times
	testCases := []struct {
		name           string
		expirationTime int64
		state          actiontypes.ActionState
		expectExpired  bool
	}{
		{
			name:           "Pending Action - Not Expired",
			expirationTime: suite.blockTime.Unix() + 3600, // 1 hour in the future
			state:          actiontypes.ActionStatePending,
			expectExpired:  false,
		},
		{
			name:           "Pending Action - Expired",
			expirationTime: suite.blockTime.Unix() - 3600, // 1 hour in the past
			state:          actiontypes.ActionStatePending,
			expectExpired:  true,
		},
		{
			name:           "Action Without Expiration Time",
			expirationTime: 0, // No expiration time
			state:          actiontypes.ActionStatePending,
			expectExpired:  false, // Should not expire
		},
		//{
		//	name:           "Processing Action - Not Expired",
		//	expirationTime: suite.blockTime.Unix() + 3600, // 1 hour in the future
		//	state:          actiontypes.ActionStateProcessing,
		//	expectExpired:  false,
		//},
		//{
		//	name:           "Processing Action - Expired",
		//	expirationTime: suite.blockTime.Unix() - 3600, // 1 hour in the past
		//	state:          actiontypes.ActionStateProcessing,
		//	expectExpired:  true,
		//},
		//{
		//	name:           "Done Action - Expired Time But Not Checked",
		//	expirationTime: suite.blockTime.Unix() - 3600, // 1 hour in the past
		//	state:          actiontypes.ActionStateDone,
		//	expectExpired:  false, // DONE actions shouldn't be checked for expiration
		//},
	}

	testPrice := sdk.NewInt64Coin("ulume", 10_100)
	// Create and store all the test actions
	for i, tc := range testCases {
		// Create cascade metadata
		cascadeMetadata := &actiontypes.CascadeMetadata{
			DataHash:   "test_hash",
			FileName:   "test.file",
			RqIdsIc:    5,
			RqIdsMax:   10,
			Signatures: suite.signature,
		}

		// Marshal metadata to bytes
		metadataBytes, err := suite.keeper.GetCodec().Marshal(cascadeMetadata)
		suite.Require().NoError(err, "Failed to marshal metadata for test case %d: %s", i, tc.name)

		// Create action
		action := &actiontypes.Action{
			Creator:        suite.testAddr.String(),
			ActionType:     actiontypes.ActionTypeCascade,
			Price:          &testPrice,
			BlockHeight:    suite.ctx.BlockHeight(),
			State:          tc.state,
			ExpirationTime: tc.expirationTime,
			Metadata:       metadataBytes,
		}

		// Register the action
		_, err = suite.keeper.RegisterAction(suite.ctx, action)
		suite.Require().NoError(err, "Failed to register action for test case %d: %s", i, tc.name)
	}

	// Run the expiration check
	suite.keeper.CheckExpiration(suite.ctx)

	// Verify each action's state after the expiration check
	var pendingCount, processingCount, expiredCount int

	err := suite.keeper.IterateActionsByState(suite.ctx, actiontypes.ActionStatePending, func(action *actiontypes.Action) bool {
		pendingCount++
		return false // Continue iteration
	})
	suite.Require().NoError(err)

	err = suite.keeper.IterateActionsByState(suite.ctx, actiontypes.ActionStateProcessing, func(action *actiontypes.Action) bool {
		processingCount++
		return false // Continue iteration
	})
	suite.Require().NoError(err)

	err = suite.keeper.IterateActionsByState(suite.ctx, actiontypes.ActionStateExpired, func(action *actiontypes.Action) bool {
		expiredCount++
		return false // Continue iteration
	})
	suite.Require().NoError(err)

	// Count expected number of expired/pending actions
	expectedExpiredCount := 0
	expectedPendingCount := 0
	expectedProcessingCount := 0

	for _, tc := range testCases {
		if tc.expectExpired {
			expectedExpiredCount++
		} else {
			if tc.state == actiontypes.ActionStatePending {
				expectedPendingCount++
			} else if tc.state == actiontypes.ActionStateProcessing {
				expectedProcessingCount++
			}
		}
	}

	// Verify counts match expected
	suite.Require().Equal(expectedExpiredCount, expiredCount, "Number of expired actions doesn't match expected")
	suite.Require().Equal(expectedPendingCount, pendingCount, "Number of pending actions doesn't match expected")
	suite.Require().Equal(expectedProcessingCount, processingCount, "Number of processing actions doesn't match expected")
}

// TestExpiredActionEvents tests that events are emitted for expired actions
func (suite *ExpirationTestSuite) TestExpiredActionEvents() {
	// Create a context with event manager to capture events
	ctx := suite.ctx.WithEventManager(sdk.NewEventManager())

	// Create sense metadata
	senseMetadata := &actiontypes.SenseMetadata{
		DataHash:             "expired_action_hash",
		DdAndFingerprintsMax: 10,
		DdAndFingerprintsIc:  5,
	}

	// Marshal metadata to bytes
	metadataBytes, err := suite.keeper.GetCodec().Marshal(senseMetadata)
	suite.Require().NoError(err)

	testPrice := sdk.NewInt64Coin("ulume", 10_100)

	// Create action
	expiredAction := &actiontypes.Action{
		Creator:        suite.testAddr.String(),
		ActionType:     actiontypes.ActionTypeSense,
		Price:          &testPrice,
		BlockHeight:    ctx.BlockHeight(),
		State:          actiontypes.ActionStatePending,
		ExpirationTime: ctx.BlockTime().Unix() - 3600, // 1 hour in the past
		Metadata:       metadataBytes,
	}

	// Register the action
	_, err = suite.keeper.RegisterAction(ctx, expiredAction)
	suite.Require().NoError(err)

	// Clear events from registration
	ctx = ctx.WithEventManager(sdk.NewEventManager())

	// Run expiration check
	suite.keeper.CheckExpiration(ctx)

	// Get events
	events := ctx.EventManager().Events()

	// Verify "action_expired" event was emitted
	foundExpiredEvent := false
	for _, event := range events {
		if event.Type == actiontypes.EventTypeActionExpired {
			foundExpiredEvent = true

			// Verify attributes
			for _, attr := range event.Attributes {
				if string(attr.Key) == "action_id" {
					suite.Require().Equal(expiredAction.ActionID, string(attr.Value))
				}
				if string(attr.Key) == "previous_state" {
					suite.Require().Equal("ACTION_STATE_PENDING", string(attr.Value))
				}
			}
		}
	}

	suite.Require().True(foundExpiredEvent, "action_expired event not found")
}

// Run the test suite
func TestExpirationTestSuite(t *testing.T) {
	suite.Run(t, new(ExpirationTestSuite))
}
