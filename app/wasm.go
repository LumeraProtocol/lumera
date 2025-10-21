package app

import (
	"fmt"

	storetypes "cosmossdk.io/store/types"
	"github.com/CosmWasm/wasmd/x/wasm"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	wasmvmtypes "github.com/CosmWasm/wasmvm/v3/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/runtime"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	"github.com/cosmos/cosmos-sdk/x/auth/posthandler"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/gogoproto/proto"
	ibcapi "github.com/cosmos/ibc-go/v10/modules/core/api"
)

func uint32Ptr(v uint32) *uint32 {
	return &v
}

// registerWasmModules register CosmWasm keepers and non dependency inject modules.
func (app *App) registerWasmModules(
	appOpts servertypes.AppOptions,
	ibcRouterV2 *ibcapi.Router,
	wasmOpts ...wasmkeeper.Option,
) (*wasm.IBCHandler, error) {
	// set up non depinject support modules store keys
	if err := app.RegisterStores(
		storetypes.NewKVStoreKey(wasmtypes.StoreKey),
	); err != nil {
		panic(err)
	}

	wasmNodeConfig, err := wasm.ReadNodeConfig(appOpts)
	if err != nil {
		return nil, fmt.Errorf("error while reading wasm config: %s", err)
	}

	vmConfig := wasmtypes.VMConfig{
		WasmLimits: wasmvmtypes.WasmLimits{
			InitialMemoryLimitPages: uint32Ptr(64), // 64 * 64KiB = 4MiB
			TableSizeLimitElements:  uint32Ptr(1024),
			MaxImports:              uint32Ptr(256),
			MaxFunctions:            uint32Ptr(1024),
			MaxFunctionParams:       uint32Ptr(16),
			MaxTotalFunctionParams:  uint32Ptr(2048),
			MaxFunctionResults:      uint32Ptr(8),
		},
	}

	homeDir, ok := appOpts.Get(FlagWasmHomeDir).(string)
	if !ok || homeDir == "" {
		homeDir = DefaultNodeHome
	}

	capabilities := append(wasmkeeper.BuiltInCapabilities(), app.Name())
	authority := authtypes.NewModuleAddress(govtypes.ModuleName).String()

	// The last arguments can contain custom message handlers, and custom query handlers,
	// if we want to allow any custom callbacks
	wasmKeeper := wasmkeeper.NewKeeper(
		app.AppCodec(),
		runtime.NewKVStoreService(app.GetKey(wasmtypes.StoreKey)),
		app.AuthKeeper,
		app.BankKeeper,
		app.StakingKeeper,
		distrkeeper.NewQuerier(app.DistrKeeper),
		app.IBCKeeper.ChannelKeeper,
		app.IBCKeeper.ChannelKeeper,
		app.TransferKeeper,
		app.MsgServiceRouter(),
		app.GRPCQueryRouter(),
		homeDir,
		wasmNodeConfig,
		vmConfig,
		capabilities,
		authority,
		ibcRouterV2,
		wasmOpts...,
	)
	app.WasmKeeper = &wasmKeeper

	// register IBC modules
	if err := app.RegisterModules(
		wasm.NewAppModule(
			app.AppCodec(),
			app.WasmKeeper,
			app.StakingKeeper,
			app.AuthKeeper,
			app.BankKeeper,
			app.MsgServiceRouter(),
			app.GetSubspace(wasmtypes.ModuleName),
		)); err != nil {
		return nil, err
	}

	if err := app.setAnteHandler(app.txConfig, wasmNodeConfig, app.GetKey(wasmtypes.StoreKey)); err != nil {
		return nil, err
	}

	if manager := app.SnapshotManager(); manager != nil {
		err := manager.RegisterExtensions(
			wasmkeeper.NewWasmSnapshotter(app.CommitMultiStore(), app.WasmKeeper),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to register snapshot extension: %s", err)
		}
	}

	if err := app.setPostHandler(); err != nil {
		return nil, err
	}

	// At startup, after all modules have been registered, check that all proto
	// annotations are correct.
	protoFiles, err := proto.MergedRegistry()
	if err != nil {
		return nil, err
	}
	err = msgservice.ValidateProtoAnnotations(protoFiles)
	if err != nil {
		return nil, err
	}

	// Create wasm ibc Stack
	wasmStackIBCHandler := wasm.NewIBCHandler(
		app.WasmKeeper,
		app.IBCKeeper.ChannelKeeper,
		app.IBCKeeper.ChannelKeeper)

	return &wasmStackIBCHandler, nil
}

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

func (app *App) setAnteHandler(txConfig client.TxConfig, wasmConfig wasmtypes.NodeConfig, txCounterStoreKey *storetypes.KVStoreKey) error {
	anteHandler, err := NewAnteHandler(
		HandlerOptions{
			HandlerOptions: ante.HandlerOptions{
				AccountKeeper:   app.AuthKeeper,
				BankKeeper:      app.BankKeeper,
				SignModeHandler: txConfig.SignModeHandler(),
				FeegrantKeeper:  app.FeeGrantKeeper,
				SigGasConsumer:  ante.DefaultSigVerificationGasConsumer,
			},
			IBCKeeper:             app.IBCKeeper,
			WasmConfig:            &wasmConfig,
			WasmKeeper:            app.WasmKeeper,
			TXCounterStoreService: runtime.NewKVStoreService(txCounterStoreKey),
			CircuitKeeper:         &app.CircuitBreakerKeeper,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create AnteHandler: %s", err)
	}

	// Set the AnteHandler for the app
	app.SetAnteHandler(anteHandler)
	return nil
}
