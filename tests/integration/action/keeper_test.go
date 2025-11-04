package action_test

import (
    "encoding/base64"
    "fmt"
    "testing"
    "time"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	queryv1beta1 "github.com/cosmos/cosmos-sdk/types/query"

    gogoproto "github.com/cosmos/gogoproto/proto"
    "github.com/golang/mock/gomock"
    "github.com/stretchr/testify/suite"

	testkeeper "github.com/LumeraProtocol/lumera/testutil/keeper"
	actionkeeper "github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
)

// KeeperIntegrationTestSuite is a test suite to test keeper functions
type KeeperIntegrationTestSuite struct {
	suite.Suite

	ctx         sdk.Context
	keeper      actionkeeper.Keeper
	queryServer actiontypes.QueryServer

	testAddrs    []sdk.AccAddress
	testValAddrs []sdk.ValAddress
	testPubKeys  []cryptotypes.PubKey
	testPrivKeys []*secp256k1.PrivKey
}

// SetupTest sets up a test suite
func (suite *KeeperIntegrationTestSuite) SetupTest() {
	numAccounts := 5
	suite.testPubKeys = make([]cryptotypes.PubKey, numAccounts)
	suite.testPrivKeys = make([]*secp256k1.PrivKey, numAccounts)
	suite.testAddrs = make([]sdk.AccAddress, numAccounts)
	suite.testValAddrs = make([]sdk.ValAddress, numAccounts)

	for i := 0; i < numAccounts; i++ {
		privKey := secp256k1.GenPrivKey()
		suite.testPrivKeys[i] = privKey
		suite.testPubKeys[i] = privKey.PubKey()
		suite.testAddrs[i] = sdk.AccAddress(suite.testPubKeys[i].Address())
		suite.testValAddrs[i] = sdk.ValAddress(suite.testPubKeys[i].Address())
	}

	var accounts []testkeeper.AccountPair
	for i, addr := range suite.testAddrs {
		accounts = append(accounts, testkeeper.AccountPair{
			Address: addr,
			PubKey:  suite.testPubKeys[i],
		})
	}

	// gomock controller
	ctrl := gomock.NewController(suite.T())
	defer ctrl.Finish()

	k, ctx := testkeeper.ActionKeeperWithAddress(suite.T(), ctrl, accounts)
	suite.ctx = ctx
	suite.keeper = k
	suite.queryServer = actionkeeper.NewQueryServerImpl(k)

	// Ensure all test accounts have sufficient funds (5,000,000 ulume each)
	bankKeeper := k.GetBankKeeper()
	for _, addr := range suite.testAddrs {
		coins := sdk.NewCoins(sdk.NewCoin("ulume", math.NewInt(5000000)))
		err := bankKeeper.SendCoinsFromModuleToAccount(ctx, "bank", addr, coins)
		suite.Require().NoError(err)
	}
}

// TestGetAction tests the GetAction function
func (suite *KeeperIntegrationTestSuite) TestGetAction() {
	price := sdk.NewInt64Coin("ulume", 1_000_000)
	action := &actiontypes.Action{
		ActionID:       "",
		Creator:        suite.testAddrs[0].String(),
		ActionType:     actiontypes.ActionTypeSense,
		State:          actiontypes.ActionStatePending,
		Price:          price.String(),
		BlockHeight:    1,
		ExpirationTime: time.Now().Unix() + 3600,
		Metadata:       []byte(`{"key": "value"}`),
	}

	actionID, err := suite.keeper.RegisterAction(suite.ctx, action)
	suite.Require().NoError(err)
	suite.Require().NotEmpty(actionID)

	testCases := []struct {
		name          string
		actionId      string
		expectError   bool
		expectedState actiontypes.ActionState
	}{
		{
			name:          "Get existing action",
			actionId:      actionID,
			expectError:   false,
			expectedState: actiontypes.ActionState(actiontypes.ActionStatePending),
		},
		{
			name:          "Get non-existent action",
			actionId:      "non-existent",
			expectError:   true,
			expectedState: actiontypes.ActionState(actiontypes.ActionStateUnspecified),
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			req := &actiontypes.QueryGetActionRequest{
				ActionID: tc.actionId,
			}
			response, err := suite.queryServer.GetAction(suite.ctx, req)
			if tc.expectError {
				suite.Require().Error(err)
				suite.Require().Nil(response)
			} else {
				suite.Require().NoError(err)
				suite.Require().NotNil(response)
				suite.Require().Equal(tc.expectedState, actiontypes.ActionState(response.Action.State))
			}
		})
	}
}

// TestListActions tests the ListActions function
func (suite *KeeperIntegrationTestSuite) TestListActions() {
	senseMetadata := &actiontypes.SenseMetadata{
		DataHash:            "hash123",
		DdAndFingerprintsIc: 5,
	}
	senseMetadataBytes, err := gogoproto.Marshal(senseMetadata)
	suite.Require().NoError(err)

	signatureData := "base64data"
	signatureBytes, err := suite.testPrivKeys[1].Sign([]byte(signatureData))
	suite.Require().NoError(err)
	signature := base64.StdEncoding.EncodeToString(signatureBytes)
	cascadeMetadata := &actiontypes.CascadeMetadata{
		DataHash:   "hash456",
		FileName:   "test.file",
		RqIdsIc:    5,
		Signatures: fmt.Sprintf("%s.%s", signatureData, signature),
	}
	cascadeMetadataBytes, err := gogoproto.Marshal(cascadeMetadata)
	suite.Require().NoError(err)

	testPrice1 := sdk.NewInt64Coin("ulume", 1_000_000)
	testPrice2 := sdk.NewInt64Coin("ulume", 2_000_000)

	actions := []*actiontypes.Action{
		{
			ActionID:       "",
			Creator:        suite.testAddrs[0].String(),
			ActionType:     actiontypes.ActionTypeSense,
			State:          actiontypes.ActionStatePending,
			Price:          testPrice1.String(),
			BlockHeight:    1,
			ExpirationTime: time.Now().Unix() + 3600,
			Metadata:       senseMetadataBytes,
		},
		{
			ActionID:       "",
			Creator:        suite.testAddrs[1].String(),
			ActionType:     actiontypes.ActionTypeCascade,
			State:          actiontypes.ActionStatePending,
			Price:          testPrice2.String(),
			BlockHeight:    2,
			ExpirationTime: time.Now().Unix() + 3600,
			Metadata:       cascadeMetadataBytes,
		},
	}

	var actionIDs []string
	for _, action := range actions {
		actionID, err := suite.keeper.RegisterAction(suite.ctx, action)
		suite.Require().NoError(err)
		suite.Require().NotEmpty(actionID)
		actionIDs = append(actionIDs, actionID)
	}

	testCases := []struct {
		name          string
		actionType    actiontypes.ActionType
		actionState   actiontypes.ActionState
		expectedCount int
		pagination    *queryv1beta1.PageRequest
		expectError   bool
	}{
		{
			name:          "List all actions",
			actionType:    actiontypes.ActionTypeUnspecified,
			actionState:   actiontypes.ActionStateUnspecified,
			expectedCount: 2,
			pagination:    &queryv1beta1.PageRequest{Limit: 10},
			expectError:   false,
		},
		{
			name:          "List actions by type",
			actionType:    actiontypes.ActionTypeSense,
			actionState:   actiontypes.ActionStateUnspecified,
			expectedCount: 1,
			pagination:    &queryv1beta1.PageRequest{Limit: 10},
			expectError:   false,
		},
		{
			name:          "List actions by state",
			actionType:    actiontypes.ActionTypeUnspecified,
			actionState:   actiontypes.ActionStatePending,
			expectedCount: 2,
			pagination:    &queryv1beta1.PageRequest{Limit: 10},
			expectError:   false,
		},
		{
			name:          "List actions with pagination",
			actionType:    actiontypes.ActionTypeUnspecified,
			actionState:   actiontypes.ActionStateUnspecified,
			expectedCount: 1,
			pagination:    &queryv1beta1.PageRequest{Limit: 1},
			expectError:   false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			req := &actiontypes.QueryListActionsRequest{
				ActionType:  actiontypes.ActionType(tc.actionType),
				ActionState: actiontypes.ActionState(tc.actionState),
				Pagination:  tc.pagination,
			}
			response, err := suite.queryServer.ListActions(suite.ctx, req)
			if tc.expectError {
				suite.Require().Error(err)
				suite.Require().Nil(response)
			} else {
				suite.Require().NoError(err)
				suite.Require().NotNil(response)
				suite.Require().Len(response.Actions, tc.expectedCount)
			}
		})
	}
}

// TestListActionsBySuperNode tests the ListActionsBySuperNode function
func (suite *KeeperIntegrationTestSuite) TestListActionsBySuperNode() {
	senseMetadata := &actiontypes.SenseMetadata{
		DataHash:            "hash123",
		DdAndFingerprintsIc: 5,
	}
	senseMetadataBytes, err := gogoproto.Marshal(senseMetadata)
	suite.Require().NoError(err)

	signatureData := "base64data"
	signatureBytes, err := suite.testPrivKeys[1].Sign([]byte(signatureData))
	suite.Require().NoError(err)
	signature := base64.StdEncoding.EncodeToString(signatureBytes)
	cascadeMetadata := &actiontypes.CascadeMetadata{
		DataHash:   "hash456",
		FileName:   "test.file",
		RqIdsIc:    5,
		Signatures: fmt.Sprintf("%s.%s", signatureData, signature),
	}
	cascadeMetadataBytes, err := gogoproto.Marshal(cascadeMetadata)
	suite.Require().NoError(err)

	testPrice1 := sdk.NewInt64Coin("ulume", 1_000_000)
	testPrice2 := sdk.NewInt64Coin("ulume", 2_000_000)

	actions := []*actiontypes.Action{
		{
			ActionID:       "",
			Creator:        suite.testAddrs[0].String(),
			ActionType:     actiontypes.ActionTypeSense,
			State:          actiontypes.ActionStatePending,
			Price:          testPrice1.String(),
			BlockHeight:    1,
			ExpirationTime: time.Now().Unix() + 3600,
			Metadata:       senseMetadataBytes,
			SuperNodes:     []string{suite.testAddrs[0].String()},
		},
		{
			ActionID:       "",
			Creator:        suite.testAddrs[1].String(),
			ActionType:     actiontypes.ActionTypeCascade,
			State:          actiontypes.ActionStatePending,
			Price:          testPrice2.String(),
			BlockHeight:    2,
			ExpirationTime: time.Now().Unix() + 3600,
			Metadata:       cascadeMetadataBytes,
			SuperNodes:     []string{suite.testAddrs[1].String()},
		},
	}

	var actionIDs []string
	for _, action := range actions {
		actionID, err := suite.keeper.RegisterAction(suite.ctx, action)
		suite.Require().NoError(err)
		suite.Require().NotEmpty(actionID)
		actionIDs = append(actionIDs, actionID)
	}

	action2, found := suite.keeper.GetActionByID(suite.ctx, actionIDs[1])
	suite.Require().True(found)
	action2.State = actiontypes.ActionStateApproved
	err = suite.keeper.SetAction(suite.ctx, action2)
	suite.Require().NoError(err)

	testCases := []struct {
		name          string
		supernodeAddr string
		expectedCount int
		pagination    *queryv1beta1.PageRequest
		expectError   bool
	}{
		{
			name:          "List actions for existing supernode",
			supernodeAddr: suite.testAddrs[0].String(),
			expectedCount: 1,
			pagination:    &queryv1beta1.PageRequest{Limit: 10},
			expectError:   false,
		},
		{
			name:          "List actions for non-existent supernode",
			supernodeAddr: "non-existent",
			expectedCount: 0,
			pagination:    &queryv1beta1.PageRequest{Limit: 10},
			expectError:   false,
		},
		{
			name:          "List actions with pagination",
			supernodeAddr: suite.testAddrs[0].String(),
			expectedCount: 1,
			pagination:    &queryv1beta1.PageRequest{Limit: 1},
			expectError:   false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			req := &actiontypes.QueryListActionsBySuperNodeRequest{
				SuperNodeAddress: tc.supernodeAddr,
				Pagination:       tc.pagination,
			}
			response, err := suite.queryServer.ListActionsBySuperNode(suite.ctx, req)
			if tc.expectError {
				suite.Require().Error(err)
				suite.Require().Nil(response)
			} else {
				suite.Require().NoError(err)
				suite.Require().NotNil(response)
				suite.Require().Len(response.Actions, tc.expectedCount)
			}
		})
	}
}

// TestListActionsByBlockHeight tests the ListActionsByBlockHeight function
func (suite *KeeperIntegrationTestSuite) TestListActionsByBlockHeight() {
	header := suite.ctx.BlockHeader()
	header.Height = 1
	suite.ctx = suite.ctx.WithBlockHeader(header)

	senseMetadata := &actiontypes.SenseMetadata{
		DataHash:            "hash123",
		DdAndFingerprintsIc: 5,
	}
	senseMetadataBytes, err := gogoproto.Marshal(senseMetadata)
	suite.Require().NoError(err)

	signatureData := "base64data"
	signatureBytes, err := suite.testPrivKeys[1].Sign([]byte(signatureData))
	suite.Require().NoError(err)
	signature := base64.StdEncoding.EncodeToString(signatureBytes)
	cascadeMetadata := &actiontypes.CascadeMetadata{
		DataHash:   "hash456",
		FileName:   "test.file",
		RqIdsIc:    5,
		Signatures: fmt.Sprintf("%s.%s", signatureData, signature),
	}
	cascadeMetadataBytes, err := gogoproto.Marshal(cascadeMetadata)
	suite.Require().NoError(err)

	testPrice1 := sdk.NewInt64Coin("ulume", 1_000_000)
	testPrice2 := sdk.NewInt64Coin("ulume", 2_000_000)

	actions := []*actiontypes.Action{
		{
			ActionID:       "",
			Creator:        suite.testAddrs[0].String(),
			ActionType:     actiontypes.ActionTypeSense,
			State:          actiontypes.ActionStatePending,
			Price:          testPrice1.String(),
			BlockHeight:    1,
			ExpirationTime: time.Now().Unix() + 3600,
			Metadata:       senseMetadataBytes,
			SuperNodes:     []string{suite.testAddrs[0].String()},
		},
		{
			ActionID:       "",
			Creator:        suite.testAddrs[1].String(),
			ActionType:     actiontypes.ActionTypeCascade,
			State:          actiontypes.ActionStatePending,
			Price:          testPrice2.String(),
			BlockHeight:    2,
			ExpirationTime: time.Now().Unix() + 3600,
			Metadata:       cascadeMetadataBytes,
			SuperNodes:     []string{suite.testAddrs[1].String()},
		},
	}

	var actionIDs []string
	for _, action := range actions {
		header := suite.ctx.BlockHeader()
		header.Height = action.BlockHeight
		suite.ctx = suite.ctx.WithBlockHeader(header)

		actionID, err := suite.keeper.RegisterAction(suite.ctx, action)
		suite.Require().NoError(err)
		suite.Require().NotEmpty(actionID)
		actionIDs = append(actionIDs, actionID)
	}

	action2, found := suite.keeper.GetActionByID(suite.ctx, actionIDs[1])
	suite.Require().True(found)
	action2.State = actiontypes.ActionStateApproved
	err = suite.keeper.SetAction(suite.ctx, action2)
	suite.Require().NoError(err)

	testCases := []struct {
		name          string
		blockHeight   int64
		expectedCount int
		pagination    *queryv1beta1.PageRequest
		expectError   bool
	}{
		{
			name:          "List actions for block height 1",
			blockHeight:   1,
			expectedCount: 1,
			pagination:    &queryv1beta1.PageRequest{Limit: 10},
			expectError:   false,
		},
		{
			name:          "List actions for block height 2",
			blockHeight:   2,
			expectedCount: 1,
			pagination:    &queryv1beta1.PageRequest{Limit: 10},
			expectError:   false,
		},
		{
			name:          "List actions for non-existent block height",
			blockHeight:   3,
			expectedCount: 0,
			pagination:    &queryv1beta1.PageRequest{Limit: 10},
			expectError:   false,
		},
		{
			name:          "List actions with pagination",
			blockHeight:   1,
			expectedCount: 1,
			pagination:    &queryv1beta1.PageRequest{Limit: 1},
			expectError:   false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			req := &actiontypes.QueryListActionsByBlockHeightRequest{
				BlockHeight: tc.blockHeight,
				Pagination:  tc.pagination,
			}
			response, err := suite.queryServer.ListActionsByBlockHeight(suite.ctx, req)
			if tc.expectError {
				suite.Require().Error(err)
				suite.Require().Nil(response)
			} else {
				suite.Require().NoError(err)
				suite.Require().NotNil(response)
				suite.Require().Len(response.Actions, tc.expectedCount)
			}
		})
	}
}

// TestListExpiredActions tests the ListExpiredActions function
func (suite *KeeperIntegrationTestSuite) TestListExpiredActions() {
	senseMetadata := &actiontypes.SenseMetadata{
		DataHash:            "hash123",
		DdAndFingerprintsIc: 5,
	}
	senseMetadataBytes, err := gogoproto.Marshal(senseMetadata)
	suite.Require().NoError(err)

	signatureData := "base64data"
	signatureBytes, err := suite.testPrivKeys[1].Sign([]byte(signatureData))
	suite.Require().NoError(err)
	signature := base64.StdEncoding.EncodeToString(signatureBytes)
	cascadeMetadata := &actiontypes.CascadeMetadata{
		DataHash:   "hash456",
		FileName:   "test.file",
		RqIdsIc:    5,
		Signatures: fmt.Sprintf("%s.%s", signatureData, signature),
	}
	cascadeMetadataBytes, err := gogoproto.Marshal(cascadeMetadata)
	suite.Require().NoError(err)

	testPrice1 := sdk.NewInt64Coin("ulume", 1_000_000)
	testPrice2 := sdk.NewInt64Coin("ulume", 2_000_000)

	now := time.Now().Unix()
	actions := []*actiontypes.Action{
		{
			ActionID:       "",
			Creator:        suite.testAddrs[0].String(),
			ActionType:     actiontypes.ActionTypeSense,
			State:          actiontypes.ActionStatePending,
			Price:          testPrice1.String(),
			BlockHeight:    1,
			ExpirationTime: now - 3600,
			Metadata:       senseMetadataBytes,
			SuperNodes:     []string{suite.testAddrs[0].String()},
		},
		{
			ActionID:       "",
			Creator:        suite.testAddrs[1].String(),
			ActionType:     actiontypes.ActionTypeCascade,
			State:          actiontypes.ActionStatePending,
			Price:          testPrice2.String(),
			BlockHeight:    2,
			ExpirationTime: now - 7200,
			Metadata:       cascadeMetadataBytes,
			SuperNodes:     []string{suite.testAddrs[1].String()},
		},
	}

	var actionIDs []string
	for _, action := range actions {
		actionID, err := suite.keeper.RegisterAction(suite.ctx, action)
		suite.Require().NoError(err)
		suite.Require().NotEmpty(actionID)
		actionIDs = append(actionIDs, actionID)

		actionObj, found := suite.keeper.GetActionByID(suite.ctx, actionID)
		suite.Require().True(found)
		actionObj.State = actiontypes.ActionStateExpired
		err = suite.keeper.SetAction(suite.ctx, actionObj)
		suite.Require().NoError(err)
	}

	testCases := []struct {
		name          string
		expectedCount int
		pagination    *queryv1beta1.PageRequest
		expectError   bool
	}{
		{
			name:          "List expired actions",
			expectedCount: 2,
			pagination:    &queryv1beta1.PageRequest{Limit: 10},
			expectError:   false,
		},
		{
			name:          "List expired actions with pagination",
			expectedCount: 1,
			pagination:    &queryv1beta1.PageRequest{Limit: 1},
			expectError:   false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			req := &actiontypes.QueryListExpiredActionsRequest{
				Pagination: tc.pagination,
			}
			response, err := suite.queryServer.ListExpiredActions(suite.ctx, req)
			if tc.expectError {
				suite.Require().Error(err)
				suite.Require().Nil(response)
			} else {
				suite.Require().NoError(err)
				suite.Require().NotNil(response)
				suite.Require().Len(response.Actions, tc.expectedCount)
			}
		})
	}
}

// TestQueryActionByMetadata tests the QueryActionByMetadata function
func (suite *KeeperIntegrationTestSuite) TestQueryActionByMetadata() {
	senseMetadata := &actiontypes.SenseMetadata{
		DataHash:            "hash123",
		DdAndFingerprintsIc: 5,
	}
	senseMetadataBytes, err := gogoproto.Marshal(senseMetadata)
	suite.Require().NoError(err)

	signatureData := "base64data"
	signatureBytes, err := suite.testPrivKeys[1].Sign([]byte(signatureData))
	suite.Require().NoError(err)
	signature := base64.StdEncoding.EncodeToString(signatureBytes)
	cascadeMetadata := &actiontypes.CascadeMetadata{
		DataHash:   "hash456",
		FileName:   "test.file",
		RqIdsIc:    5,
		Signatures: fmt.Sprintf("%s.%s", signatureData, signature),
	}
	cascadeMetadataBytes, err := gogoproto.Marshal(cascadeMetadata)
	suite.Require().NoError(err)

	testPrice1 := sdk.NewInt64Coin("ulume", 1_000_000)
	testPrice2 := sdk.NewInt64Coin("ulume", 2_000_000)

	actions := []*actiontypes.Action{
		{
			ActionID:       "",
			Creator:        suite.testAddrs[0].String(),
			ActionType:     actiontypes.ActionTypeSense,
			State:          actiontypes.ActionStatePending,
			Price:          testPrice1.String(),
			BlockHeight:    1,
			ExpirationTime: time.Now().Unix() + 3600,
			Metadata:       senseMetadataBytes,
		},
		{
			ActionID:       "",
			Creator:        suite.testAddrs[1].String(),
			ActionType:     actiontypes.ActionTypeCascade,
			State:          actiontypes.ActionStatePending,
			Price:          testPrice2.String(),
			BlockHeight:    2,
			ExpirationTime: time.Now().Unix() + 3600,
			Metadata:       cascadeMetadataBytes,
		},
	}

	var actionIDs []string
	for _, action := range actions {
		actionID, err := suite.keeper.RegisterAction(suite.ctx, action)
		suite.Require().NoError(err)
		suite.Require().NotEmpty(actionID)
		actionIDs = append(actionIDs, actionID)
	}

	testCases := []struct {
		name          string
		actionType    actiontypes.ActionType
		key           string
		value         string
		expectedCount int
		pagination    *queryv1beta1.PageRequest
		expectError   bool
	}{
		{
			name:          "Query actions by metadata key-value",
			actionType:    actiontypes.ActionTypeSense,
			key:           "data_hash",
			value:         "hash123",
			expectedCount: 1,
			pagination:    &queryv1beta1.PageRequest{Limit: 10},
			expectError:   false,
		},
		{
			name:          "Query actions by non-existent metadata",
			actionType:    actiontypes.ActionTypeSense,
			key:           "data_hash",
			value:         "nonexistent",
			expectedCount: 0,
			pagination:    &queryv1beta1.PageRequest{Limit: 10},
			expectError:   false,
		},
		{
			name:          "Query actions with pagination",
			actionType:    actiontypes.ActionTypeCascade,
			key:           "file_name",
			value:         "test.file",
			expectedCount: 1,
			pagination:    &queryv1beta1.PageRequest{Limit: 1},
			expectError:   false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			req := &actiontypes.QueryActionByMetadataRequest{
				ActionType:    actiontypes.ActionType(tc.actionType),
				MetadataQuery: fmt.Sprintf("%s=%s", tc.key, tc.value),
				Pagination:    tc.pagination,
			}
			response, err := suite.queryServer.QueryActionByMetadata(suite.ctx, req)
			if tc.expectError {
				suite.Require().Error(err)
				suite.Require().Nil(response)
			} else {
				suite.Require().NoError(err)
				suite.Require().NotNil(response)
				suite.Require().Len(response.Actions, tc.expectedCount)
				if tc.expectedCount > 0 {
					if tc.actionType == actiontypes.ActionTypeSense {
						var metadata actiontypes.SenseMetadata
						err = gogoproto.Unmarshal(response.Actions[0].Metadata, &metadata)
						suite.Require().NoError(err)
						if tc.key == "data_hash" {
							suite.Require().Equal(tc.value, metadata.DataHash)
						}
					} else {
						var metadata actiontypes.CascadeMetadata
						err = gogoproto.Unmarshal(response.Actions[0].Metadata, &metadata)
						suite.Require().NoError(err)
						if tc.key == "file_name" {
							suite.Require().Equal(tc.value, metadata.FileName)
						}
					}
				}
			}
		})
	}
}

func (suite *KeeperIntegrationTestSuite) TestGetActionFee() {
	params := suite.keeper.GetParams(suite.ctx)

	// Override with known values for testing
	params.BaseActionFee = sdk.NewCoin("ulume", math.NewInt(10000))
	params.FeePerKbyte = sdk.NewCoin("ulume", math.NewInt(100))
	suite.keeper.SetParams(suite.ctx, params)

	testCases := []struct {
		name        string
		dataSize    string
		expectErr   bool
		expectedFee string
	}{
		{
			name:        "valid request with zero data",
			dataSize:    "0",
			expectedFee: "10000", // Only base fee
		},
		{
			name:        "valid request with 200 bytes",
			dataSize:    "200",
			expectedFee: "30000", // 200*100 + 10000
		},
		{
			name:      "invalid dataSize string",
			dataSize:  "invalid",
			expectErr: true,
		},
		{
			name:      "empty dataSize string",
			dataSize:  "",
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			req := &actiontypes.QueryGetActionFeeRequest{
				DataSize: tc.dataSize,
			}
			resp, err := suite.queryServer.GetActionFee(suite.ctx, req)

			if tc.expectErr {
				suite.Require().Error(err)
				suite.Require().Nil(resp)
			} else {
				suite.Require().NoError(err)
				suite.Require().NotNil(resp)
				suite.Require().Equal(tc.expectedFee, resp.Amount)
			}
		})
	}
}

func TestKeeperIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(KeeperIntegrationTestSuite))
}
