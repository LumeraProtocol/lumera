package keeper_test

import (
	"github.com/LumeraProtocol/lumera/x/action/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	actionapi "github.com/LumeraProtocol/lumera/api/lumera/action"
	"github.com/LumeraProtocol/lumera/testutil/cryptotestutils"
	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	"github.com/LumeraProtocol/lumera/x/action/keeper"
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
	key, address := cryptotestutils.KeyAndAddress()
	pubKey := key.PubKey()
	pairs := []keepertest.AccountPair{{Address: address, PubKey: pubKey}}
	suite.keeper, suite.ctx = keepertest.ActionKeeperWithAddress(suite.T(), pairs)

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
		state          actionapi.ActionState
		expectExpired  bool
	}{
		{
			name:           "Pending Action - Not Expired",
			expirationTime: suite.blockTime.Unix() + 3600, // 1 hour in the future
			state:          actionapi.ActionState_ACTION_STATE_PENDING,
			expectExpired:  false,
		},
		{
			name:           "Pending Action - Expired",
			expirationTime: suite.blockTime.Unix() - 3600, // 1 hour in the past
			state:          actionapi.ActionState_ACTION_STATE_PENDING,
			expectExpired:  true,
		},
		{
			name:           "Action Without Expiration Time",
			expirationTime: 0, // No expiration time
			state:          actionapi.ActionState_ACTION_STATE_PENDING,
			expectExpired:  false, // Should not expire
		},
		//{
		//	name:           "Processing Action - Not Expired",
		//	expirationTime: suite.blockTime.Unix() + 3600, // 1 hour in the future
		//	state:          actionapi.ActionState_ACTION_STATE_PROCESSING,
		//	expectExpired:  false,
		//},
		//{
		//	name:           "Processing Action - Expired",
		//	expirationTime: suite.blockTime.Unix() - 3600, // 1 hour in the past
		//	state:          actionapi.ActionState_ACTION_STATE_PROCESSING,
		//	expectExpired:  true,
		//},
		//{
		//	name:           "Done Action - Expired Time But Not Checked",
		//	expirationTime: suite.blockTime.Unix() - 3600, // 1 hour in the past
		//	state:          actionapi.ActionState_ACTION_STATE_DONE,
		//	expectExpired:  false, // DONE actions shouldn't be checked for expiration
		//},
	}

	// Create and store all the test actions
	for i, tc := range testCases {
		// Create cascade metadata
		cascadeMetadata := &actionapi.CascadeMetadata{
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
		action := &actionapi.Action{
			Creator:        suite.testAddr.String(),
			ActionType:     actionapi.ActionType_ACTION_TYPE_CASCADE,
			Price:          "100ulume",
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

	err := suite.keeper.IterateActionsByState(suite.ctx, actionapi.ActionState_ACTION_STATE_PENDING, func(action *actionapi.Action) bool {
		pendingCount++
		return false // Continue iteration
	})
	suite.Require().NoError(err)

	err = suite.keeper.IterateActionsByState(suite.ctx, actionapi.ActionState_ACTION_STATE_PROCESSING, func(action *actionapi.Action) bool {
		processingCount++
		return false // Continue iteration
	})
	suite.Require().NoError(err)

	err = suite.keeper.IterateActionsByState(suite.ctx, actionapi.ActionState_ACTION_STATE_EXPIRED, func(action *actionapi.Action) bool {
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
			if tc.state == actionapi.ActionState_ACTION_STATE_PENDING {
				expectedPendingCount++
			} else if tc.state == actionapi.ActionState_ACTION_STATE_PROCESSING {
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
	senseMetadata := &actionapi.SenseMetadata{
		DataHash:             "expired_action_hash",
		DdAndFingerprintsMax: 10,
		DdAndFingerprintsIc:  5,
	}

	// Marshal metadata to bytes
	metadataBytes, err := suite.keeper.GetCodec().Marshal(senseMetadata)
	suite.Require().NoError(err)

	// Create action
	expiredAction := &actionapi.Action{
		Creator:        suite.testAddr.String(),
		ActionType:     actionapi.ActionType_ACTION_TYPE_SENSE,
		Price:          "100ulume",
		BlockHeight:    ctx.BlockHeight(),
		State:          actionapi.ActionState_ACTION_STATE_PENDING,
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
		if event.Type == types.EventTypeActionExpired {
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
