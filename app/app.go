package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	_ "cosmossdk.io/api/cosmos/tx/config/v1" // import for side-effects
	clienthelpers "cosmossdk.io/client/v2/helpers"
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/depinject"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	_ "cosmossdk.io/x/circuit" // import for side-effects
	circuitkeeper "cosmossdk.io/x/circuit/keeper"
	_ "cosmossdk.io/x/evidence" // import for side-effects
	evidencekeeper "cosmossdk.io/x/evidence/keeper"
	feegrantkeeper "cosmossdk.io/x/feegrant/keeper"
	_ "cosmossdk.io/x/feegrant/module" // import for side-effects
	_ "cosmossdk.io/x/upgrade"         // import for side-effects
	upgradekeeper "cosmossdk.io/x/upgrade/keeper"
	abci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
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
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/version"
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

	"github.com/CosmWasm/wasmd/x/wasm"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	ibcpacketforwardkeeper "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v10/packetforward/keeper"
	icacontrollerkeeper "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/controller/keeper"
	icahostkeeper "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/host/keeper"
	ibctransferkeeper "github.com/cosmos/ibc-go/v10/modules/apps/transfer/keeper"
	ibcporttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"

	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	"github.com/cosmos/cosmos-sdk/x/auth/posthandler"
	evmante "github.com/cosmos/evm/ante"
	evmantetypes "github.com/cosmos/evm/ante/types"
	evmmempool "github.com/cosmos/evm/mempool"
	evmserver "github.com/cosmos/evm/server"
	cosmosevmutils "github.com/cosmos/evm/utils"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
	corevm "github.com/ethereum/go-ethereum/core/vm"

	appevm "github.com/LumeraProtocol/lumera/app/evm"
	appopenrpc "github.com/LumeraProtocol/lumera/app/openrpc"
	upgrades "github.com/LumeraProtocol/lumera/app/upgrades"
	appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
	lcfg "github.com/LumeraProtocol/lumera/config"
	actionmodulekeeper "github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	auditmodulekeeper "github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	claimmodulekeeper "github.com/LumeraProtocol/lumera/x/claim/keeper"
	evmigrationmodulekeeper "github.com/LumeraProtocol/lumera/x/evmigration/keeper"
	lumeraidmodulekeeper "github.com/LumeraProtocol/lumera/x/lumeraid/keeper"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	erc20keeper "github.com/cosmos/evm/x/erc20/keeper"
	feemarketkeeper "github.com/cosmos/evm/x/feemarket/keeper"
	precisebankkeeper "github.com/cosmos/evm/x/precisebank/keeper"
	evmkeeper "github.com/cosmos/evm/x/vm/keeper"

	// this line is used by starport scaffolding # stargate/app/moduleImport

	"github.com/LumeraProtocol/lumera/docs"
)

const (
	Name = "lumera"
)

var (
	// DefaultNodeHome default home directories for the application daemon
	DefaultNodeHome string
)

var (
	_ runtime.AppI            = (*App)(nil)
	_ servertypes.Application = (*App)(nil)
	_ evmserver.Application   = (*App)(nil)
)

// App extends an ABCI application, but with most of its parameters exported.
// They are exported for convenience in creating helper functions, as object
// capabilities aren't needed for testing.
type App struct {
	*runtime.App
	legacyAmino        *codec.LegacyAmino
	appCodec           codec.Codec
	txConfig           client.TxConfig
	clientCtx          client.Context
	interfaceRegistry  codectypes.InterfaceRegistry
	ibcRouter          *ibcporttypes.Router
	pendingTxListeners []evmante.PendingTxListener
	evmMempool         *evmmempool.ExperimentalEVMMempool
	// evmTxBroadcaster is used to asynchronously broadcast promoted EVM transactions from the mempool to the network without blocking CheckTx execution.
	evmTxBroadcaster *evmTxBroadcastDispatcher
	// if true, the app will log additional information about mempool transaction broadcasts, which can be noisy but is useful for debugging mempool behavior.
	evmBroadcastDebug  bool
	evmBroadcastLogger log.Logger

	// jsonrpcRateLimitProxy is the optional rate-limiting reverse proxy for JSON-RPC.
	jsonrpcRateLimitProxy       *http.Server
	jsonrpcRateLimitCleanupStop chan struct{}

	// keepers
	// only keepers required by the app are exposed
	// the list of all modules is available in the app_config
	AuthKeeper            authkeeper.AccountKeeper
	BankKeeper            bankkeeper.Keeper
	StakingKeeper         *stakingkeeper.Keeper
	SlashingKeeper        slashingkeeper.Keeper
	MintKeeper            mintkeeper.Keeper
	DistrKeeper           distrkeeper.Keeper
	GovKeeper             *govkeeper.Keeper
	UpgradeKeeper         *upgradekeeper.Keeper
	AuthzKeeper           authzkeeper.Keeper
	ConsensusParamsKeeper consensuskeeper.Keeper
	CircuitBreakerKeeper  circuitkeeper.Keeper
	ParamsKeeper          paramskeeper.Keeper
	EvidenceKeeper        evidencekeeper.Keeper
	FeeGrantKeeper        feegrantkeeper.Keeper
	GroupKeeper           groupkeeper.Keeper

	// ibc keepers
	IBCKeeper           *ibckeeper.Keeper
	ICAControllerKeeper icacontrollerkeeper.Keeper
	ICAHostKeeper       icahostkeeper.Keeper
	TransferKeeper      ibctransferkeeper.Keeper

	// IBC middleware keepers
	PacketForwardKeeper *ibcpacketforwardkeeper.Keeper

	// CosmWasm
	WasmKeeper *wasmkeeper.Keeper

	LumeraidKeeper  lumeraidmodulekeeper.Keeper
	ClaimKeeper     claimmodulekeeper.Keeper
	SupernodeKeeper sntypes.SupernodeKeeper
	AuditKeeper     auditmodulekeeper.Keeper
	ActionKeeper    actionmodulekeeper.Keeper

	// EVM keepers
	FeeMarketKeeper   feemarketkeeper.Keeper
	PreciseBankKeeper precisebankkeeper.Keeper
	EVMKeeper         *evmkeeper.Keeper
	Erc20Keeper       erc20keeper.Keeper
	EvmigrationKeeper    evmigrationmodulekeeper.Keeper
	erc20PolicyWrapper   *erc20PolicyKeeperWrapper
	// this line is used by starport scaffolding # stargate/app/keeperDeclaration

	// simulation manager
	sm *module.SimulationManager
}

func init() {
	var err error
	clienthelpers.EnvPrefix = strings.ToUpper(Name)
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
func AppConfig(appOpts servertypes.AppOptions) depinject.Config {
	return depinject.Configs(
		appConfig,
		// Alternatively, load the app config from a YAML file.
		// appconfig.LoadYAML(AppConfigYAML),
		depinject.Supply(
			appOpts,
			// supply custom module basics
			map[string]module.AppModuleBasic{
				genutiltypes.ModuleName: genutil.NewAppModuleBasic(genutiltypes.DefaultMessageValidator),
				govtypes.ModuleName:     gov.NewAppModuleBasic(getGovProposalHandlers()),
				// this line is used by starport scaffolding # stargate/appConfig/moduleBasic
			},
		),
		// EVM custom signers: MsgEthereumTx uses a non-standard signer derivation
		// that must be registered with the interface registry via depinject.
		depinject.Provide(appevm.ProvideCustomGetSigners),
		depinject.Invoke(lcfg.RegisterExtraInterfaces),
	)
}

// New returns a reference to an initialized App.
func New(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	loadLatest bool,
	appOpts servertypes.AppOptions,
	wasmOpts []wasmkeeper.Option,
	baseAppOptions ...func(*baseapp.BaseApp),
) *App {
	var (
		app        = &App{}
		appBuilder *runtime.AppBuilder

		// merge the AppConfig and other configuration in one config
		appConfig = depinject.Configs(
			AppConfig(appOpts),
			depinject.Supply(
				logger, // supply logger
				// here alternative options can be supplied to the DI container.
				// those options can be used f.e to override the default behavior of some modules.
				// for instance supplying a custom address codec for not using bech32 addresses.
				// read the depinject documentation and depinject module wiring for more information
				// on available options and how to use them.
			),
		)
	)

	var appModules map[string]appmodule.AppModule
	if err := depinject.Inject(appConfig,
		&appBuilder,
		&appModules,
		&app.appCodec,
		&app.legacyAmino,
		&app.txConfig,
		&app.interfaceRegistry,
		&app.AuthKeeper,
		&app.BankKeeper,
		&app.StakingKeeper,
		&app.SlashingKeeper,
		&app.MintKeeper,
		&app.DistrKeeper,
		&app.GovKeeper,
		&app.UpgradeKeeper,
		&app.ParamsKeeper,
		&app.AuthzKeeper,
		&app.ConsensusParamsKeeper,
		&app.EvidenceKeeper,
		&app.FeeGrantKeeper,
		&app.GroupKeeper,
		&app.CircuitBreakerKeeper,
		&app.LumeraidKeeper,
		&app.ClaimKeeper,
		&app.SupernodeKeeper,
		&app.AuditKeeper,
		&app.ActionKeeper,
		&app.EvmigrationKeeper,

		// this line is used by starport scaffolding # stargate/app/keeperDefinition
	); err != nil {
		panic(err)
	}
	// Keep LegacyAmino aligned with Cosmos EVM so SDK ante code paths that still
	// marshal StdSignature via legacy.Cdc support eth_secp256k1 pubkeys.
	registerLumeraLegacyAminoCodec(app.legacyAmino)

	// add to default baseapp options, enable optimistic execution
	baseAppOptions = append(baseAppOptions, baseapp.SetOptimisticExecution())

	// build app
	app.App = appBuilder.Build(db, traceStore, baseAppOptions...)
	app.SetVersion(version.Version)
	app.appendEVMPrecompileSendRestriction()

	// configure EVM coin info (must happen before EVM module keepers are created)
	if err := appevm.Configure(); err != nil {
		panic(err)
	}

	// register EVM modules first — the ante handler (set during IBC/wasm registration)
	// depends on EVM keepers (FeeMarketKeeper, EVMKeeper).
	if err := app.registerEVMModules(appOpts); err != nil {
		panic(err)
	}

	// Create the ERC20 registration policy wrapper (governance-controlled IBC voucher
	// ERC20 auto-registration). Must be created before registerIBCModules, which wires
	// the wrapper into the IBC transfer middleware stacks.
	app.registerERC20Policy()

	// register legacy modules (IBC, wasm)
	if err := app.registerIBCModules(appOpts, wasmOpts...); err != nil {
		panic(err)
	}
	// Inject IBC store keys into the EVM keeper's KV store map so the snapshot
	// multi-store used by StateDB includes "ibc" and "transfer" stores.
	// registerEVMModules captured kvStoreKeys() before IBC stores were registered;
	// adding them here fixes Bug #6 (ICS20 precompile panic).
	app.syncEVMStoreKeys()

	// Enable Cosmos EVM static precompiles once IBC keepers are available.
	app.configureEVMStaticPrecompiles()

	// set ante and post handlers — must happen after all modules are registered
	// since the ante handler depends on EVM, Wasm, and IBC keepers.
	if err := app.setAnteHandler(appOpts); err != nil {
		panic(err)
	}
	// wire the Cosmos EVM mempool into BaseApp after ante is set
	if err := app.configureEVMMempool(appOpts, logger); err != nil {
		panic(fmt.Errorf("failed to configure EVM mempool: %w", err))
	}
	if err := app.setPostHandler(); err != nil {
		panic(err)
	}

	// register streaming services
	if err := app.RegisterStreamingServices(appOpts, app.kvStoreKeys()); err != nil {
		panic(err)
	}

	// Start optional JSON-RPC rate-limiting reverse proxy.
	app.startJSONRPCRateLimitProxy(appOpts, logger)

	// **** SETUP UPGRADES (upgrade handlers and store loaders) ****
	// This needs to be done after keepers are initialized but before loading state.
	app.setupUpgrades()

	/****  Module Options ****/

	// create the simulation manager and define the order of the modules for deterministic simulations
	overrideModules := map[string]module.AppModuleSimulation{
		authtypes.ModuleName: auth.NewAppModule(app.appCodec, app.AuthKeeper, authsims.RandomGenesisAccounts, app.GetSubspace(authtypes.ModuleName)),
	}
	app.sm = module.NewSimulationManagerFromAppModules(app.ModuleManager.Modules, overrideModules)

	app.sm.RegisterStoreDecoders()

	// A custom InitChainer sets if extra pre-init-genesis logic is required.
	// This is necessary for manually registered modules that do not support app wiring.
	// Manually set the module version map as shown below.
	// The upgrade module will automatically handle de-duplication of the module version map.
	app.SetInitChainer(func(ctx sdk.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
		if err := app.UpgradeKeeper.SetModuleVersionMap(ctx, app.ModuleManager.GetVersionMap()); err != nil {
			panic(err)
		}

		// Pre-populate the ERC20 registration policy with default allowlist
		// base denoms (uatom, uosmo, uusdc) on first genesis.
		app.initERC20PolicyDefaults(ctx)

		return app.App.InitChainer(ctx, req)
	})

	if err := app.Load(loadLatest); err != nil {
		panic(err)
	}

	ctx := app.NewUncachedContext(true, tmproto.Header{})
	if err := app.WasmKeeper.InitializePinnedCodes(ctx); err != nil {
		panic(fmt.Errorf("failed to initialize pinned wasm codes: %w", err))
	}

	return app
}

// GetSubspace returns a param subspace for a given module name.
func (app *App) GetSubspace(moduleName string) paramstypes.Subspace {
	subspace, _ := app.ParamsKeeper.GetSubspace(moduleName)
	return subspace
}

// setupUpgrades configures the store loader for upcoming upgrades and registers upgrade handlers.
// This needs to be called BEFORE app.Load()
func (app *App) setupUpgrades() {
	params := appParams.AppUpgradeParams{
		ChainID:               app.ChainID(),
		Logger:                app.Logger(),
		ModuleManager:         app.ModuleManager,
		Configurator:          app.Configurator(),
		ActionKeeper:          &app.ActionKeeper,
		SupernodeKeeper:       app.SupernodeKeeper,
		ParamsKeeper:          &app.ParamsKeeper,
		ConsensusParamsKeeper: &app.ConsensusParamsKeeper,
		AuditKeeper:           &app.AuditKeeper,
	}

	allUpgrades := upgrades.AllUpgrades(params)
	for upgradeName, upgradeConfig := range allUpgrades {
		if upgradeConfig.Handler == nil {
			continue
		}
		app.UpgradeKeeper.SetUpgradeHandler(upgradeName, upgradeConfig.Handler)
		app.Logger().Info("Registered upgrade handler", "name", upgradeName)
	}

	upgradeInfo, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	// No upgrade scheduled, skip
	if err != nil {
		// Only panic if the error is unexpected, not if the file simply doesn't exist
		if !os.IsNotExist(err) {
			panic(fmt.Sprintf("failed to read upgrade info from disk: %v", err))
		}
		return // No upgrade info file, normal startup
	}

	if upgradeInfo.Name == "" {
		app.Logger().Info("No pending upgrade plan found on disk")
		return
	}

	upgradeConfig, found := allUpgrades[upgradeInfo.Name]
	if !found {
		panic(fmt.Sprintf("upgrade plan %q is scheduled at height %d but not registered in this binary", upgradeInfo.Name, upgradeInfo.Height))
	}

	useAdaptiveStoreUpgrades := upgrades.ShouldEnableStoreUpgradeManager(params.ChainID)
	if upgradeConfig.StoreUpgrade == nil && !useAdaptiveStoreUpgrades {
		app.Logger().Info("No store upgrades registered for pending plan", "name", upgradeInfo.Name)
		return
	}

	if app.UpgradeKeeper.IsSkipHeight(upgradeInfo.Height) {
		app.Logger().Info("Skipping store loader because height is flagged to skip", "name", upgradeInfo.Name, "height", upgradeInfo.Height)
		return
	}

	if useAdaptiveStoreUpgrades {
		expectedStoreNames := upgrades.KVStoreNames(app.GetStoreKeys())
		selection := upgrades.StoreLoaderForUpgrade(
			upgradeInfo.Name,
			upgradeInfo.Height,
			upgradeConfig.StoreUpgrade,
			expectedStoreNames,
			app.Logger(),
			true,
		)
		app.SetStoreLoader(selection.Loader)
		app.Logger().Info(selection.LogMessage(), "name", upgradeInfo.Name, "height", upgradeInfo.Height)
		return
	}

	selection := upgrades.StoreLoaderForUpgrade(
		upgradeInfo.Name,
		upgradeInfo.Height,
		upgradeConfig.StoreUpgrade,
		nil,
		app.Logger(),
		false,
	)
	app.SetStoreLoader(selection.Loader)
	app.Logger().Info(selection.LogMessage(), "name", upgradeInfo.Name, "height", upgradeInfo.Height)
}

// LegacyAmino returns App's amino codec.
func (app *App) LegacyAmino() *codec.LegacyAmino {
	return app.legacyAmino
}

// AppCodec returns App's app codec.
func (app *App) AppCodec() codec.Codec {
	return app.appCodec
}

// InterfaceRegistry returns App's InterfaceRegistry.
func (app *App) InterfaceRegistry() codectypes.InterfaceRegistry {
	return app.interfaceRegistry
}

// TxConfig returns App's TxConfig
func (app *App) TxConfig() client.TxConfig {
	return app.txConfig
}

// RegisterPendingTxListener registers a callback consumed by JSON-RPC pending
// transaction streaming.
func (app *App) RegisterPendingTxListener(listener func(common.Hash)) {
	app.pendingTxListeners = append(app.pendingTxListeners, listener)
}

func (app *App) onPendingTx(hash common.Hash) {
	for _, listener := range app.pendingTxListeners {
		listener(hash)
	}
}

// GetMempool returns the app-side EVM mempool when configured.
func (app *App) GetMempool() sdkmempool.ExtMempool {
	return app.evmMempool
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

// GetTransientKey returns the TransientStoreKey for the provided store key.
func (app *App) GetTransientKey(storeKey string) *storetypes.TransientStoreKey {
	key, ok := app.UnsafeFindStoreKey(storeKey).(*storetypes.TransientStoreKey)
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

// GetIBCKeeper returns the IBC keeper.
func (app *App) GetIBCKeeper() *ibckeeper.Keeper {
	return app.IBCKeeper
}

// SimulationManager implements the SimulationApp interface
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
	apiSvr.Router.HandleFunc(appopenrpc.HTTPPath, appopenrpc.ServeHTTP).Methods(http.MethodGet, http.MethodHead, http.MethodOptions)

	// register app's OpenAPI routes.
	docs.RegisterOpenAPIService(Name, apiSvr.Router)
}

// GetMaccPerms returns a copy of the module account permissions
//
// NOTE: This is solely to be used for testing purposes.
func GetMaccPerms() map[string][]string {
	dup := make(map[string][]string)
	for _, perms := range moduleAccPerms {
		dup[perms.GetAccount()] = perms.GetPermissions()
	}

	return dup
}

// setPostHandler sets the app's post handler, which is responsible for post-processing transactions after they are executed.
func (app *App) setPostHandler() error {
	postHandler, err := posthandler.NewPostHandler(
		posthandler.HandlerOptions{},
	)
	if err != nil {
		return err
	}
	app.SetPostHandler(postHandler)
	return nil
}

// setAnteHandler sets the app's ante handler, which is responsible for pre-processing transactions before they are executed.
func (app *App) setAnteHandler(appOpts servertypes.AppOptions) error {
	wasmConfig, err := wasm.ReadNodeConfig(appOpts)
	if err != nil {
		return fmt.Errorf("error while reading wasm config: %s", err)
	}

	anteHandler, err := appevm.NewAnteHandler(
		appevm.HandlerOptions{
			HandlerOptions: ante.HandlerOptions{
				AccountKeeper:          app.AuthKeeper,
				BankKeeper:             app.BankKeeper,
				SignModeHandler:        app.txConfig.SignModeHandler(),
				FeegrantKeeper:         app.FeeGrantKeeper,
				SigGasConsumer:         evmante.SigVerificationGasConsumer,
				ExtensionOptionChecker: evmantetypes.HasDynamicFeeExtensionOption,
			},
			IBCKeeper:             app.IBCKeeper,
			WasmConfig:            &wasmConfig,
			WasmKeeper:            app.WasmKeeper,
			TXCounterStoreService: runtime.NewKVStoreService(app.GetKey(wasmtypes.StoreKey)),
			CircuitKeeper:         &app.CircuitBreakerKeeper,
			// EVM keepers for dual-routing ante handler
			EVMAccountKeeper:  app.AuthKeeper,
			FeeMarketKeeper:   app.FeeMarketKeeper,
			EvmKeeper:         app.EVMKeeper,
			PendingTxListener: app.onPendingTx,
			// no max gas limit in the ante handler, as the EVM mempool will enforce its own max gas limit for transactions entering the mempool
			MaxTxGasWanted: 0,
			// enable dynamic fee checking by default, with the option to disable via app config
			DynamicFeeChecker: true,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create AnteHandler: %s", err)
	}

	app.SetAnteHandler(anteHandler)
	return nil
}

// BlockedAddresses returns all the app's blocked account addresses.
func BlockedAddresses() map[string]bool {
	result := make(map[string]bool)

	if len(blockAccAddrs) > 0 {
		for _, moduleName := range blockAccAddrs {
			result[authtypes.NewModuleAddress(moduleName).String()] = true
		}
	} else {
		for moduleName := range GetMaccPerms() {
			result[authtypes.NewModuleAddress(moduleName).String()] = true
		}
	}

	for addr := range blockedPrecompileAddresses() {
		result[addr] = true
	}

	return result
}

func blockedPrecompileAddresses() map[string]bool {
	blocked := make(map[string]bool)

	blockedPrecompilesHex := append([]string{}, evmtypes.AvailableStaticPrecompiles...)
	for _, addr := range corevm.PrecompiledAddressesPrague {
		blockedPrecompilesHex = append(blockedPrecompilesHex, addr.Hex())
	}

	for _, precompile := range blockedPrecompilesHex {
		blocked[cosmosevmutils.Bech32StringFromHexAddress(precompile)] = true
	}

	return blocked
}

func (app *App) appendEVMPrecompileSendRestriction() {
	blocked := blockedPrecompileAddresses()
	app.BankKeeper.AppendSendRestriction(func(_ context.Context, _, toAddr sdk.AccAddress, _ sdk.Coins) (sdk.AccAddress, error) {
		if blocked[toAddr.String()] {
			return toAddr, sdkerrors.ErrUnauthorized.Wrapf("sending coins to EVM precompile address %s is not allowed", toAddr.String())
		}
		return toAddr, nil
	})
}
