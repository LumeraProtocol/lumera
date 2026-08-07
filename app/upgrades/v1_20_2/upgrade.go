package v1_20_2

import (
	"context"
	"fmt"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	appevm "github.com/LumeraProtocol/lumera/app/evm"
	appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
	upgrade_v1_20_0 "github.com/LumeraProtocol/lumera/app/upgrades/v1_20_0"
	upgrade_v1_20_1 "github.com/LumeraProtocol/lumera/app/upgrades/v1_20_1"
)

// UpgradeName is the on-chain name used for this upgrade.
//
// This constant -- not the git tag -- is what the governance proposal --name,
// the cosmovisor upgrades/<NAME>/ directory, and `q upgrade applied <NAME>` all
// key off. Keep them identical.
const UpgradeName = "v1.20.2"

// v1.20.2 is the coordinated consensus activation boundary for the evmigration
// SuperNode-ownership and continuity fixes.
//
// # WHY THIS UPGRADE EXISTS
//
// The fixes in this release change DeliverTx outcomes for the SAME migration
// transaction:
//
//   - old code rewrote existing PrevSupernodeAccounts rows from the legacy
//     account to the destination; new code preserves them and appends exactly
//     one transition;
//   - old code resolved SuperNode ownership by literal text and accepted stale
//     or duplicate index states; new code resolves canonically and rejects them;
//   - old code left the validator-keyed Everlight SNDistState behind on an
//     operator-address change; new code moves it with the validator.
//
// Two validators running different binaries would therefore commit different
// state for the same block. That is an app-hash divergence, so this release
// MUST NOT be rolled out as a node-by-node binary replacement while migration
// transactions can execute. It needs one named halt height that every validator
// stops at, exactly like v1.20.0 and v1.20.1.
//
// There is no store migration and no module consensus-version bump here. In
// addition to making the behavior change atomic across the validator set, the
// handler applies the release's configured feemarket base fee.
//
// # TWO ARRIVAL SHAPES, ONE BINARY
//
// Live state verified on 2026-08-01:
//
//	lumera-mainnet-1   app_version 1.12.0   audit v2   EVM stack ABSENT
//	lumera-testnet-2   app_version 1.20.1   audit v2   EVM stack PRESENT
//
// Testnet has already executed both v1.20.0 and v1.20.1, and an upgrade name
// cannot run twice, so those handlers can no longer carry anything to it.
// Mainnet is still pre-EVM and has executed neither. This one binary must
// therefore be correct for two very different starting states:
//
//	from 1.20.1 (testnet)  EVM present -> migrations + base-fee update
//	from 1.12.0 (mainnet)  EVM absent  -> full EVM bring-up + base-fee update
//
// Both are decided by inspecting committed STATE (fromVM), never by chain-id,
// so a network that arrives by an unexpected path still converges on the same
// result. This mirrors v1.20.1 deliberately: the two upgrades must not drift.
//
// The bring-up branch is not optional defensiveness. Skipping it on a pre-EVM
// chain panics during upgrade with:
//
//	panic: error initializing evm coin info: denom metadata aatom could not
//	be found
//
// because cosmos/evm's defaults assume the upstream atom denom. The matching
// store additions (declared via v1_20_0.StoreUpgrades in SetupUpgrades, mounted
// by the add-only store loader) prevent the companion failure:
//
//	panic: failed to load latest version: version of store evmigration
//	mismatch root store's version; expected 155 got 0
//
// Both were observed on a faithful 1.12.0 mainnet-shaped devnet replica, not
// derived from reading the code.
var evmBringUpModules = []string{
	evmtypes.ModuleName,
	feemarkettypes.ModuleName,
	precisebanktypes.ModuleName,
	erc20types.ModuleName,
}

// evmModuleState partitions evmBringUpModules by their presence in fromVM.
// Because v1.20.0 registers all four atomically, "some but not all present" is
// not a state any correct upgrade path can produce.
func evmModuleState(fromVM module.VersionMap) (present, absent []string) {
	for _, name := range evmBringUpModules {
		if _, ok := fromVM[name]; ok {
			present = append(present, name)
		} else {
			absent = append(absent, name)
		}
	}
	return present, absent
}

// CreateUpgradeHandler returns the state-driven v1.20.2 handler.
//
// When the EVM stack is absent it delegates to the full v1.20.0 bring-up, which
// upserts bank denom metadata, finalizes Lumera EVM params, initializes EVM coin
// info, seeds the ERC20 registration policy, derives migration_end_time from the
// upgrade block time, and then runs migrations.
//
// When the EVM stack is present this runs migrations without re-running EVM
// bring-up. Both paths then set the configured feemarket base fee so chains
// arriving from v1.12.0 and v1.20.1 converge on the same value.
//
// Partial EVM state fails closed rather than guessing.
func CreateUpgradeHandler(p appParams.AppUpgradeParams) upgradetypes.UpgradeHandler {
	return func(goCtx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		present, absent := evmModuleState(fromVM)

		switch {
		case len(present) == 0:
			p.Logger.Info(fmt.Sprintf(
				"Starting upgrade %s: EVM not yet initialized, running full v1.20.0 bring-up", UpgradeName))
		case len(absent) > 0:
			// Neither branch is safe here: the bring-up would double-initialize
			// the modules that are present, and the existing-EVM path would
			// skip param finalization for the ones that are absent.
			return nil, fmt.Errorf(
				"%s: inconsistent EVM module state, refusing to run — present=%v absent=%v; "+
					"expected all EVM modules present or all absent (full bring-up)",
				UpgradeName, present, absent,
			)
		default:
			p.Logger.Info(fmt.Sprintf(
				"Starting upgrade %s: EVM already initialized, running migrations and updating the feemarket base fee", UpgradeName))
		}
		if p.FeeMarketKeeper == nil {
			return nil, fmt.Errorf("%s upgrade requires the feemarket keeper to be wired", UpgradeName)
		}

		// Both surviving shapes are exactly what v1.20.1 already implements and
		// has been rehearsed on. Delegate rather than duplicating the branch, so
		// the two upgrades cannot drift apart in a later edit.
		newVM, err := upgrade_v1_20_1.CreateUpgradeHandler(p)(goCtx, plan, fromVM)
		if err != nil {
			p.Logger.Error(fmt.Sprintf("Upgrade %s failed", UpgradeName), "error", err)
			return nil, err
		}

		ctx := sdk.UnwrapSDKContext(goCtx)
		feeMarketParams := p.FeeMarketKeeper.GetParams(ctx)
		feeMarketParams.BaseFee = appevm.LumeraFeemarketGenesisState().Params.BaseFee
		if err := p.FeeMarketKeeper.SetParams(ctx, feeMarketParams); err != nil {
			return nil, fmt.Errorf("set v1.20.2 feemarket base fee: %w", err)
		}
		p.Logger.Info("Updated feemarket base fee", "base_fee", feeMarketParams.BaseFee.String())

		p.Logger.Info(fmt.Sprintf("Successfully completed upgrade %s", UpgradeName))
		return newVM, nil
	}
}

// StoreUpgrades is intentionally the SAME declaration v1.20.0 and v1.20.1 use.
//
// It is not a second, drifting copy: the add-only store loader mounts only the
// declared keys that are missing from committed state and never deletes a store,
// so declaring the full EVM set is a no-op on a chain that already ran v1.20.0
// (testnet) and mounts feemarket/precisebank/vm/erc20/evmigration on a direct
// 1.12.0 one-hop (mainnet).
var StoreUpgrades = upgrade_v1_20_0.StoreUpgrades
