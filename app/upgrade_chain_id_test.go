package app

import (
	"os"
	"path/filepath"
	"testing"

	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/stretchr/testify/require"
)

type upgradeRoutingAppOptions map[string]interface{}

func (o upgradeRoutingAppOptions) Get(key string) interface{} {
	return o[key]
}

func TestUpgradeRoutingChainIDFallsBackToGenesisForPlaceholder(t *testing.T) {
	home := writeGenesisWithChainID(t, "lumera-mainnet-1")
	opts := upgradeRoutingAppOptions{
		flags.FlagHome: home,
	}

	got := upgradeRoutingChainID(Name, opts, log.NewNopLogger())

	require.Equal(t, "lumera-mainnet-1", got)
}

func TestUpgradeRoutingChainIDKeepsExplicitNetworkChainID(t *testing.T) {
	home := writeGenesisWithChainID(t, "lumera-mainnet-1")
	opts := upgradeRoutingAppOptions{
		flags.FlagHome: home,
	}

	got := upgradeRoutingChainID("lumera-testnet-2", opts, log.NewNopLogger())

	require.Equal(t, "lumera-testnet-2", got)
}

func TestUpgradeRoutingChainIDKeepsPlaceholderWhenGenesisUnavailable(t *testing.T) {
	opts := upgradeRoutingAppOptions{
		flags.FlagHome: t.TempDir(),
	}

	got := upgradeRoutingChainID(Name, opts, log.NewNopLogger())

	require.Equal(t, Name, got)
}

func writeGenesisWithChainID(t *testing.T, chainID string) string {
	t.Helper()

	home := t.TempDir()
	configDir := filepath.Join(home, "config")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	genesis := []byte(`{"chain_id":"` + chainID + `"}`)
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "genesis.json"), genesis, 0o644))
	return home
}
