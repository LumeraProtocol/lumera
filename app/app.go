package app

import (
	"io"

	_ "cosmossdk.io/api/cosmos/tx/config/v1" // import for side-effects
	clienthelpers "cosmossdk.io/client/v2/helpers"
	"cosmossdk.io/depinject"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	_ "cosmossdk.io/x/circuit" // import for side-effects
	circuitkeeper "cosmossdk.io/x/circuit/keeper"
	_ "cosmossdk.io/x/evidence" // import for side-effects
	evidencekeeper "cosmossdk.io/x/evidence/keeper"
	feegrantkeeper "cosmossdk.io/x/feegrant/keeper"
	_ "cosmossdk.io/x/feegrant/module" // import for side-effects
	nftkeeper "cosmossdk.io/x/nft/keeper"
	_ "cosmossdk.io/x/nft/module" // import for side-effects
	_ "cosmossdk.io/x/upgrade"    // import for side-effects
	upgradekeeper "cosmossdk.io/x/upgrade/keeper"
	abci "github.com/cometbft/cometbft/abci/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/server/api"
	"github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authsims "github.com/cosmos/cosmos-sdk/x/auth/simulation"
	_ "github.com/cosmos/cosmos-sdk/x/auth/tx/config" // import for side-effects
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	_ "github.com/cosmos/cosmos-sdk/x/auth/vesting" // import for side-effects
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	_ "github.com/cosmos/cosmos-sdk/x/authz/module" // import for side-effects
	_ "github.com/cosmos/cosmos-sdk/x/bank"         // import for side-effects
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	_ "github.com/cosmos/cosmos-sdk/x/consensus" // import for side-effects
	consensuskeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	_ "github.com/cosmos/cosmos-sdk/x/crisis" // import for side-effects
	crisiskeeper "github.com/cosmos/cosmos-sdk/x/crisis/keeper"
	_ "github.com/cosmos/cosmos-sdk/x/distribution" // import for side-effects
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govclient "github.com/cosmos/cosmos-sdk/x/gov/client"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	groupkeeper "github.com/cosmos/cosmos-sdk/x/group/keeper"
	_ "github.com/cosmos/cosmos-sdk/x/group/module" // import for side-effects
	_ "github.com/cosmos/cosmos-sdk/x/mint"         // import for side-effects
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	_ "github.com/cosmos/cosmos-sdk/x/params" // import for side-effects
	paramsclient "github.com/cosmos/cosmos-sdk/x/params/client"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	_ "github.com/cosmos/cosmos-sdk/x/slashing" // import for side-effects
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	_ "github.com/cosmos/cosmos-sdk/x/staking" // import for side-effects
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	_ "github.com/cosmos/ibc-go/modules/capability" // import for side-effects
	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"
	_ "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts" // import for side-effects
	icacontrollerkeeper "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/controller/keeper"
	icahostkeeper "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host/keeper"
	_ "github.com/cosmos/ibc-go/v8/modules/apps/29-fee" // import for side-effects
	ibcfeekeeper "github.com/cosmos/ibc-go/v8/modules/apps/29-fee/keeper"
	ibctransferkeeper "github.com/cosmos/ibc-go/v8/modules/apps/transfer/keeper"
	ibckeeper "github.com/cosmos/ibc-go/v8/modules/core/keeper"

	"github.com/evmos/evmos/v20/x/feemarket"
	feemarketkeeper "github.com/evmos/evmos/v20/x/feemarket/keeper"
	feemarkettypes "github.com/evmos/evmos/v20/x/feemarket/types"
	"github.com/evmos/evmos/v20/x/evm"
	evmkeeper "github.com/evmos/evmos/v20/x/evm/keeper"
	evmtypes "github.com/evmos/evmos/v20/x/evm/types"
	"github.com/evmos/evmos/v20/x/erc20"
	erc20keeper "github.com/evmos/evmos/v20/x/erc20/keeper"
	erc20types "github.com/evmos/evmos/v20/x/erc20/types"

	claimmodulekeeper "github.com/LumeraProtocol/lumera/x/claim/keeper"
	lumeraidmodulekeeper "github.com/LumeraProtocol/lumera/x/lumeraid/keeper"
	supernodemodulekeeper "github.com/LumeraProtocol/lumera/x/supernode/keeper"

	actionmodulekeeper "github.com/LumeraProtocol/lumera/x/action/keeper"
	// this line is used by starport scaffolding # stargate/app/moduleImport

	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	"github.com/LumeraProtocol/lumera/docs"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
)

const (
	AccountAddressPrefix = "lumera"
	Name                 = "lumera"
)

var (
	// DefaultNodeHome default home directories for the application daemon
	DefaultNodeHome string
)

var (
	_ runtime.AppI            = (*App)(nil)
	_ servertypes.Application = (*App)(nil)
)

// App extends an ABCI application, but with most of its parameters exported.
// They are exported for convenience in creating helper functions, as object
// capabilities aren't needed for testing.
type App struct {
	*runtime.App
	legacyAmino       *codec.LegacyAmino
	appCodec          codec.Codec
	txConfig          client.TxConfig
	interfaceRegistry codectypes.InterfaceRegistry

	// keepers
	AccountKeeper         authkeeper.AccountKeeper
	BankKeeper            bankkeeper.Keeper
	StakingKeeper         *stakingkeeper.Keeper
	DistrKeeper           distrkeeper.Keeper
	ConsensusParamsKeeper consensuskeeper.Keeper

	SlashingKeeper       slashingkeeper.Keeper
	MintKeeper           mintkeeper.Keeper
	GovKeeper            *govkeeper.Keeper
	CrisisKeeper         *crisiskeeper.Keeper
	UpgradeKeeper        *upgradekeeper.Keeper
	ParamsKeeper         paramskeeper.Keeper
	AuthzKeeper          authzkeeper.Keeper
	EvidenceKeeper       evidencekeeper.Keeper
	FeeGrantKeeper       feegrantkeeper.Keeper
	GroupKeeper          groupkeeper.Keeper
	NFTKeeper            nftkeeper.Keeper
	CircuitBreakerKeeper circuitkeeper.Keeper

	// EVM
	EVMKeeper 			*evmkeeper.Keeper
	FeeMarketKeeper		feemarketkeeper.Keeper
	Erc20Keeper			erc20keeper.Keeper

	// IBC
	IBCKeeper           *ibckeeper.Keeper // IBC Keeper must be a pointer in the app, so we can SetRouter on it correctly
	CapabilityKeeper    *capabilitykeeper.Keeper
	IBCFeeKeeper        ibcfeekeeper.Keeper
	ICAControllerKeeper icacontrollerkeeper.Keeper
	ICAHostKeeper       icahostkeeper.Keeper
	TransferKeeper      ibctransferkeeper.Keeper

	// Scoped IBC
	ScopedIBCKeeper           capabilitykeeper.ScopedKeeper
	ScopedIBCTransferKeeper   capabilitykeeper.ScopedKeeper
	ScopedICAControllerKeeper capabilitykeeper.ScopedKeeper
	ScopedICAHostKeeper       capabilitykeeper.ScopedKeeper
	ScopedKeepers             map[string]capabilitykeeper.ScopedKeeper

	LumeraidKeeper lumeraidmodulekeeper.Keeper

	// CosmWasm
	WasmKeeper       wasmkeeper.Keeper
	ScopedWasmKeeper capabilitykeeper.ScopedKeeper

	ClaimKeeper     claimmodulekeeper.Keeper
	SupernodeKeeper supernodemodulekeeper.Keeper
	ActionKeeper    actionmodulekeeper.Keeper
	// this line is used by starport scaffolding # stargate/app/keeperDeclaration

	// simulation manager
	sm *module.SimulationManager
}

func init() {
	var err error
	clienthelpers.EnvPrefix = Name
	DefaultNodeHome, err = clienthelpers.GetNodeHomeDirectory("." + Name)
	if err != nil {
		panic(err)
	}
}

// getGovProposalHandlers return the chain proposal handlers.
func getGovProposalHandlers() []govclient.ProposalHandler {
	var govProposalHandlers []govclient.ProposalHandler
	// this line is used by starport scaffolding # stargate/app/govProposalHandlers

	govProposalHandlers = append(govProposalHandlers,
		paramsclient.ProposalHandler,
		// this line is used by starport scaffolding # stargate/app/govProposalHandler
	)

	return govProposalHandlers
}

// AppConfig returns the default app config.
func AppConfig() depinject.Config {
	return depinject.Configs(
		appConfig,
		// Alternatively, load the app config from a YAML file.
		// appconfig.LoadYAML(AppConfigYAML),
		depinject.Supply(
			// supply custom module basics
			map[string]module.AppModuleBasic{
				genutiltypes.ModuleName: genutil.NewAppModuleBasic(genutiltypes.DefaultMessageValidator),
				govtypes.ModuleName:     gov.NewAppModuleBasic(getGovProposalHandlers()),
				evmtypes.ModuleName:     evm.AppModuleBasic{},
				// this line is used by starport scaffolding # stargate/appConfig/moduleBasic
			},
		),
	)
}

// New returns a reference to an initialized App.
func New(
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

		// merge the AppConfig and other configuration in one config
		appConfig = depinject.Configs(
			AppConfig(),
			depinject.Supply(
				appOpts, // supply app options
				logger,  // supply logger
				// Supply with IBC keeper getter for the IBC modules with App Wiring.
				// The IBC Keeper cannot be passed because it has not been initiated yet.
				// Passing the getter, the app IBC Keeper will always be accessible.
				// This needs to be removed after IBC supports App Wiring.
				app.GetIBCKeeper,
				app.GetCapabilityScopedKeeper,

				// here alternative options can be supplied to the DI container.
				// those options can be used f.e to override the default behavior of some modules.
				// for instance supplying a custom address codec for not using bech32 addresses.
				// read the depinject documentation and depinject module wiring for more information
				// on available options and how to use them.
			),
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
		&app.LumeraidKeeper,
		&app.ClaimKeeper,
		&app.SupernodeKeeper,
		&app.ActionKeeper,
		// this line is used by starport scaffolding # stargate/app/keeperDefinition
	); err != nil {
		panic(err)
	}

	// add to default baseapp options
	// enable optimistic execution
	baseAppOptions = append(baseAppOptions, baseapp.SetOptimisticExecution())

	// build app
	app.App = appBuilder.Build(db, traceStore, baseAppOptions...)

	// register legacy modules that don't use dependency injection
	if err := app.registerIBCModules(appOpts); err != nil {
		return nil, err
	}

    // register EVM-related modules
    app.registerEvmModules()

	// register streaming services
	if err := app.RegisterStreamingServices(appOpts, app.kvStoreKeys()); err != nil {
		return nil, err
	}

	/****  Module Options ****/

	app.ModuleManager.RegisterInvariants(app.CrisisKeeper)

	// create the simulation manager and define the order of the modules for deterministic simulations
	overrideModules := map[string]module.AppModuleSimulation{
		authtypes.ModuleName: auth.NewAppModule(app.appCodec, app.AccountKeeper, authsims.RandomGenesisAccounts, app.GetSubspace(authtypes.ModuleName)),
	}
	app.sm = module.NewSimulationManagerFromAppModules(app.ModuleManager.Modules, overrideModules)
	app.sm.RegisterStoreDecoders()

	// A custom InitChainer sets if extra pre-init-genesis logic is required.
	// This is necessary for manually registered modules that do not support app wiring.
	// Manually set the module version map as shown below.
	// The upgrade module will automatically handle de-duplication of the module version map.
	app.SetInitChainer(func(ctx sdk.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
		if err := app.UpgradeKeeper.SetModuleVersionMap(ctx, app.ModuleManager.GetVersionMap()); err != nil {
			return nil, err
		}
		return app.App.InitChainer(ctx, req)
	})

	if err := app.Load(loadLatest); err != nil {
		return nil, err
	}

	return app, app.WasmKeeper.InitializePinnedCodes(app.NewUncachedContext(true, tmproto.Header{}))

}

// registerEvmModules registers and initializes all EVM-related modules
func (app *App) registerEvmModules() {
    // 1. Initialize FeeMarket module first
    app.initFeeMarketKeeper()
    
    // 2. Initialize EVM module (depends on FeeMarket)
    app.initEvmKeeper()
    
    // 3. Initialize ERC20 module (depends on EVM)
    app.initErc20Keeper()
    
    // Register the modules with the module manager
    feeMarketModule := feemarket.NewAppModule(
		app.FeeMarketKeeper,
		app.GetSubspace(feemarkettypes.ModuleName),
	)
    evmModule := evm.NewAppModule(
		app.EVMKeeper,
		app.AccountKeeper,
		app.GetSubspace(evmtypes.ModuleName),
	)
    erc20Module := erc20.NewAppModule(
		app.Erc20Keeper,
		app.AccountKeeper,
		app.GetSubspace(erc20types.ModuleName),
	)
    
    // Add modules to the module manager
    app.ModuleManager.Modules[feemarkettypes.ModuleName] = feeMarketModule
    app.ModuleManager.Modules[evmtypes.ModuleName] = evmModule
    app.ModuleManager.Modules[erc20types.ModuleName] = erc20Module
}

// initFeeMarketKeeper initializes the FeeMarket keeper
func (app *App) initFeeMarketKeeper() {
    // Create and mount store keys
    feeMarketKey := storetypes.NewKVStoreKey(feemarkettypes.StoreKey)
    feeMarketTransientKey := storetypes.NewTransientStoreKey(feemarkettypes.TransientKey)
    
    app.MountStore(feeMarketKey, storetypes.StoreTypeIAVL)
    app.MountStore(feeMarketTransientKey, storetypes.StoreTypeTransient)

	feeMarketSubspace := app.ParamsKeeper.Subspace(feemarkettypes.ModuleName)
	if !feeMarketSubspace.HasKeyTable() {
		feeMarketSubspace = feeMarketSubspace.WithKeyTable(feemarkettypes.ParamKeyTable())
	}	
    // Initialize the FeeMarket keeper
    app.FeeMarketKeeper = feemarketkeeper.NewKeeper(
        app.appCodec,
        authtypes.NewModuleAddress(govtypes.ModuleName), // authority
        feeMarketKey,
        feeMarketTransientKey,
        feeMarketSubspace,
    )
}

// initEvmKeeper initializes the EVM keeper
func (app *App) initEvmKeeper() {
	// Create and mount store keys
	evmKey := storetypes.NewKVStoreKey(evmtypes.StoreKey)
	evmTransientKey := storetypes.NewTransientStoreKey(evmtypes.TransientKey)

	 // Mount both stores
	app.MountStore(evmKey, storetypes.StoreTypeIAVL)
	app.MountStore(evmTransientKey, storetypes.StoreTypeTransient)

	// Create parameter subspace for EVM module if it doesn't exist
	evmSubspace := app.ParamsKeeper.Subspace(evmtypes.ModuleName)
	if !evmSubspace.HasKeyTable() {
		evmSubspace = evmSubspace.WithKeyTable(evmtypes.ParamKeyTable())
	}

    // Define the authority
    authority := authtypes.NewModuleAddress(govtypes.ModuleName)

    // Initialize the EVM Keeper
    app.EVMKeeper = evmkeeper.NewKeeper(
        app.appCodec,                // codec
        evmKey,                      // store key
        evmTransientKey,                // transient store key
        authority,                   // authority (typically governance module)
        app.AccountKeeper,           // account keeper
        app.BankKeeper,              // bank keeper
        app.StakingKeeper,           // staking keeper
        app.FeeMarketKeeper,         // fee market keeper
        &app.Erc20Keeper,   		 // erc20 keeper
        "",                          // tracer (empty string if not using)
        evmSubspace,                 // params subspace
    )
}

// initErc20Keeper initializes the ERC20 keeper
func (app *App) initErc20Keeper() {
    // Create and mount store key
    erc20Key := storetypes.NewKVStoreKey(erc20types.StoreKey)
    app.MountStore(erc20Key, storetypes.StoreTypeIAVL)

	erc20Subspace := app.ParamsKeeper.Subspace(erc20types.ModuleName)
	if !erc20Subspace.HasKeyTable() {
    	erc20Subspace = erc20Subspace.WithKeyTable(erc20types.ParamKeyTable())
	}
	
	// Define the authority
	authority := authtypes.NewModuleAddress(govtypes.ModuleName)

    // Initialize the ERC20 keeper
    app.Erc20Keeper = erc20keeper.NewKeeper(
        erc20Key,
        app.appCodec,
        authority, // authority
        app.AccountKeeper,
        app.BankKeeper,
        app.EVMKeeper,
        app.StakingKeeper,
        app.AuthzKeeper,
        nil, // IBC Transfer keeper if needed
    )
}

// LegacyAmino returns App's amino codec.
//
// NOTE: This is solely to be used for testing purposes as it may be desirable
// for modules to register their own custom testing types.
func (app *App) LegacyAmino() *codec.LegacyAmino {
	return app.legacyAmino
}

// AppCodec returns App's app codec.
//
// NOTE: This is solely to be used for testing purposes as it may be desirable
// for modules to register their own custom testing types.
func (app *App) AppCodec() codec.Codec {
	return app.appCodec
}

// InterfaceRegistry returns App's interfaceRegistry.
func (app *App) InterfaceRegistry() codectypes.InterfaceRegistry {
	return app.interfaceRegistry
}

// TxConfig returns App's tx config.
func (app *App) TxConfig() client.TxConfig {
	return app.txConfig
}

// GetKey returns the KVStoreKey for the provided store key.
func (app *App) GetKey(storeKey string) *storetypes.KVStoreKey {
	kvStoreKey, ok := app.UnsafeFindStoreKey(storeKey).(*storetypes.KVStoreKey)
	if !ok {
		return nil
	}
	return kvStoreKey
}

// GetMemKey returns the MemoryStoreKey for the provided store key.
func (app *App) GetMemKey(storeKey string) *storetypes.MemoryStoreKey {
	key, ok := app.UnsafeFindStoreKey(storeKey).(*storetypes.MemoryStoreKey)
	if !ok {
		return nil
	}

	return key
}

// kvStoreKeys returns all the kv store keys registered inside App.
func (app *App) kvStoreKeys() map[string]*storetypes.KVStoreKey {
	keys := make(map[string]*storetypes.KVStoreKey)
	for _, k := range app.GetStoreKeys() {
		if kv, ok := k.(*storetypes.KVStoreKey); ok {
			keys[kv.Name()] = kv
		}
	}

	return keys
}

// GetSubspace returns a param subspace for a given module name.
func (app *App) GetSubspace(moduleName string) paramstypes.Subspace {
	subspace, _ := app.ParamsKeeper.GetSubspace(moduleName)
	return subspace
}

// GetIBCKeeper returns the IBC keeper.
func (app *App) GetIBCKeeper() *ibckeeper.Keeper {
	return app.IBCKeeper
}

// GetCapabilityScopedKeeper returns the capability scoped keeper.
func (app *App) GetCapabilityScopedKeeper(moduleName string) capabilitykeeper.ScopedKeeper {
	sk, ok := app.ScopedKeepers[moduleName]
	if !ok {
		sk = app.CapabilityKeeper.ScopeToModule(moduleName)
		app.ScopedKeepers[moduleName] = sk
	}
	return sk
}

// SimulationManager implements the SimulationApp interface.
func (app *App) SimulationManager() *module.SimulationManager {
	return app.sm
}

// RegisterAPIRoutes registers all application module routes with the provided
// API server.
func (app *App) RegisterAPIRoutes(apiSvr *api.Server, apiConfig config.APIConfig) {
	app.App.RegisterAPIRoutes(apiSvr, apiConfig)
	// register swagger API in app.go so that other applications can override easily
	if err := server.RegisterSwaggerAPI(apiSvr.ClientCtx, apiSvr.Router, apiConfig.Swagger); err != nil {
		panic(err)
	}

	// register app's OpenAPI routes.
	docs.RegisterOpenAPIService(Name, apiSvr.Router)
}

// GetMaccPerms returns a copy of the module account permissions
//
// NOTE: This is solely to be used for testing purposes.
func GetMaccPerms() map[string][]string {
	dup := make(map[string][]string)
	for _, perms := range moduleAccPerms {
		dup[perms.Account] = perms.Permissions
	}
	return dup
}

// BlockedAddresses returns all the app's blocked account addresses.
func BlockedAddresses() map[string]bool {
	result := make(map[string]bool)
	if len(blockAccAddrs) > 0 {
		for _, addr := range blockAccAddrs {
			result[addr] = true
		}
	} else {
		for addr := range GetMaccPerms() {
			result[addr] = true
		}
	}
	return result
}
