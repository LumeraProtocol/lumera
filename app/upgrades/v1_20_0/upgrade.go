package v1_20_0

import (
	"context"
	"fmt"
	"time"

	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	appevm "github.com/LumeraProtocol/lumera/app/evm"
	appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
	lcfg "github.com/LumeraProtocol/lumera/config"
	erc20policytypes "github.com/LumeraProtocol/lumera/x/erc20policy/types"
	evmigrationtypes "github.com/LumeraProtocol/lumera/x/evmigration/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// UpgradeName is the on-chain name used for this upgrade.
const UpgradeName = "v1.20.0"

// All networks derive a finite migration_end_time automatically from the
// upgrade block time so they run against a real deadline without hardcoding an
// absolute timestamp. Devnet uses a short fixed window for rehearsals; testnet
// and mainnet both run against a 3-calendar-month window measured from the
// upgrade block.
const devnetMigrationWindow = 2 * 24 * time.Hour

// autoMigrationEndTime returns the migration_end_time to auto-apply for the
// given chain ID and upgrade block time, plus whether a deadline should be set
// at all. Unrecognized chain IDs leave the deadline unset (ok == false).
//
// A calendar-month offset (AddDate) is used for testnet/mainnet rather than a
// fixed time.Duration because "3 months" is not a constant number of hours; it
// must be applied to the block time directly so month lengths and leap years
// are handled correctly.
func autoMigrationEndTime(chainID string, blockTime time.Time) (time.Time, bool) {
	switch {
	case lcfg.IsDevnetChainID(chainID):
		return blockTime.Add(devnetMigrationWindow), true
	case lcfg.IsTestnetChainID(chainID), lcfg.IsMainnetChainID(chainID):
		return blockTime.AddDate(0, 3, 0), true
	default:
		return time.Time{}, false
	}
}

// StoreUpgrades declares store additions for this upgrade.
var StoreUpgrades = storetypes.StoreUpgrades{
	Added: []string{
		feemarkettypes.StoreKey,   // added EVM fee market store key
		precisebanktypes.StoreKey, // added EVM precise bank store key
		evmtypes.StoreKey,         // added EVM state store key
		erc20types.StoreKey,       // added ERC20 token pairs store key
		evmigrationtypes.StoreKey, // added EVM migration store key
	},
}

// CreateUpgradeHandler executes v1.20.0 migrations and finalizes Lumera-specific
// EVM params so upgraded chains don't retain upstream atom defaults.
func CreateUpgradeHandler(p appParams.AppUpgradeParams) upgradetypes.UpgradeHandler {
	return func(goCtx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		p.Logger.Info(fmt.Sprintf("Starting upgrade %s...", UpgradeName))
		ctx := sdk.UnwrapSDKContext(goCtx)

		if p.BankKeeper == nil {
			return nil, fmt.Errorf("%s upgrade requires bank keeper to be wired", UpgradeName)
		}

		// Ensure both chain-native metadata and a legacy atom-style fallback are present
		// before RunMigrations initializes newly-added EVM modules.
		upserted := lcfg.UpsertChainBankMetadata(p.BankKeeper.GetAllDenomMetaData(ctx))
		for _, md := range upserted {
			p.BankKeeper.SetDenomMetaData(ctx, md)
		}

		legacyExtendedDenom := lcfg.ChainEVMExtendedDenom // Lumera extended denom: alume
		if !p.BankKeeper.HasDenomMetaData(ctx, legacyExtendedDenom) {
			p.BankKeeper.SetDenomMetaData(ctx, banktypes.Metadata{
				Description: "Legacy fallback metadata for EVM upgrade compatibility",
				DenomUnits: []*banktypes.DenomUnit{
					{Denom: legacyExtendedDenom, Exponent: 0, Aliases: []string{"atto" + lcfg.ChainDisplayDenom}},
					{Denom: lcfg.ChainDisplayDenom, Exponent: 18},
				},
				Base:    legacyExtendedDenom,
				Display: lcfg.ChainDisplayDenom,
				Name:    lcfg.ChainTokenName,
				Symbol:  lcfg.ChainTokenSymbol,
			})
		}
		// Skip RunMigrations' default InitGenesis for EVM modules.
		// cosmos/evm v0.6.0's DefaultParams() sets EvmDenom=DefaultEVMExtendedDenom ("aatom"),
		// which would pollute the EVM coin info KV store with the wrong denom.
		// We initialize all EVM module state manually below with Lumera-specific params.
		// Per Cosmos SDK docs, setting fromVM[module] = ConsensusVersion skips InitGenesis.
		fromVM[evmtypes.ModuleName] = 1
		fromVM[feemarkettypes.ModuleName] = 1
		fromVM[precisebanktypes.ModuleName] = 1
		fromVM[erc20types.ModuleName] = 1

		p.Logger.Info("Running module migrations...")
		newVM, err := p.ModuleManager.RunMigrations(ctx, p.Configurator, fromVM)
		if err != nil {
			p.Logger.Error("Failed to run migrations", "error", err)
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}
		p.Logger.Info("Module migrations completed.")

		if p.EVMKeeper == nil || p.FeeMarketKeeper == nil || p.Erc20Keeper == nil {
			return nil, fmt.Errorf("%s upgrade requires EVM, feemarket, and erc20 keepers to be wired", UpgradeName)
		}

		lumeraEVMGenesis := appevm.LumeraEVMGenesisState()
		if err := p.EVMKeeper.SetParams(ctx, lumeraEVMGenesis.Params); err != nil {
			return nil, fmt.Errorf("set evm params: %w", err)
		}
		if err := p.EVMKeeper.InitEvmCoinInfo(ctx); err != nil {
			return nil, fmt.Errorf("init evm coin info: %w", err)
		}

		lumeraFeeMarketGenesis := appevm.LumeraFeemarketGenesisState()
		if err := p.FeeMarketKeeper.SetParams(ctx, lumeraFeeMarketGenesis.Params); err != nil {
			return nil, fmt.Errorf("set feemarket params: %w", err)
		}

		// erc20 InitGenesis is skipped above together with the other EVM modules.
		// Unlike precisebank, erc20 persists module params in its own KV store, so
		// an empty store would otherwise read back as both booleans=false.
		if err := p.Erc20Keeper.SetParams(ctx, appevm.LumeraERC20DefaultParams()); err != nil {
			return nil, fmt.Errorf("set erc20 default params: %w", err)
		}

		// Initialize the ERC20 IBC auto-registration policy. On fresh genesis this
		// is handled by initERC20PolicyDefaults in InitChainer, but upgrade paths
		// skip InitChainer so the policy must be seeded here.
		if p.Erc20StoreKey != nil {
			erc20Store := ctx.KVStore(p.Erc20StoreKey)
			if !erc20Store.Has(erc20policytypes.PolicyModeKey) {
				erc20Store.Set(erc20policytypes.PolicyModeKey, []byte(erc20policytypes.PolicyModeAllowlist))
				tracePfxStore := prefix.NewStore(erc20Store, erc20policytypes.PolicyAllowBaseTracePfx)
				for _, entry := range erc20policytypes.DefaultAllowedBaseDenomTraces {
					traceKey := erc20policytypes.EncodeTraceKey(entry.Trace)
					key := append([]byte(entry.BaseDenom), 0x00)
					key = append(key, traceKey...)
					tracePfxStore.Set(key, []byte{1})
				}
				p.Logger.Info("Initialized ERC20 registration policy", "mode", erc20policytypes.PolicyModeAllowlist,
					"base_denom_traces", len(erc20policytypes.DefaultAllowedBaseDenomTraces))
			}
		}

		// Derive a finite migration_end_time from the upgrade block time so the
		// network runs against a real deadline without hardcoding an absolute
		// timestamp. RunMigrations already seeded the evmigration module with
		// default params (enable_migration=true, migration_end_time=0); here we
		// only override the deadline. Devnet gets a short rehearsal window;
		// testnet and mainnet both get a 3-calendar-month window.
		//
		// The network is identified from the SDK context (ctx.ChainID()), which
		// carries the genesis-derived chain ID from the block header. We must not
		// use the app-level ChainID captured during setupUpgrades: that value
		// comes from the --chain-id flag, which defaults to the non-empty
		// "lumera" and so never falls back to genesis, leaving mainnet's deadline
		// silently unset on the common `lumerad start` path.
		if endTime, ok := autoMigrationEndTime(ctx.ChainID(), ctx.BlockTime()); ok {
			if p.EvmigrationKeeper == nil {
				return nil, fmt.Errorf("%s upgrade requires evmigration keeper to be wired", UpgradeName)
			}
			emParams, err := p.EvmigrationKeeper.Params.Get(ctx)
			if err != nil {
				return nil, fmt.Errorf("get evmigration params: %w", err)
			}
			emParams.MigrationEndTime = endTime.Unix()
			if err := p.EvmigrationKeeper.Params.Set(ctx, emParams); err != nil {
				return nil, fmt.Errorf("set evmigration migration_end_time: %w", err)
			}
			p.Logger.Info("Set migration_end_time from upgrade block time",
				"chain_id", ctx.ChainID(),
				"migration_end_time", emParams.MigrationEndTime,
			)
		}

		p.Logger.Info(fmt.Sprintf("Successfully completed upgrade %s", UpgradeName))
		return newVM, nil
	}
}
