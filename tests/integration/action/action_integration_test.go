package action_test

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	lumeraapp "github.com/LumeraProtocol/lumera/app"
	"github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// ActionIntegrationTestSuite is a test suite to test action module integration
type ActionIntegrationTestSuite struct {
	suite.Suite

	app       *lumeraapp.App
	ctx       sdk.Context
	keeper    keeper.Keeper
	msgServer actiontypes.MsgServer

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
	ctx := app.BaseApp.NewContext(false).WithBlockHeight(1).WithBlockTime(time.Now())

	suite.app = app
	suite.ctx = ctx
	suite.keeper = app.ActionKeeper
	suite.msgServer = keeper.NewMsgServerImpl(suite.keeper)

	// Create test accounts
	//var baseAccounts []*authtypes.BaseAccount
	initCoins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 1_000_000))

	suite.testAddrs, suite.privKeys, _ = createTestAddAddrsWithKeys(5)
	for i, addr := range suite.testAddrs {
		acc := app.AuthKeeper.GetAccount(suite.ctx, addr)
		if acc == nil {
			account := app.AuthKeeper.NewAccountWithAddress(suite.ctx, addr)
			baseAcc := account.(*authtypes.BaseAccount)
			baseAcc.SetPubKey(suite.privKeys[i].PubKey())
			app.AuthKeeper.SetAccount(suite.ctx, baseAcc)
		}
		require.NoError(suite.T(), app.BankKeeper.MintCoins(suite.ctx, actiontypes.ModuleName, initCoins))
		require.NoError(suite.T(), app.BankKeeper.SendCoinsFromModuleToAccount(suite.ctx, actiontypes.ModuleName, addr, initCoins))
	}

	valAddr := sdk.ValAddress(suite.privKeys[1].PubKey().Address())
	sn := sntypes.SuperNode{
		ValidatorAddress: valAddr.String(),
		SupernodeAccount: suite.testAddrs[1].String(),
		Note:             "1.0.0",
		States:           []*sntypes.SuperNodeStateRecord{{State: sntypes.SuperNodeStateActive}},
		PrevIpAddresses:  []*sntypes.IPAddressHistory{{Address: "192.168.1.1"}},
		P2PPort:          "2134",
	}
	require.NoError(suite.T(), app.SupernodeKeeper.SetSuperNode(suite.ctx, sn))

	// Set default params
	params := actiontypes.DefaultParams()
	params.ExpirationDuration = time.Minute
	err := suite.keeper.SetParams(suite.ctx, params)
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
		msg := &actiontypes.MsgRequestAction{
			Creator:        txCreator,
			ActionType:     actiontypes.ActionTypeCascade.String(),
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

		msg := &actiontypes.MsgFinalizeAction{
			ActionId:   actionID,
			Creator:    sn,
			ActionType: actiontypes.ActionTypeCascade.String(),
			Metadata:   metadata,
		}
		_, err = suite.msgServer.FinalizeAction(suite.ctx, msg)
		require.NoError(suite.T(), err)
		action, found := suite.keeper.GetActionByID(suite.ctx, actionID)
		require.True(suite.T(), found)
		require.Equal(suite.T(), actiontypes.ActionStateDone, action.State)
	})

	suite.Run("Approve Cascade Action", func() {
		msg := &actiontypes.MsgApproveAction{
			ActionId: actionID,
			Creator:  txCreator,
		}
		_, err := suite.msgServer.ApproveAction(suite.ctx, msg)
		require.NoError(suite.T(), err)
		action, found := suite.keeper.GetActionByID(suite.ctx, actionID)
		require.True(suite.T(), found)
		require.Equal(suite.T(), actiontypes.ActionStateApproved, action.State)
	})
}

// TestActionExpiration verifies that pending/processing actions expire, emit events, and refund fees.
func (suite *ActionIntegrationTestSuite) TestActionExpiration() {
	params := suite.keeper.GetParams(suite.ctx)
	minPrice := params.BaseActionFee.Amount.Add(params.FeePerKbyte.Amount)
	price := sdk.NewCoin(params.BaseActionFee.Denom, minPrice.AddRaw(1_000))

	initialAccountBalance := suite.app.BankKeeper.GetBalance(suite.ctx, suite.testAddrs[0], price.Denom)
	initialModuleBalance := suite.app.BankKeeper.GetBalance(suite.ctx, actiontypes.ModuleAccountAddress, price.Denom)
	suite.True(initialModuleBalance.IsZero())

	sigStr, err := createValidCascadeSignatureString(suite.privKeys[0], 1)
	require.NoError(suite.T(), err)

	metadata := fmt.Sprintf(`{"data_hash":"expire_hash","file_name":"expire.dat","rq_ids_ic":1,"signatures":"%s"}`, sigStr)
	expiration := suite.ctx.BlockTime().Add(params.ExpirationDuration)

	msg := &actiontypes.MsgRequestAction{
		Creator:        suite.testAddrs[0].String(),
		ActionType:     actiontypes.ActionTypeCascade.String(),
		Metadata:       metadata,
		Price:          price.String(),
		ExpirationTime: fmt.Sprintf("%d", expiration.Unix()),
	}

	res, err := suite.msgServer.RequestAction(suite.ctx, msg)
	require.NoError(suite.T(), err)
	require.NotEmpty(suite.T(), res.ActionId)

	moduleBalanceAfterRequest := suite.app.BankKeeper.GetBalance(suite.ctx, actiontypes.ModuleAccountAddress, price.Denom)
	require.Equal(suite.T(), price, moduleBalanceAfterRequest)

	accountBalanceAfterRequest := suite.app.BankKeeper.GetBalance(suite.ctx, suite.testAddrs[0], price.Denom)
	require.True(suite.T(), accountBalanceAfterRequest.Amount.Equal(initialAccountBalance.Amount.Sub(price.Amount)))

	// advance block time beyond expiration and clear events before running end blocker
	suite.ctx = suite.ctx.WithBlockTime(expiration.Add(2 * time.Minute))
	suite.ctx = suite.ctx.WithEventManager(sdk.NewEventManager())

	require.NoError(suite.T(), suite.keeper.EndBlocker(suite.ctx))

	action, found := suite.keeper.GetActionByID(suite.ctx, res.ActionId)
	require.True(suite.T(), found)
	require.Equal(suite.T(), actiontypes.ActionStateExpired, action.State)

	moduleBalanceAfterExpiration := suite.app.BankKeeper.GetBalance(suite.ctx, actiontypes.ModuleAccountAddress, price.Denom)
	require.True(suite.T(), moduleBalanceAfterExpiration.IsZero())

	accountBalanceAfterExpiration := suite.app.BankKeeper.GetBalance(suite.ctx, suite.testAddrs[0], price.Denom)
	require.True(suite.T(), accountBalanceAfterExpiration.IsEqual(initialAccountBalance))

	events := suite.ctx.EventManager().Events()
	foundExpiredEvent := false
	for _, event := range events {
		if event.Type != actiontypes.EventTypeActionExpired {
			continue
		}
		foundExpiredEvent = true

		attrMap := make(map[string]string, len(event.Attributes))
		for _, attr := range event.Attributes {
			attrMap[string(attr.Key)] = string(attr.Value)
		}

		require.Equal(suite.T(), res.ActionId, attrMap[actiontypes.AttributeKeyActionID])
		require.Equal(suite.T(), suite.testAddrs[0].String(), attrMap[actiontypes.AttributeKeyCreator])
		require.Equal(suite.T(), actiontypes.ActionTypeCascade.String(), attrMap[actiontypes.AttributeKeyActionType])
	}
	require.True(suite.T(), foundExpiredEvent, "action_expired event not emitted")
}

func (suite *ActionIntegrationTestSuite) TestInvalidActionLifecycle() {
	suite.Run("Missing Metadata", func() {
		msg := &actiontypes.MsgRequestAction{
			Creator:    suite.testAddrs[0].String(),
			ActionType: actiontypes.ActionTypeSense.String(),
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
		action := &actiontypes.Action{
			ActionID: "8888",
			Creator:  suite.testAddrs[0].String(),
			State:    actiontypes.ActionStateApproved,
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
