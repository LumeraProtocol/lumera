package upgrades

import (
	"testing"

	"cosmossdk.io/log"
	"github.com/stretchr/testify/require"

	upgrade_v1_10_1 "github.com/LumeraProtocol/lumera/app/upgrades/v1_10_1"
	upgrade_v1_11_1 "github.com/LumeraProtocol/lumera/app/upgrades/v1_11_1"
	upgrade_v1_20_1 "github.com/LumeraProtocol/lumera/app/upgrades/v1_20_1"
)

func TestStoreLoaderForUpgrade_AdaptiveConsensusRename(t *testing.T) {
	selection := StoreLoaderForUpgrade(
		upgrade_v1_10_1.UpgradeName,
		100,
		nil,
		map[string]struct{}{},
		log.NewNopLogger(),
		true,
	)

	require.NotNil(t, selection.Loader)
	require.Equal(t, "Configured store loader for upgrade (consensus rename)", selection.LogMessage())
}

func TestStoreLoaderForUpgrade_AdaptiveAuditStore(t *testing.T) {
	selection := StoreLoaderForUpgrade(
		upgrade_v1_11_1.UpgradeName,
		100,
		nil,
		map[string]struct{}{},
		log.NewNopLogger(),
		true,
	)

	require.NotNil(t, selection.Loader)
	require.Equal(t, "Configured store loader for upgrade (conditional audit store)", selection.LogMessage())
}

func TestStoreLoaderForUpgrade_AdaptiveDefault(t *testing.T) {
	selection := StoreLoaderForUpgrade(
		"v9.9.9",
		100,
		nil,
		map[string]struct{}{},
		log.NewNopLogger(),
		true,
	)

	require.NotNil(t, selection.Loader)
	require.Equal(t, "Configured store loader for upgrade (adaptive mode)", selection.LogMessage())
}

func TestStoreLoaderForUpgrade_NonAdaptiveConsensusRename(t *testing.T) {
	selection := StoreLoaderForUpgrade(
		upgrade_v1_10_1.UpgradeName,
		100,
		nil,
		nil,
		log.NewNopLogger(),
		false,
	)

	require.NotNil(t, selection.Loader)
	require.Equal(t, "Configured store loader for upgrade (consensus rename)", selection.LogMessage())
}

func TestStoreLoaderForUpgrade_NonAdaptiveAuditStore(t *testing.T) {
	selection := StoreLoaderForUpgrade(
		upgrade_v1_11_1.UpgradeName,
		100,
		nil,
		nil,
		log.NewNopLogger(),
		false,
	)

	require.NotNil(t, selection.Loader)
	require.Equal(t, "Configured store loader for upgrade (conditional audit store)", selection.LogMessage())
}

// v1.20.1 must select the add-only loader regardless of whether the adaptive
// store manager is enabled, so the state-driven EVM bring-up works flag-free on
// every network.
func TestStoreLoaderForUpgrade_V1201AddOnlyRegardlessOfAdaptive(t *testing.T) {
	for _, adaptive := range []bool{true, false} {
		selection := StoreLoaderForUpgrade(
			upgrade_v1_20_1.UpgradeName,
			100,
			nil,
			map[string]struct{}{},
			log.NewNopLogger(),
			adaptive,
		)

		require.NotNil(t, selection.Loader)
		require.Equal(t, "Configured store loader for upgrade (add-only EVM bring-up)", selection.LogMessage(),
			"v1.20.1 should use the add-only loader with adaptive=%v", adaptive)
	}
}

func TestStoreLoaderForUpgrade_NonAdaptiveDefault(t *testing.T) {
	selection := StoreLoaderForUpgrade(
		"v9.9.9",
		100,
		nil,
		nil,
		log.NewNopLogger(),
		false,
	)

	require.NotNil(t, selection.Loader)
	require.Equal(t, "Configured store loader for upgrade", selection.LogMessage())
}
