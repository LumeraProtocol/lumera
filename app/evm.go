package app

import (
	"encoding/json"

	storetypes "cosmossdk.io/store/types"
	actionprecompile "github.com/LumeraProtocol/lumera/precompiles/action"
	supernodeprecompile "github.com/LumeraProtocol/lumera/precompiles/supernode"
	precompiletypes "github.com/cosmos/evm/precompiles/types"

	"github.com/spf13/cast"

	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	erc20keeper "github.com/cosmos/evm/x/erc20/keeper"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarketkeeper "github.com/cosmos/evm/x/feemarket/keeper"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	precisebankkeeper "github.com/cosmos/evm/x/precisebank/keeper"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	evmkeeper "github.com/cosmos/evm/x/vm/keeper"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	srvflags "github.com/cosmos/evm/server/flags"

	erc20module "github.com/cosmos/evm/x/erc20"
	feemarket "github.com/cosmos/evm/x/feemarket"
	precisebank "github.com/cosmos/evm/x/precisebank"
	evmmodule "github.com/cosmos/evm/x/vm"

	appevm "github.com/LumeraProtocol/lumera/app/evm"
	lcfg "github.com/LumeraProtocol/lumera/config"
)

// registerEVMModules registers EVM-related keepers and non-depinject modules.
// This follows the same pattern as registerIBCModules for manually wired modules.
func (app *App) registerEVMModules(appOpts servertypes.AppOptions) error {
	// Register store keys for EVM modules.
	if err := app.RegisterStores(
		// EVM-related module store keys.
		storetypes.NewKVStoreKey(feemarkettypes.StoreKey),
		storetypes.NewKVStoreKey(precisebanktypes.StoreKey),
		storetypes.NewKVStoreKey(evmtypes.StoreKey),
		storetypes.NewKVStoreKey(erc20types.StoreKey),
		// EVM-related module transient store keys.
		storetypes.NewTransientStoreKey(feemarkettypes.TransientKey),
		storetypes.NewTransientStoreKey(evmtypes.TransientKey),
	); err != nil {
		return err
	}

	govAuthority := authtypes.NewModuleAddress(govtypes.ModuleName)

	// Create FeeMarket keeper.
	app.FeeMarketKeeper = feemarketkeeper.NewKeeper(
		app.appCodec,
		govAuthority,
		app.GetKey(feemarkettypes.StoreKey),
		app.GetTransientKey(feemarkettypes.TransientKey),
	)

	// Create PreciseBank keeper.
	app.PreciseBankKeeper = precisebankkeeper.NewKeeper(
		app.appCodec,
		app.GetKey(precisebanktypes.StoreKey),
		app.BankKeeper,
		app.AuthKeeper,
	)

	// Read the EVM tracer from app.toml [evm] section / --evm.tracer flag.
	// Valid values: "json", "struct", "access_list", "markdown", or "" (disabled).
	// When set, enables debug_traceTransaction and related JSON-RPC methods.
	evmTracer := cast.ToString(appOpts.Get(srvflags.EVMTracer))

	// Create EVM (x/vm) keeper.
	// Pass &app.Erc20Keeper (pointer to App field) to resolve the circular dependency:
	// EVMKeeper needs Erc20Keeper for ERC20 precompiles, and Erc20Keeper needs EVMKeeper
	// for contract calls. The pointer remains valid after Erc20Keeper is populated below.
	app.EVMKeeper = evmkeeper.NewKeeper(
		app.appCodec,
		app.GetKey(evmtypes.StoreKey),
		app.GetTransientKey(evmtypes.TransientKey),
		app.kvStoreKeys(),
		govAuthority,
		app.AuthKeeper,
		app.PreciseBankKeeper, // PreciseBank wraps Bank with multi-decimal support
		app.StakingKeeper,
		app.FeeMarketKeeper,
		&app.ConsensusParamsKeeper,
		&app.Erc20Keeper, // pointer back-ref, populated below
		lcfg.EVMChainID,  // Lumera EVM chain ID
		evmTracer,        // tracer from app.toml / --evm.tracer flag
	)

	// Set default EVM coin info (production only — see evm/defaults_prod.go / defaults_testbuild.go).
	appevm.SetKeeperDefaults(app.EVMKeeper)

	// Create ERC20 keeper and populate app.Erc20Keeper (the EVMKeeper already holds
	// &app.Erc20Keeper, so this assignment makes precompiles available).
	// We pass &app.TransferKeeper so ERC20 precompiles and IBC callbacks can use
	// transfer functionality once registerIBCModules initializes this keeper.
	app.Erc20Keeper = erc20keeper.NewKeeper(
		app.GetKey(erc20types.StoreKey),
		app.appCodec,
		govAuthority,
		app.AuthKeeper,
		app.BankKeeper,
		app.EVMKeeper,
		app.StakingKeeper,
		&app.TransferKeeper, // pointer to resolve circular dependency with IBC transfer keeper
	)

	// Register EVM modules.
	if err := app.RegisterModules(
		feemarket.NewAppModule(app.FeeMarketKeeper),
		precisebank.NewAppModule(app.PreciseBankKeeper, app.BankKeeper, app.AuthKeeper),
		evmmodule.NewAppModule(app.EVMKeeper, app.AuthKeeper, app.BankKeeper, app.AuthKeeper.AddressCodec()),
		erc20module.NewAppModule(app.Erc20Keeper, app.AuthKeeper),
	); err != nil {
		return err
	}

	return nil
}

// syncEVMStoreKeys adds any KV store keys that were registered after the EVM
// keeper was created (e.g. IBC stores from registerIBCModules) into the keeper's
// store key map. The EVM's snapshot multi-store reads this map lazily when
// creating a StateDB, so keys added here are visible to precompile execution.
func (app *App) syncEVMStoreKeys() {
	evmKeys := app.EVMKeeper.KVStoreKeys()
	for _, k := range app.GetStoreKeys() {
		kv, ok := k.(*storetypes.KVStoreKey)
		if !ok {
			continue
		}
		if _, exists := evmKeys[kv.Name()]; !exists {
			evmKeys[kv.Name()] = kv
		}
	}
}

// configureEVMStaticPrecompiles wires Cosmos EVM's static precompile registry
// once all keepers are initialized (including IBC transfer/channel keepers).
func (app *App) configureEVMStaticPrecompiles() {
	// Get default cosmos-evm precompiles (bank, staking, distribution, etc.)
	precompiles := precompiletypes.DefaultStaticPrecompiles(
		*app.StakingKeeper,
		app.DistrKeeper,
		app.PreciseBankKeeper,
		&app.Erc20Keeper,
		&app.TransferKeeper,
		app.IBCKeeper.ChannelKeeper,
		*app.GovKeeper,
		app.SlashingKeeper,
		app.appCodec,
	)

	// Register Lumera custom precompile: Action module
	actionPC := actionprecompile.NewPrecompile(
		app.ActionKeeper,
		app.PreciseBankKeeper,
		app.AuthKeeper.AddressCodec(),
	)
	precompiles[actionPC.Address()] = actionPC

	// Register Lumera custom precompile: Supernode module
	supernodePC := supernodeprecompile.NewPrecompile(
		app.SupernodeKeeper,
		app.PreciseBankKeeper,
		app.AuthKeeper.AddressCodec(),
	)
	precompiles[supernodePC.Address()] = supernodePC

	app.EVMKeeper.WithStaticPrecompiles(precompiles)
}

// DefaultGenesis overrides the runtime.App default genesis to patch EVM-related
// module genesis states with Lumera-specific values:
//   - EVM (x/vm): uses Lumera denominations instead of upstream defaults (uatom/aatom)
//   - Feemarket: enables EIP-1559 dynamic base fee with Lumera default base fee
func (app *App) DefaultGenesis() map[string]json.RawMessage {
	genesis := app.App.DefaultGenesis()

	var bankGenesis banktypes.GenesisState
	app.appCodec.MustUnmarshalJSON(genesis[banktypes.ModuleName], &bankGenesis)
	bankGenesis.DenomMetadata = lcfg.UpsertChainBankMetadata(bankGenesis.DenomMetadata)
	genesis[banktypes.ModuleName] = app.appCodec.MustMarshalJSON(&bankGenesis)
	// override EVM and feemarket genesis with Lumera-specific defaults
	genesis[evmtypes.ModuleName] = app.appCodec.MustMarshalJSON(appevm.LumeraEVMGenesisState())
	genesis[feemarkettypes.ModuleName] = app.appCodec.MustMarshalJSON(appevm.LumeraFeemarketGenesisState())
	return genesis
}
