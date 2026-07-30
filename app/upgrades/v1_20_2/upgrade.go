package v1_20_2

// UpgradeName is the on-chain name used for this upgrade.
//
// This constant — not the git tag — is what the governance proposal `--name`,
// the cosmovisor `upgrades/<NAME>/` directory, and `q upgrade applied <NAME>`
// all key off. Keep them identical.
const UpgradeName = "v1.20.2"

// v1.20.2 is a migration-only upgrade whose sole purpose is to carry module
// consensus-version bumps that shipped after v1.20.1 had already executed.
//
// WHY THIS UPGRADE EXISTS
//
// The EVM-migration continuity work raises x/audit from ConsensusVersion 2 to 3
// and registers the 2->3 migration. RunMigrations only executes from inside an
// upgrade handler, so a version bump needs an upgrade that has NOT yet run on
// the target network in order to land.
//
// Verified live on 2026-07-30:
//
//	lumera-testnet-2   app_version 1.20.1   audit module version 2
//	lumera-mainnet-1   app_version 1.12.0   audit module version 2
//
// Testnet had already executed BOTH v1.20.0 and v1.20.1; neither can run again.
// Without this upgrade the binary would declare audit 3 while committed testnet
// state said 2, with no path between them. Mainnet, still at 1.12.0, would have
// picked the bump up for free via the v1.20.0 EVM bring-up — so the defect was
// invisible from the mainnet path and would have shipped a broken testnet
// release.
//
// DELIBERATELY NOT A NEW HANDLER FUNCTION
//
// This package intentionally exposes only UpgradeName. The upgrade is wired in
// app/upgrades/upgrades.go using the shared standardUpgradeHandler, which does
// exactly one thing: RunMigrations. Adding bespoke logic here would be a
// mistake — every behavioral change belongs in the module migration itself, so
// that a chain reaching this version by any path gets identical state.
//
// NO STORE CHANGES
//
// This upgrade declares no StoreUpgrades. Testnet already mounted the EVM store
// keys during v1.20.0, and re-adding an already-mounted key is a mount error.
// Any future store addition must go in its own upgrade, not here.
