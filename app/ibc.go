package app

import (
	"context"
	"errors"

	"cosmossdk.io/core/appmodule"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/runtime"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"

	lcfg "github.com/LumeraProtocol/lumera/config"

	pfm "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v10/packetforward"
	pfmkeeper "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v10/packetforward/keeper"
	pfmtypes "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v10/packetforward/types"
	icamodule "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts"
	icacontroller "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/controller"
	icacontrollerkeeper "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/controller/keeper"
	icacontrollertypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/controller/types"
	icahost "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/host"
	icahostkeeper "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/host/keeper"
	icahosttypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/types"
	ibccallbacks "github.com/cosmos/ibc-go/v10/modules/apps/callbacks"
	ibccallbacksv2 "github.com/cosmos/ibc-go/v10/modules/apps/callbacks/v2"
	ibctransfer "github.com/cosmos/ibc-go/v10/modules/apps/transfer"
	ibctransferkeeper "github.com/cosmos/ibc-go/v10/modules/apps/transfer/keeper"
	ibctransfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	ibctransferv2 "github.com/cosmos/ibc-go/v10/modules/apps/transfer/v2"
	ibc "github.com/cosmos/ibc-go/v10/modules/core"
	ibcclienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types" // nolint:staticcheck // Deprecated: params key table is needed for params migration
	ibcconnectiontypes "github.com/cosmos/ibc-go/v10/modules/core/03-connection/types"
	ibcporttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"
	ibcapi "github.com/cosmos/ibc-go/v10/modules/core/api"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"
	solomachine "github.com/cosmos/ibc-go/v10/modules/light-clients/06-solomachine"
	ibctm "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"

	// this line is used by starport scaffolding # ibc/app/import
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
)

// registerIBCModules register IBC keepers and non dependency inject modules.
func (app *App) registerIBCModules(
	appOpts servertypes.AppOptions,
	wasmOpts ...wasmkeeper.Option,
) error {
	// set up non depinject support modules store keys
	if err := app.RegisterStores(
		storetypes.NewKVStoreKey(ibcexported.StoreKey),
		storetypes.NewKVStoreKey(ibctransfertypes.StoreKey),
		storetypes.NewKVStoreKey(icahosttypes.StoreKey),
		storetypes.NewKVStoreKey(pfmtypes.StoreKey),
		storetypes.NewKVStoreKey(icacontrollertypes.StoreKey),
		storetypes.NewTransientStoreKey(paramstypes.TStoreKey),
	); err != nil {
		return err
	}

	// register the key tables for legacy param subspaces
	keyTable := ibcclienttypes.ParamKeyTable()
	keyTable.RegisterParamSet(&ibcconnectiontypes.Params{})
	app.ParamsKeeper.Subspace(ibcexported.ModuleName).WithKeyTable(keyTable)
	app.ParamsKeeper.Subspace(pfmtypes.ModuleName)
	app.ParamsKeeper.Subspace(ibctransfertypes.ModuleName).WithKeyTable(ibctransfertypes.ParamKeyTable())
	app.ParamsKeeper.Subspace(icacontrollertypes.SubModuleName).WithKeyTable(icacontrollertypes.ParamKeyTable())
	app.ParamsKeeper.Subspace(icahosttypes.SubModuleName).WithKeyTable(icahosttypes.ParamKeyTable())

	govAuthority, _ := app.AuthKeeper.AddressCodec().BytesToString(authtypes.NewModuleAddress(govtypes.ModuleName))

	// Create IBC keeper
	app.IBCKeeper = ibckeeper.NewKeeper(
		app.appCodec,
		runtime.NewKVStoreService(app.GetKey(ibcexported.StoreKey)),
		app.GetSubspace(ibcexported.ModuleName),
		app.UpgradeKeeper,
		govAuthority,
	)

	// Initialize the packet forward middleware Keeper
	// It's important to note that the PFM Keeper must be initialized before the Transfer Keeper
	app.PacketForwardKeeper = pfmkeeper.NewKeeper(
		app.appCodec,
		runtime.NewKVStoreService(app.GetKey(pfmtypes.StoreKey)),
		nil, // will be zero-value here, reference is set later on with SetTransferKeeper.
		app.IBCKeeper.ChannelKeeper,
		app.BankKeeper,
		app.IBCKeeper.ChannelKeeper, // ICS4Wrapper
		govAuthority,
	)

	// Create IBC transfer keeper
	app.TransferKeeper = ibctransferkeeper.NewKeeper(
		app.appCodec,
		runtime.NewKVStoreService(app.GetKey(ibctransfertypes.StoreKey)),
		app.GetSubspace(ibctransfertypes.ModuleName),
		app.IBCKeeper.ChannelKeeper, // ICS4Wrapper
		app.IBCKeeper.ChannelKeeper,
		app.MsgServiceRouter(),
		app.AuthKeeper,
		app.BankKeeper,
		govAuthority,
	)
	app.PacketForwardKeeper.SetTransferKeeper(app.TransferKeeper)

	// Create interchain account keepers
	app.ICAHostKeeper = icahostkeeper.NewKeeper(
		app.appCodec,
		runtime.NewKVStoreService(app.GetKey(icahosttypes.StoreKey)),
		app.GetSubspace(icahosttypes.SubModuleName),
		app.IBCKeeper.ChannelKeeper, // ICS4Wrapper
		app.IBCKeeper.ChannelKeeper,
		app.AuthKeeper,
		app.MsgServiceRouter(),
		app.GRPCQueryRouter(),
		govAuthority,
	)

	app.ICAControllerKeeper = icacontrollerkeeper.NewKeeper(
		app.appCodec,
		runtime.NewKVStoreService(app.GetKey(icacontrollertypes.StoreKey)),
		app.GetSubspace(icacontrollertypes.SubModuleName),
		app.IBCKeeper.ChannelKeeper,
		app.IBCKeeper.ChannelKeeper,
		app.MsgServiceRouter(),
		govAuthority,
	)

	ibcRouterV2 := ibcapi.NewRouter()

	// Wasm module
	wasmStackIBCHandler, err := app.registerWasmModules(appOpts, wasmOpts...)
	if err != nil {
		return err
	}

	// Action module
	// TODO: Register the action module with the IBC router
	// This is a placeholder for the IBC module for the action module that should be created
	// actionIBCModule := actionmodule.NewIBCModule(app.appCodec, app.ActionKeeper)

	// Create Transfer Stack
	var ibcv1transferStack ibcporttypes.IBCModule
	ibcv1transferStack = ibctransfer.NewIBCModule(app.TransferKeeper)
	// callbacks wraps the transfer stack as its base app, and uses PacketForwardKeeper as the ICS4Wrapper
	// i.e. packet-forward-middleware is higher on the stack and sits between callbacks and the ibc channel keeper
	// Since this is the lowest level middleware of the transfer stack, it should be the first entrypoint for transfer keeper's
	// WriteAcknowledgement.
	ibccbStack := ibccallbacks.NewIBCMiddleware(
		ibcv1transferStack,
		app.PacketForwardKeeper,
		wasmStackIBCHandler,
		lcfg.DefaultMaxIBCCallbackGas,
	)
	ibcv1transferStack = pfm.NewIBCMiddleware(
		ibccbStack,
		app.PacketForwardKeeper,
		0,
		pfmkeeper.DefaultForwardTransferPacketTimeoutTimestamp,
	)

	var ibcv2transferStack ibcapi.IBCModule
	ibcv2transferStack = ibctransferv2.NewIBCModule(app.TransferKeeper)
	ibcv2transferStack = ibccallbacksv2.NewIBCMiddleware(
		ibcv2transferStack,
		app.IBCKeeper.ChannelKeeperV2,
		wasmStackIBCHandler,
		app.IBCKeeper.ChannelKeeperV2,
		lcfg.DefaultMaxIBCCallbackGas,
	)
	app.TransferKeeper.WithICS4Wrapper(ibccbStack)

	// RecvPacket, message that originates from core IBC and goes down to app, the flow is:
	// channel.RecvPacket -> icaHost.OnRecvPacket
	icaHostStack := icahost.NewIBCModule(app.ICAHostKeeper)

	// Create Interchain Accounts Controller Stack
	// SendPacket, since it is originating from the application to core IBC:
	// icaAuthModuleKeeper.SendTx -> icaController.SendPacket -> fee.SendPacket -> channel.SendPacket
	var icaControllerStack ibcporttypes.IBCModule
	// integration point for custom authentication modules
	// see https://medium.com/the-interchain-foundation/ibc-go-v6-changes-to-interchain-accounts-and-how-it-impacts-your-chain-806c185300d7
	icaControllerStack = icacontroller.NewIBCMiddleware(app.ICAControllerKeeper)
	icaControllerStack = ibccallbacks.NewIBCMiddleware(
		icaControllerStack,
		app.IBCKeeper.ChannelKeeper, // ICS4Wrapper
		wasmStackIBCHandler,
		lcfg.DefaultMaxIBCCallbackGas,
	)
	icaICS4Wrapper := icaControllerStack.(ibcporttypes.ICS4Wrapper)
	// Since the callbacks middleware itself is an ics4wrapper, it needs to be passed to the ica controller keeper
	app.ICAControllerKeeper.WithICS4Wrapper(icaICS4Wrapper)

	// create static IBC router, add transfer route, then set it on the keeper
	ibcRouter := ibcporttypes.NewRouter().
		AddRoute(ibctransfertypes.ModuleName, ibcv1transferStack).
		AddRoute(wasmtypes.ModuleName, wasmStackIBCHandler).
		AddRoute(icacontrollertypes.SubModuleName, icaControllerStack).
		AddRoute(icahosttypes.SubModuleName, icaHostStack)
		// TODO: Uncomment the following line when the IBC module for the action module is implemented
		//		AddRoute(actiontypes.ModuleName, actionIBCModule)

	// Additional IBC modules can be registered here
	if v := appOpts.Get(IBCModuleRegisterFnOption); v != nil {
		if registerFn, ok := v.(func(router *ibcporttypes.Router)); ok {
			registerFn(ibcRouter)
		} else {
			return errors.New("invalid IBC module register function option")
		}
	}

	app.IBCKeeper.SetRouter(ibcRouter)
	app.ibcRouter = ibcRouter

	ibcRouterV2 = ibcRouterV2.
		AddRoute(ibctransfertypes.PortID, ibcv2transferStack).
		AddPrefixRoute(wasmkeeper.PortIDPrefixV2, wasmkeeper.NewIBC2Handler(app.WasmKeeper))

	// Additional IBC v2 modules can be registered here.
	if v := appOpts.Get(IBCModuleRegisterFnOptionV2); v != nil {
		if registerFn, ok := v.(func(router *ibcapi.Router)); ok {
			registerFn(ibcRouterV2)
		} else {
			return errors.New("invalid IBC v2 module register function option")
		}
	}
	app.IBCKeeper.SetRouterV2(ibcRouterV2)

	clientKeeper := app.IBCKeeper.ClientKeeper
	storeProvider := clientKeeper.GetStoreProvider()

	tmLightClientModule := ibctm.NewLightClientModule(app.appCodec, storeProvider)
	clientKeeper.AddRoute(ibctm.ModuleName, &tmLightClientModule)

	soloLightClientModule := solomachine.NewLightClientModule(app.appCodec, storeProvider)
	clientKeeper.AddRoute(solomachine.ModuleName, &soloLightClientModule)

	// register IBC modules
	if err := app.RegisterModules(
		ibc.NewAppModule(app.IBCKeeper),
		pfm.NewAppModule(app.PacketForwardKeeper, app.GetSubspace(pfmtypes.ModuleName)),
		ibctransfer.NewAppModule(app.TransferKeeper),
		icamodule.NewAppModule(&app.ICAControllerKeeper, &app.ICAHostKeeper),
		ibctm.NewAppModule(tmLightClientModule),
		solomachine.NewAppModule(soloLightClientModule),
	); err != nil {
		return err
	}

	return nil
}

// RegisterIBC Since the IBC modules don't support dependency injection,
// we need to manually register the modules on the client side.
// This needs to be removed after IBC supports App Wiring.
func RegisterIBC(cdc codec.Codec) map[string]appmodule.AppModule {
	modules := map[string]appmodule.AppModule{
		ibcexported.ModuleName:      ibc.NewAppModule(&ibckeeper.Keeper{}),
		ibctransfertypes.ModuleName: ibctransfer.NewAppModule(ibctransferkeeper.Keeper{}),
		icatypes.ModuleName:         icamodule.NewAppModule(&icacontrollerkeeper.Keeper{}, &icahostkeeper.Keeper{}),
		ibctm.ModuleName:            ibctm.NewAppModule(ibctm.NewLightClientModule(cdc, ibcclienttypes.StoreProvider{})),
		solomachine.ModuleName:      solomachine.NewAppModule(solomachine.NewLightClientModule(cdc, ibcclienttypes.StoreProvider{})),
	}

	for _, m := range modules {
		if mr, ok := m.(module.AppModuleBasic); ok {
			mr.RegisterInterfaces(cdc.InterfaceRegistry())
		}
	}

	return modules
}

// isInterchainAccount reports whether the provided account is an ICA account.
// Useful for modules that need to branch behavior for ICS-27-controlled accounts.
func isInterchainAccount(account sdk.AccountI) bool {
	_, ok := account.(*icatypes.InterchainAccount)
	return ok
}

// isInterchainAccountAddr reports whether the address resolves to an ICA account.
func isInterchainAccountAddr(ctx context.Context, ak authkeeper.AccountKeeper, addr sdk.AccAddress) bool {
	acct := ak.GetAccount(ctx, addr)
	if acct == nil {
		return false
	}
	return isInterchainAccount(acct)
}
