package integration_test

import (
	"os"
	"testing"

	"cosmossdk.io/log"

	"github.com/LumeraProtocol/lumera/app"
	"github.com/LumeraProtocol/lumera/x/lumeraid/keeper"
	"github.com/LumeraProtocol/lumera/x/lumeraid/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type KeeperIntegrationSuite struct {
	suite.Suite
	app       *app.App
	ctx       sdk.Context
	keeper    keeper.Keeper
	authority sdk.AccAddress
}

// SetupSuite initializes the integration test suite
func (suite *KeeperIntegrationSuite) SetupSuite() {
	os.Setenv("SYSTEM_TESTS", "true")

	suite.app = app.Setup(suite.T())
	suite.ctx = suite.app.BaseApp.NewContext(true)

	suite.authority = authtypes.NewModuleAddress(govtypes.ModuleName)
	storeService := runtime.NewKVStoreService(suite.app.GetKey(types.StoreKey))

	suite.keeper = keeper.NewKeeper(
		suite.app.AppCodec(),
		storeService,
		suite.app.Logger(),
		suite.authority.String(),
		suite.app.BankKeeper,
		suite.app.AuthKeeper,
	)
}

// TearDownSuite cleans up after the test suite
func (suite *KeeperIntegrationSuite) TearDownSuite() {
	suite.app = nil
}

// TestGetAuthorityIntegration tests the GetAuthority method in an integration context
func (suite *KeeperIntegrationSuite) TestGetAuthorityIntegration() {
	require.Equal(suite.T(), suite.authority.String(), suite.keeper.GetAuthority(), "GetAuthority should return the correct authority address")
}

// TestLoggerIntegration tests the Logger method in an integration context
func (suite *KeeperIntegrationSuite) TestLoggerIntegration() {

	testCases := []struct {
		name   string
		logger log.Logger
	}{
		{"Using NopLogger", log.NewNopLogger()},
		{"Using standard Logger", log.NewLogger(os.Stdout)},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			suite.keeper = keeper.NewKeeper(
				suite.app.AppCodec(),
				runtime.NewKVStoreService(suite.app.GetKey(types.StoreKey)),
				tc.logger,
				suite.authority.String(),
				suite.app.BankKeeper,
				suite.app.AuthKeeper,
			)

			logger := suite.keeper.Logger()
			require.NotNil(suite.T(), logger, "Logger should not be nil")
		})
	}
}

func TestKeeperIntegration(t *testing.T) {
	suite.Run(t, new(KeeperIntegrationSuite))
}
