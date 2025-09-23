package action_test

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	actionapi "github.com/LumeraProtocol/lumera/api/lumera/action"
	lumeraapp "github.com/LumeraProtocol/lumera/app"
	"github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	types2 "github.com/LumeraProtocol/lumera/x/action/v1/types"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// ActionIntegrationTestSuite is a test suite to test action module integration
type ActionIntegrationTestSuite struct {
	suite.Suite

	ctx       sdk.Context
	keeper    keeper.Keeper
	msgServer types2.MsgServer

	// Test accounts for simulation
	testAddrs    []sdk.AccAddress
	testValAddrs []sdk.ValAddress
	privKeys     []*secp256k1.PrivKey
}

// SetupTest sets up a test suite

func (suite *ActionIntegrationTestSuite) SetupTest() {
	// Setup would normally create a test keeper and context
	// For now we just create empty structs since we're only setting up the test structure
	app := lumeraapp.Setup(suite.T()) // Proper app initialization
	ctx := app.BaseApp.NewContext(false).WithBlockHeight(1)

	suite.ctx = ctx
	suite.keeper = app.ActionKeeper
	suite.msgServer = keeper.NewMsgServerImpl(suite.keeper)

	// Create test accounts
	//var baseAccounts []*authtypes.BaseAccount
	initCoins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 1_000_000))

	suite.testAddrs, suite.privKeys, _ = createTestAddAddrsWithKeys(5)
	for i, addr := range suite.testAddrs {
		acc := app.AccountKeeper.GetAccount(suite.ctx, addr)
		if acc == nil {
			account := app.AccountKeeper.NewAccountWithAddress(suite.ctx, addr)
			baseAcc := account.(*authtypes.BaseAccount)
			baseAcc.SetPubKey(suite.privKeys[i].PubKey())
			app.AccountKeeper.SetAccount(suite.ctx, baseAcc)
		}
		require.NoError(suite.T(), app.BankKeeper.MintCoins(suite.ctx, types2.ModuleName, initCoins))
		require.NoError(suite.T(), app.BankKeeper.SendCoinsFromModuleToAccount(suite.ctx, types2.ModuleName, addr, initCoins))
	}

	valAddr := sdk.ValAddress(suite.privKeys[1].PubKey().Address())
	sn := types.SuperNode{
		ValidatorAddress: valAddr.String(),
		SupernodeAccount: suite.testAddrs[1].String(),
		Note:             "1.0.0",
		States:           []*types.SuperNodeStateRecord{{State: types.SuperNodeStateActive}},
		PrevIpAddresses:  []*types.IPAddressHistory{{Address: "192.168.1.1"}},
		P2PPort:          "2134",
	}
	require.NoError(suite.T(), app.SupernodeKeeper.SetSuperNode(suite.ctx, sn))

	// Set default params
	err := suite.keeper.SetParams(ctx, types2.DefaultParams())
	require.NoError(suite.T(), err)
}

// createTestAddrs creates test addresses
func createTestAddrs(numAddrs int) []sdk.AccAddress {
	addrs := make([]sdk.AccAddress, numAddrs)
	for i := 0; i < numAddrs; i++ {
		addr := make([]byte, 20)
		addr[0] = byte(i)
		addrs[i] = sdk.AccAddress(addr)
	}
	return addrs
}

func createTestAddAddrsWithKeys(num int) ([]sdk.AccAddress, []*secp256k1.PrivKey, []*authtypes.BaseAccount) {
	addrs := make([]sdk.AccAddress, num)
	privs := make([]*secp256k1.PrivKey, num)
	baseAccounts := make([]*authtypes.BaseAccount, num)

	for i := 0; i < num; i++ {
		priv := secp256k1.GenPrivKey()
		pubKey := priv.PubKey()

		baseAcc, _ := authtypes.NewBaseAccountWithPubKey(pubKey)
		addrs[i] = baseAcc.GetAddress()
		privs[i] = priv
		baseAccounts[i] = baseAcc
	}
	return addrs, privs, baseAccounts
}

// createTestValAddrs creates test validator addresses
func createTestValAddrs(numAddrs int) []sdk.ValAddress {
	addrs := make([]sdk.ValAddress, numAddrs)
	for i := 0; i < numAddrs; i++ {
		addr := make([]byte, 20)
		addr[0] = byte(i)
		addrs[i] = sdk.ValAddress(addr)
	}
	return addrs
}

// TestActionLifecycle tests the full action lifecycle
func (suite *ActionIntegrationTestSuite) TestActionLifecycle() {
	txCreator := suite.testAddrs[0].String()
	sn := suite.testAddrs[1].String() // Simulated supernode account

	var actionID string
	//var signatureDataPart string

	suite.Run("Request Cascade Action", func() {
		sigStr, err := createValidCascadeSignatureString(suite.privKeys[0], 1)
		require.NoError(suite.T(), err)

		//signatureDataPart = strings.Split(sigStr, ".")[0] // <-- Add this

		metadata := fmt.Sprintf(`{"data_hash":"abc123","file_name":"file.txt","rq_ids_ic":1,"signatures":"%s"}`, sigStr)
		msg := &types2.MsgRequestAction{
			Creator:        txCreator,
			ActionType:     actionapi.ActionType_ACTION_TYPE_CASCADE.String(),
			Metadata:       metadata,
			Price:          "100000ulume",
			ExpirationTime: fmt.Sprintf("%d", time.Now().Add(10*time.Minute).Unix()),
		}
		res, err := suite.msgServer.RequestAction(suite.ctx, msg)
		require.NoError(suite.T(), err)
		actionID = res.ActionId
		require.NotEmpty(suite.T(), actionID)
	})

	suite.Run("Finalize Cascade Action", func() {
		sigStr, err := createValidCascadeSignatureString(suite.privKeys[0], 1)
		require.NoError(suite.T(), err)

		ids, err := generateValidCascadeIDs(sigStr, 1, 50) // pass full sigStr here
		require.NoError(suite.T(), err)

		metadata := fmt.Sprintf(`{"rq_ids_ids":%s}`, toJSONStringArray(ids))

		msg := &types2.MsgFinalizeAction{
			ActionId:   actionID,
			Creator:    sn,
			ActionType: actionapi.ActionType_ACTION_TYPE_CASCADE.String(),
			Metadata:   metadata,
		}
		_, err = suite.msgServer.FinalizeAction(suite.ctx, msg)
		require.NoError(suite.T(), err)
		action, found := suite.keeper.GetActionByID(suite.ctx, actionID)
		require.True(suite.T(), found)
		require.Equal(suite.T(), actionapi.ActionState_ACTION_STATE_DONE, action.State)
	})

	suite.Run("Approve Cascade Action", func() {
		msg := &types2.MsgApproveAction{
			ActionId: actionID,
			Creator:  txCreator,
		}
		_, err := suite.msgServer.ApproveAction(suite.ctx, msg)
		require.NoError(suite.T(), err)
		action, found := suite.keeper.GetActionByID(suite.ctx, actionID)
		require.True(suite.T(), found)
		require.Equal(suite.T(), actionapi.ActionState_ACTION_STATE_APPROVED, action.State)
	})
}

func (suite *ActionIntegrationTestSuite) TestInvalidActionLifecycle() {
	suite.Run("Missing Metadata", func() {
		msg := &types2.MsgRequestAction{
			Creator:    suite.testAddrs[0].String(),
			ActionType: actionapi.ActionType_ACTION_TYPE_SENSE.String(),
			Metadata:   "",
			Price:      "10token",
		}
		_, err := suite.msgServer.RequestAction(suite.ctx, msg)
		require.Error(suite.T(), err)
	})

	suite.Run("Unauthorized Approval", func() {
		err := suite.keeper.ApproveAction(suite.ctx, "9999", suite.testAddrs[1].String())
		require.Error(suite.T(), err)
	})

	suite.Run("Invalid State Transition", func() {
		action := &actionapi.Action{
			ActionID: "8888",
			Creator:  suite.testAddrs[0].String(),
			State:    actionapi.ActionState_ACTION_STATE_APPROVED,
		}
		suite.keeper.SetAction(suite.ctx, action)
		err := suite.keeper.ApproveAction(suite.ctx, action.ActionID, action.Creator)
		require.Error(suite.T(), err)
	})
}

func TestActionIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(ActionIntegrationTestSuite))
}

func createValidCascadeSignatureString(priv *secp256k1.PrivKey, ic int) (string, error) {
	rawData := fmt.Sprintf("rqid-%d", ic)
	dataBase64 := base64.StdEncoding.EncodeToString([]byte(rawData))

	sig, err := priv.Sign([]byte(dataBase64)) // sign base64 ONCE
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s",
		dataBase64,
		base64.StdEncoding.EncodeToString(sig),
	), nil
}

func toJSONStringArray(values []string) string {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = fmt.Sprintf(`"%s"`, v)
	}
	return fmt.Sprintf("[%s]", strings.Join(quoted, ","))
}

func generateValidCascadeIDs(signature string, ic, count int) ([]string, error) {
	var ids []string
	for i := 0; i < count; i++ {
		id, err := keeper.CreateKademliaID(signature, uint64(ic+i))
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}
