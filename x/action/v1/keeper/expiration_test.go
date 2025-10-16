package keeper_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"

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

// TestActionFeeRefundOnExpiration ensures that action fees are refunded when actions expire
func (suite *ExpirationTestSuite) TestActionFeeRefundOnExpiration() {
	bankKeeper, ok := suite.keeper.GetBankKeeper().(*keepertest.ActionBankKeeper)
	suite.Require().True(ok)

	initialBalance := bankKeeper.GetAccountCoins(suite.testAddr)
	expectedInitial := sdk.NewCoins(sdk.NewInt64Coin("ulume", keepertest.TestAccountAmount))
	suite.Require().Equal(expectedInitial, initialBalance)
	suite.Require().True(bankKeeper.GetModuleBalance(actiontypes.ModuleName).IsZero())

	params := suite.keeper.GetParams(suite.ctx)
	minPriceAmount := params.BaseActionFee.Amount.Add(params.FeePerKbyte.Amount)
	priceDenom := params.BaseActionFee.Denom

	scenarios := []struct {
		name  string
		state actiontypes.ActionState
	}{
		{name: "pending", state: actiontypes.ActionStatePending},
		{name: "processing", state: actiontypes.ActionStateProcessing},
	}

	for i, scenario := range scenarios {
		// ensure module balance clean before starting scenario
		suite.Require().True(bankKeeper.GetModuleBalance(actiontypes.ModuleName).IsZero())

		price := sdk.NewCoin(priceDenom, minPriceAmount.AddRaw(int64(i*1_000)))

		cascadeMetadata := &actiontypes.CascadeMetadata{
			DataHash:   fmt.Sprintf("refund-hash-%d", i),
			FileName:   fmt.Sprintf("refund-file-%d", i),
			RqIdsIc:    1,
			RqIdsMax:   2,
			Signatures: suite.signature,
		}

		metadataBytes, err := suite.keeper.GetCodec().Marshal(cascadeMetadata)
		suite.Require().NoError(err)

		action := &actiontypes.Action{
			Creator:        suite.testAddr.String(),
			ActionType:     actiontypes.ActionTypeCascade,
			Price:          &price,
			ExpirationTime: suite.blockTime.Unix() - 10,
			Metadata:       metadataBytes,
		}

		_, err = suite.keeper.RegisterAction(suite.ctx, action)
		suite.Require().NoError(err, "scenario %s failed to register action", scenario.name)

		suite.Equal(sdk.NewCoins(price), bankKeeper.GetModuleBalance(actiontypes.ModuleName))

		expectedAfterFee := initialBalance.Sub(sdk.NewCoins(price)...)
		suite.Equal(expectedAfterFee, bankKeeper.GetAccountCoins(suite.testAddr))

		stored, found := suite.keeper.GetActionByID(suite.ctx, action.ActionID)
		suite.Require().True(found)

		if scenario.state == actiontypes.ActionStateProcessing {
			stored.State = actiontypes.ActionStateProcessing
			err = suite.keeper.SetAction(suite.ctx, stored)
			suite.Require().NoError(err)
		}

		suite.keeper.CheckExpiration(suite.ctx)

		updated, found := suite.keeper.GetActionByID(suite.ctx, action.ActionID)
		suite.Require().True(found)
		suite.Equal(actiontypes.ActionStateExpired, updated.State)

		suite.True(bankKeeper.GetModuleBalance(actiontypes.ModuleName).IsZero())
		suite.Equal(initialBalance, bankKeeper.GetAccountCoins(suite.testAddr))
	}
}

// TestExpiredActionEvents tests that events are emitted for expired actions
func (suite *ExpirationTestSuite) TestExpiredActionEvents() {
	params := suite.keeper.GetParams(suite.ctx)
	minPriceAmount := params.BaseActionFee.Amount.Add(params.FeePerKbyte.Amount)
	priceDenom := params.BaseActionFee.Denom

	scenarios := []struct {
		name      string
		state     actiontypes.ActionState
		actionTyp actiontypes.ActionType
	}{
		{name: "pending", state: actiontypes.ActionStatePending, actionTyp: actiontypes.ActionTypeSense},
		{name: "processing", state: actiontypes.ActionStateProcessing, actionTyp: actiontypes.ActionTypeCascade},
	}

	for i, scenario := range scenarios {
		ctx := suite.ctx.WithEventManager(sdk.NewEventManager())

		var metadataBytes []byte
		var err error
		if scenario.actionTyp == actiontypes.ActionTypeCascade {
			metadata := &actiontypes.CascadeMetadata{
				DataHash:   fmt.Sprintf("event-cascade-hash-%d", i),
				FileName:   fmt.Sprintf("event-cascade-file-%d", i),
				RqIdsIc:    1,
				RqIdsMax:   2,
				Signatures: suite.signature,
			}
			metadataBytes, err = suite.keeper.GetCodec().Marshal(metadata)
		} else {
			metadata := &actiontypes.SenseMetadata{
				DataHash:             fmt.Sprintf("event-sense-hash-%d", i),
				DdAndFingerprintsIc:  1,
				DdAndFingerprintsMax: 2,
			}
			metadataBytes, err = suite.keeper.GetCodec().Marshal(metadata)
		}
		suite.Require().NoError(err)

		price := sdk.NewCoin(priceDenom, minPriceAmount.AddRaw(int64(i*500)))

		action := &actiontypes.Action{
			Creator:        suite.testAddr.String(),
			ActionType:     scenario.actionTyp,
			Price:          &price,
			ExpirationTime: ctx.BlockTime().Unix() - 60,
			Metadata:       metadataBytes,
		}

		actionID, err := suite.keeper.RegisterAction(ctx, action)
		suite.Require().NoError(err, "scenario %s failed to register action", scenario.name)

		stored, found := suite.keeper.GetActionByID(ctx, actionID)
		suite.Require().True(found)
		if scenario.state == actiontypes.ActionStateProcessing {
			stored.State = actiontypes.ActionStateProcessing
			err = suite.keeper.SetAction(ctx, stored)
			suite.Require().NoError(err)
		}

		ctx = ctx.WithEventManager(sdk.NewEventManager())

		suite.keeper.CheckExpiration(ctx)

		events := ctx.EventManager().Events()
		foundExpiredEvent := false

		for _, event := range events {
			if event.Type != actiontypes.EventTypeActionExpired {
				continue
			}
			foundExpiredEvent = true

			attrs := make(map[string]string)
			for _, attr := range event.Attributes {
				attrs[string(attr.Key)] = string(attr.Value)
			}

			suite.Equal(actionID, attrs[actiontypes.AttributeKeyActionID])
			suite.Equal(suite.testAddr.String(), attrs[actiontypes.AttributeKeyCreator])
			suite.Equal(scenario.actionTyp.String(), attrs[actiontypes.AttributeKeyActionType])
		}

		suite.Require().True(foundExpiredEvent, "scenario %s did not emit action_expired event", scenario.name)
	}
}

// Run the test suite
func TestExpirationTestSuite(t *testing.T) {
	suite.Run(t, new(ExpirationTestSuite))
}
