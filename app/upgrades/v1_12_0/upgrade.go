package v1_12_0

import (
	"context"
	"fmt"

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
	evmigrationtypes "github.com/LumeraProtocol/lumera/x/evmigration/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// UpgradeName is the on-chain name used for this upgrade.
const UpgradeName = "v1.12.0"

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

// CreateUpgradeHandler executes v1.12.0 migrations and finalizes Lumera-specific
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
		p.Erc20Keeper.SetParams(ctx, erc20types.DefaultParams())

		p.Logger.Info(fmt.Sprintf("Successfully completed upgrade %s", UpgradeName))
		return newVM, nil
	}
}
