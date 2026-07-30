package upgrades

import (
	"testing"

	"cosmossdk.io/log"
	"github.com/stretchr/testify/require"

	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	upgrade_v1_20_2 "github.com/LumeraProtocol/lumera/app/upgrades/v1_20_2"
	evmigrationtypes "github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// TestV1202MountsEVMStoresForMainnetOneHop is the regression guard for the
// Phase 2 devnet failure of 2026-07-30.
//
// # THE DEFECT IT PREVENTS
//
// v1.20.2 originally declared NO StoreUpgrades. That is correct reasoning for
// testnet, where v1.20.0 already mounted the EVM store keys and re-adding a
// mounted key is an error. But mainnet is still on 1.12.0 and has NONE of
// those stores. A mainnet operator upgrading straight to v1.20.2 therefore
// mounted nothing, and every validator crash-looped on startup with:
//
//	panic: failed to load latest version: version of store evmigration
//	mismatch root store's version; expected 155 got 0; new stores should be
//	added using StoreUpgrades
//
// Reproduced on a 5-validator devnet built from the real v1.12.0 release
// artifact, with genesis trimmed to the v1.12.0 module set so the chain was a
// faithful mainnet replica (audit v2, all five EVM modules absent).
//
// # THE FIX
//
// Declare the same EVM store additions v1.20.1 declares and use the same
// add-only store loader. AddOnlyStoreLoader mounts only the keys missing from
// committed state and never deletes one, so it is a no-op on a chain that
// already ran v1.20.0 (testnet) and performs the full bring-up on a chain that
// did not (mainnet). One binary, both shapes, no chain-id branching.
func TestV1202MountsEVMStoresForMainnetOneHop(t *testing.T) {
	required := []string{
		feemarkettypes.StoreKey,
		precisebanktypes.StoreKey,
		evmtypes.StoreKey,
		erc20types.StoreKey,
		evmigrationtypes.StoreKey,
	}

	for _, chainID := range []string{"lumera-mainnet-1", "lumera-testnet-2", "lumera-devnet-1"} {
		t.Run(chainID, func(t *testing.T) {
			cfg, found := SetupUpgrades(upgrade_v1_20_2.UpgradeName, newTestUpgradeParams(chainID))
			require.True(t, found, "v1.20.2 must be registered on %s", chainID)

			require.NotNil(t, cfg.StoreUpgrade,
				"v1.20.2 MUST declare EVM store additions: a chain arriving from "+
					"1.12.0 has none of them, and without StoreUpgrades every "+
					"validator panics with 'store evmigration mismatch ... expected N got 0'")

			for _, key := range required {
				require.Containsf(t, cfg.StoreUpgrade.Added, key,
					"v1.20.2 must add store key %q on %s so a direct 1.12.0 -> 1.20.2 "+
						"upgrade mounts it", key, chainID)
			}

			require.Empty(t, cfg.StoreUpgrade.Deleted,
				"v1.20.2 must never delete a store; deletion is unsafe on a chain "+
					"that already ran v1.20.0")
			require.Empty(t, cfg.StoreUpgrade.Renamed,
				"v1.20.2 must never rename a store")
		})
	}
}

// TestV1202UsesAddOnlyStoreLoader asserts the loader selection, which is what
// makes declaring the store keys safe on a chain that already has them.
//
// Without the add-only loader, declaring an already-mounted key is a mount
// error on testnet — so the store declaration above and this loader are a
// matched pair. Changing one without the other breaks exactly one network,
// which is the failure mode this whole rehearsal series exists to catch.
func TestV1202UsesAddOnlyStoreLoader(t *testing.T) {
	sel := StoreLoaderForUpgrade(
		upgrade_v1_20_2.UpgradeName,
		100,
		&upgrade_v1_20_2.StoreUpgrades,
		map[string]struct{}{},
		log.NewNopLogger(),
		false, // adaptive off: the add-only path must not depend on the env flag
	)
	require.NotNil(t, sel.Loader,
		"v1.20.2 must select a store loader even with adaptive mode off")
	require.Equal(t, "add-only EVM bring-up", sel.LogLabel,
		"v1.20.2 must use the add-only loader so mounting is safe whether or not "+
			"the EVM stores already exist")
}
