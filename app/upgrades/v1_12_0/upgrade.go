package v1_12_0

import (
	"context"
	"fmt"

	upgradetypes "cosmossdk.io/x/upgrade/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	consensusparams "github.com/LumeraProtocol/lumera/app/upgrades/internal/consensusparams"
	appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
	audittypes "github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// UpgradeName is the on-chain name used for this upgrade.
const UpgradeName = "v1.12.0"

// Activation-time policy constants.
//
// These are written exactly once during the upgrade; thereafter, params are
// governed by MsgUpdateParams.
const (
	// everlightEnabled is informational: Everlight payout logic activates as
	// soon as supernode params carry a non-zero RewardDistribution. We persist
	// the default RewardDistribution explicitly in this handler so the very
	// first post-upgrade query returns the canonical values.
	everlightEnabled = true

	// storageTruthEnforcementMode is the LEP-6 enforcement mode burned in at
	// activation. SHADOW means evidence is collected but no enforcement actions
	// (postpone, slash) are taken. Future modes (SOFT, FULL) require an
	// explicit governance MsgUpdateParams.
	storageTruthEnforcementMode = audittypes.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_SHADOW
)

// CreateUpgradeHandler runs migrations and persists default values for params
// and per-chain anchors introduced after v1.11.1-hotfix:
//   - LEP-5 (action): SvcChallengeCount, SvcMinChunksForChallenge.
//   - Everlight + LEP-4 (supernode): RewardDistribution, MetricsUpdateIntervalBlocks,
//     MetricsGracePeriodBlocks, MetricsFreshnessMaxBlocks, MinSupernodeVersion,
//     MinCpu/Mem/Storage, MaxCpu/Mem/StorageUsagePercent, RequiredOpenPorts.
//   - LEP-6 (audit): StorageTruth* params and explicit enforcement mode (SHADOW).
//
// Module ConsensusVersion is unchanged for action and supernode (purely
// additive params with safe defaults). x/audit's v1→v2 migration is invoked by
// RunMigrations and bumps KeepLastEpochEntries to cover new windows.
//
// Per-chain anchors:
//   - Anchors LastDistributionHeight to the upgrade height so the first
//     Everlight payout fires one PaymentPeriodBlocks after activation, not on
//     the very next block.
//   - Calls SupernodeKeeper.EnsureModuleAccount so the supernode ModuleAccount
//     is materialised with its updated permissions (Minter+Burner+Staking).
func CreateUpgradeHandler(p appParams.AppUpgradeParams) upgradetypes.UpgradeHandler {
	return func(goCtx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		p.Logger.Info(fmt.Sprintf("Starting upgrade %s...", UpgradeName))

		ctx := sdk.UnwrapSDKContext(goCtx)

		if err := consensusparams.EnsurePresent(ctx, p, UpgradeName); err != nil {
			return nil, err
		}

		// RunMigrations triggers x/audit v1→v2 migration (KeepLastEpochEntries
		// floor) and any additive InitGenesis for unseen modules.
		p.Logger.Info("Running module migrations...")
		newVM, err := p.ModuleManager.RunMigrations(ctx, p.Configurator, fromVM)
		if err != nil {
			p.Logger.Error("Failed to run migrations", "error", err)
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}
		p.Logger.Info("Module migrations completed.")

		// All four keepers are required by this handler. Fail loudly on any
		// wiring regression rather than nil-deref mid-upgrade.
		if p.ActionKeeper == nil {
			return nil, fmt.Errorf("%s upgrade requires action keeper to be wired", UpgradeName)
		}
		if p.SupernodeKeeper == nil {
			return nil, fmt.Errorf("%s upgrade requires supernode keeper to be wired", UpgradeName)
		}
		if p.AuditKeeper == nil {
			return nil, fmt.Errorf("%s upgrade requires audit keeper to be wired", UpgradeName)
		}

		// 1) Persist LEP-5 action params defaults.
		// keeper.GetParams() now applies WithDefaults() on read and SetParams
		// applies it on write — calling SetParams here burns the canonical
		// values into the on-disk blob so external param queries return them
		// without waiting for the next governance write.
		actionParams := p.ActionKeeper.GetParams(ctx)
		if err := p.ActionKeeper.SetParams(ctx, actionParams); err != nil {
			return nil, fmt.Errorf("persist action params with defaults: %w", err)
		}
		p.Logger.Info("Action params persisted with LEP-5 defaults",
			"svc_challenge_count", actionParams.SvcChallengeCount,
			"svc_min_chunks_for_challenge", actionParams.SvcMinChunksForChallenge,
		)

		// 2) Persist Everlight + LEP-4 supernode params defaults.
		snParams := p.SupernodeKeeper.GetParams(ctx).WithDefaults()
		if err := p.SupernodeKeeper.SetParams(ctx, snParams); err != nil {
			return nil, fmt.Errorf("persist supernode params with defaults: %w", err)
		}
		if snParams.RewardDistribution != nil {
			p.Logger.Info("Supernode reward distribution params persisted",
				"payment_period_blocks", snParams.RewardDistribution.PaymentPeriodBlocks,
				"registration_fee_share_bps", snParams.RewardDistribution.RegistrationFeeShareBps,
				"min_cascade_bytes_for_payment", snParams.RewardDistribution.MinCascadeBytesForPayment,
				"everlight_enabled", everlightEnabled,
			)
		}

		// 3) Anchor Everlight distribution clock at upgrade height so the
		// first payout fires one PaymentPeriodBlocks AFTER the upgrade, not on
		// the next block. Without this, currentHeight - lastDistHeight (=0)
		// >= PaymentPeriodBlocks on every chain past PaymentPeriod blocks tall,
		// triggering an immediate post-upgrade distribution.
		p.SupernodeKeeper.SetLastDistributionHeight(ctx, ctx.BlockHeight())
		p.Logger.Info("Anchored Everlight last distribution height",
			"height", ctx.BlockHeight(),
		)

		// 4) Materialise the supernode ModuleAccount so it carries the
		// updated permissions (Minter+Burner+Staking) declared in app_config
		// instead of a stale BaseAccount or pre-Everlight ModuleAccount entry.
		p.SupernodeKeeper.EnsureModuleAccount(ctx)
		p.Logger.Info("Ensured supernode ModuleAccount with updated permissions")

		// 5) LEP-6 (audit) — burn explicit StorageTruth enforcement mode at
		// activation. WithDefaults intentionally does NOT promote UNSPECIFIED
		// to SHADOW, so we set it here explicitly. The audit v1→v2 migration
		// already ran via RunMigrations and applied StorageTruth* defaults.
		auditParams := p.AuditKeeper.GetParams(ctx)
		auditParams.StorageTruthEnforcementMode = storageTruthEnforcementMode
		if err := p.AuditKeeper.SetParams(ctx, auditParams); err != nil {
			return nil, fmt.Errorf("set audit storage_truth_enforcement_mode: %w", err)
		}
		p.Logger.Info("Audit storage-truth enforcement mode set",
			"mode", storageTruthEnforcementMode.String(),
		)

		p.Logger.Info(fmt.Sprintf("Successfully completed upgrade %s", UpgradeName))
		return newVM, nil
	}
}
