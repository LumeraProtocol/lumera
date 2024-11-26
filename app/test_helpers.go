package app

import (
	"cosmossdk.io/depinject"
	"cosmossdk.io/log"
	"errors"
	"io"
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/runtime"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"
)

func NewTestApp(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	loadLatest bool,
	appOpts servertypes.AppOptions,
	baseAppOptions ...func(*baseapp.BaseApp),
) (*App, error) {
	var (
		app        = &App{ScopedKeepers: make(map[string]capabilitykeeper.ScopedKeeper)}
		appBuilder *runtime.AppBuilder

		// configure dependency injection with your app settings
		appConfig = depinject.Configs(
			AppConfig(),
			depinject.Supply(appOpts, logger, app.GetIBCKeeper, app.GetCapabilityScopedKeeper),
		)
	)

	if err := depinject.Inject(appConfig,
		&appBuilder,
		&app.appCodec,
		&app.legacyAmino,
		&app.txConfig,
		&app.interfaceRegistry,
		&app.AccountKeeper,
		&app.BankKeeper,
		&app.StakingKeeper,
		&app.DistrKeeper,
		&app.ConsensusParamsKeeper,
		&app.SlashingKeeper,
		&app.MintKeeper,
		&app.GovKeeper,
		&app.CrisisKeeper,
		&app.UpgradeKeeper,
		&app.ParamsKeeper,
		&app.AuthzKeeper,
		&app.EvidenceKeeper,
		&app.FeeGrantKeeper,
		&app.NFTKeeper,
		&app.GroupKeeper,
		&app.CircuitBreakerKeeper,
		&app.PastelidKeeper,
	); err != nil {
		return nil, err
	}

	// Ensure that BaseApp is initialized by building it with AppBuilder
	app.App = appBuilder.Build(db, traceStore, baseAppOptions...)
	if app.App == nil {
		return nil, errors.New("failed to initialize BaseApp")
	}

	// Additional configuration if needed, such as loading the latest version
	if loadLatest {
		if err := app.LoadLatestVersion(); err != nil {
			return nil, err
		}
	}

	return app, nil
}

// Setup initializes a new App instance for testing.
func Setup(t *testing.T) *App {
	// Initialize an in-memory database for testing
	db := dbm.NewMemDB()

	// Set up a no-op logger for testing to avoid logging clutter
	logger := log.NewNopLogger()

	// Configure any other test-specific options if needed

	// Instantiate the app with the in-memory database, no-op logger, and no trace store
	app, err := NewTestApp(logger, db, nil, true, EmptyAppOptions{})
	if err != nil {
		t.Fatalf("failed to initialize test app: %v", err)
	}

	return app
}

// EmptyAppOptions is a mock implementation of AppOptions with no options set.
type EmptyAppOptions struct{}

// Get implements the AppOptions interface but always returns nil for testing.
func (ao EmptyAppOptions) Get(_ string) interface{} {
	return nil
}
