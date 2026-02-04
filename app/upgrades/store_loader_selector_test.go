package upgrades

import (
	"testing"

	"cosmossdk.io/log"
	"github.com/stretchr/testify/require"

	upgrade_v1_10_1 "github.com/LumeraProtocol/lumera/app/upgrades/v1_10_1"
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
