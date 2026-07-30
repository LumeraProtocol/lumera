package upgrades

import (
	"testing"

	"github.com/stretchr/testify/require"

	upgrade_v1_20_1 "github.com/LumeraProtocol/lumera/app/upgrades/v1_20_1"
	upgrade_v1_20_2 "github.com/LumeraProtocol/lumera/app/upgrades/v1_20_2"
	audittypes "github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// TestAuditConsensusVersionHasCarryingUpgrade is the regression guard for the
// release blocker found on 2026-07-30 during upgrade-path verification.
//
// # THE DEFECT IT PREVENTS
//
// x/audit was raised from ConsensusVersion 2 to 3 and the 2->3 migration was
// correctly registered, but no NEW upgrade handler was added to carry it.
// RunMigrations only executes from inside an upgrade handler, and on
// lumera-testnet-2 both v1.20.0 and v1.20.1 had ALREADY executed (verified live:
// app_version 1.20.1, audit module version 2). Neither would ever run again, so
// shipping that binary would leave the module declaring 3 while committed state
// said 2, with no path between them.
//
// Mainnet masked the bug: at 1.12.0 it had not yet run v1.20.0, so the EVM
// bring-up would have carried audit 2->3 for free. A mainnet-only rehearsal
// would have passed and shipped a broken testnet release. That asymmetry is
// exactly why this test asserts the invariant structurally instead of relying
// on any single network's rehearsal.
//
// # THE INVARIANT
//
// Whenever a module's ConsensusVersion is raised, the LAST upgrade in
// upgradeNames must be one that has not yet executed on every target network,
// so that RunMigrations is guaranteed to fire. Concretely: the newest upgrade
// must sort strictly after the newest version already live on any network.
func TestAuditConsensusVersionHasCarryingUpgrade(t *testing.T) {
	// Pin the audit consensus version this release ships. If someone bumps it
	// again without adding a carrying upgrade, this fails and points here.
	require.Equal(t, 3, audittypes.ConsensusVersion,
		"audit ConsensusVersion changed; add a NEW upgrade handler to carry the "+
			"migration and update this test - see the comment above")

	require.NotEmpty(t, upgradeNames)
	newest := upgradeNames[len(upgradeNames)-1]

	// The newest registered upgrade must be strictly newer than v1.20.1, which
	// is already applied on lumera-testnet-2 (verified live 2026-07-30).
	require.NotEqual(t, upgrade_v1_20_1.UpgradeName, newest,
		"newest upgrade is v1.20.1, which has ALREADY executed on testnet; a "+
			"ConsensusVersion bump shipped behind it can never run RunMigrations")

	require.Equal(t, upgrade_v1_20_2.UpgradeName, newest,
		"expected v1.20.2 to be the newest registered upgrade carrying the audit 2->3 migration")
}

// TestV1202IsRegisteredAndMigrationOnly asserts the carrying upgrade is wired
// into the config and, critically, declares NO store changes.
//
// A store upgrade here would be actively dangerous: on testnet the EVM stores
// already exist (v1.20.0 ran), and re-adding a mounted key is a mount error.
// The whole point of this handler is to be the thinnest possible vehicle for
// RunMigrations.
func TestV1202IsRegisteredAndMigrationOnly(t *testing.T) {
	for _, chainID := range []string{
		"lumera-mainnet-1",
		"lumera-testnet-2",
		"lumera-devnet-1",
	} {
		t.Run(chainID, func(t *testing.T) {
			params := newTestUpgradeParams(chainID)
			cfg, found := SetupUpgrades(upgrade_v1_20_2.UpgradeName, params)
			require.True(t, found,
				"v1.20.2 must be registered on %s - an unregistered upgrade name "+
					"halts the chain at the plan height with no handler", chainID)
			require.NotNil(t, cfg.Handler, "v1.20.2 must have a handler")
			require.Nil(t, cfg.StoreUpgrade,
				"v1.20.2 must declare NO store changes: on testnet the EVM stores "+
					"already exist, and re-mounting an existing key is an error")
		})
	}
}

// TestV1202UpgradeNameMatchesDirectory guards the UpgradeName != git tag trap.
// The gov proposal --name, the cosmovisor upgrades/<NAME>/ directory and
// `q upgrade applied <NAME>` all key off this constant. A mismatch means
// cosmovisor never auto-swaps and the query returns "not found" forever.
func TestV1202UpgradeNameMatchesDirectory(t *testing.T) {
	require.Equal(t, "v1.20.2", upgrade_v1_20_2.UpgradeName)
}
