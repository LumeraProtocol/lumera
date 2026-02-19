package app

import (
	"fmt"

	storetypes "cosmossdk.io/store/types"
	"github.com/CosmWasm/wasmd/x/wasm"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	wasmvmtypes "github.com/CosmWasm/wasmvm/v3/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/gogoproto/proto"
)

func uint32Ptr(v uint32) *uint32 {
	return &v
}

// registerWasmModules register CosmWasm keepers and non dependency inject modules.
func (app *App) registerWasmModules(
	appOpts servertypes.AppOptions,
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
		app.IBCKeeper.ChannelKeeperV2,
		app.TransferKeeper,
		app.MsgServiceRouter(),
		app.GRPCQueryRouter(),
		homeDir,
		wasmNodeConfig,
		vmConfig,
		capabilities,
		authority,
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

	if manager := app.SnapshotManager(); manager != nil {
		err := manager.RegisterExtensions(
			wasmkeeper.NewWasmSnapshotter(app.CommitMultiStore(), app.WasmKeeper),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to register snapshot extension: %s", err)
		}
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
		app.TransferKeeper,
		app.IBCKeeper.ChannelKeeper,
	)

	return &wasmStackIBCHandler, nil
}
