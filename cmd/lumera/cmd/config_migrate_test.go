package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	lcfg "github.com/LumeraProtocol/lumera/config"
)

// TestNeedsConfigMigration_LegacyConfig verifies that a pre-EVM app.toml
// (no [evm], [json-rpc], [tls], or [lumera.*] sections) triggers migration.
func TestNeedsConfigMigration_LegacyConfig(t *testing.T) {
	t.Parallel()

	v := viper.New()
	// Simulate a legacy config with no EVM sections at all — Viper returns
	// zero values for all keys.
	assert.True(t, needsConfigMigration(v), "empty viper (pre-EVM config) must trigger migration")
}

// TestNeedsConfigMigration_UpstreamDefault verifies that the cosmos/evm
// upstream default chain ID (262144) triggers migration.
func TestNeedsConfigMigration_UpstreamDefault(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set("evm.evm-chain-id", uint64(262144)) // upstream default, not Lumera
	v.Set("json-rpc.enable", true)
	v.Set("lumera.json-rpc-ratelimit.proxy-address", "0.0.0.0:8547")
	v.Set("tls.certificate-path", "")

	assert.True(t, needsConfigMigration(v), "upstream default chain ID must trigger migration")
}

// TestNeedsConfigMigration_PartialManualEdit verifies that an operator who
// manually set evm-chain-id but is still missing [json-rpc] triggers migration.
func TestNeedsConfigMigration_PartialManualEdit(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set("evm.evm-chain-id", lcfg.EVMChainID) // correct
	// json-rpc.enable is false (absent) — must still trigger migration.
	v.Set("lumera.json-rpc-ratelimit.proxy-address", "0.0.0.0:8547")
	v.Set("tls.certificate-path", "")

	assert.True(t, needsConfigMigration(v), "correct chain ID but missing json-rpc must trigger migration")
}

// TestNeedsConfigMigration_MissingLumeraSection verifies that a config with
// correct [evm] and [json-rpc] but missing [lumera.*] triggers migration.
func TestNeedsConfigMigration_MissingLumeraSection(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set("evm.evm-chain-id", lcfg.EVMChainID)
	v.Set("json-rpc.enable", true)
	// lumera.json-rpc-ratelimit.proxy-address is empty — must trigger.
	v.Set("tls.certificate-path", "")

	assert.True(t, needsConfigMigration(v), "missing lumera section must trigger migration")
}

// TestNeedsConfigMigration_OperatorDisabledJSONRPC verifies that an operator
// who explicitly set json-rpc.enable = false does NOT trigger migration
// (their choice is respected, not treated as a missing section).
func TestNeedsConfigMigration_OperatorDisabledJSONRPC(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set("evm.evm-chain-id", lcfg.EVMChainID)
	v.Set("json-rpc.enable", false) // explicitly set by operator
	v.Set("lumera.json-rpc-ratelimit.proxy-address", "0.0.0.0:8547")
	v.Set("tls.certificate-path", "")

	assert.False(t, needsConfigMigration(v), "operator-disabled json-rpc must NOT trigger migration")
}

// TestNeedsConfigMigration_FullyMigrated verifies that a correctly migrated
// config does NOT trigger migration.
func TestNeedsConfigMigration_FullyMigrated(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set("evm.evm-chain-id", lcfg.EVMChainID)
	v.Set("json-rpc.enable", true)
	v.Set("lumera.json-rpc-ratelimit.proxy-address", "0.0.0.0:8547")
	v.Set("tls.certificate-path", "") // IsSet returns true when explicitly set

	assert.False(t, needsConfigMigration(v), "fully migrated config must not trigger migration")
}

func TestNeedsConfigMigration_DisabledMempool(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set("evm.evm-chain-id", lcfg.EVMChainID)
	v.Set("json-rpc.enable", true)
	v.Set("lumera.json-rpc-ratelimit.proxy-address", "0.0.0.0:8547")
	v.Set("tls.certificate-path", "")
	v.Set("mempool.max-txs", -1)

	assert.True(t, needsConfigMigration(v), "disabled app mempool must trigger migration repair")
}

// TestMigrateAppConfig_LegacyTomlOnDisk verifies the full migration flow:
// start with a legacy pre-EVM app.toml, run the migrator, and confirm both
// the disk file and in-memory Viper contain the correct EVM config.
func TestMigrateAppConfig_LegacyTomlOnDisk(t *testing.T) {
	t.Parallel()

	// Create a temp directory with a minimal legacy app.toml (no EVM sections).
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	require.NoError(t, os.MkdirAll(configDir, 0o755))

	legacyToml := `
[api]
enable = true
address = "tcp://0.0.0.0:1317"

[grpc]
enable = true
address = "0.0.0.0:9090"

[mempool]
max-txs = 3000
`
	appCfgPath := filepath.Join(configDir, "app.toml")
	require.NoError(t, os.WriteFile(appCfgPath, []byte(legacyToml), 0o644))

	// Set up Viper pointing to the legacy config.
	v := viper.New()
	v.SetConfigType("toml")
	v.SetConfigName("app")
	v.AddConfigPath(configDir)
	require.NoError(t, v.MergeInConfig())

	// Preconditions: EVM keys are absent/default.
	require.NotEqual(t, lcfg.EVMChainID, v.GetUint64("evm.evm-chain-id"),
		"precondition: evm-chain-id should not be set in legacy config")
	require.True(t, needsConfigMigration(v), "precondition: legacy config must need migration")

	// Run the real migration entrypoint.
	require.NoError(t, doMigrateAppConfig(v, appCfgPath))

	// ── Verify disk state by reading the file with a fresh Viper ──────
	v2 := viper.New()
	v2.SetConfigType("toml")
	v2.SetConfigName("app")
	v2.AddConfigPath(configDir)
	require.NoError(t, v2.MergeInConfig())

	assert.Equal(t, lcfg.EVMChainID, v2.GetUint64("evm.evm-chain-id"),
		"disk: evm-chain-id must match Lumera constant")
	assert.True(t, v2.GetBool("json-rpc.enable"),
		"disk: json-rpc must be enabled")
	assert.True(t, v2.GetBool("json-rpc.enable-indexer"),
		"disk: json-rpc indexer must be enabled")
	assert.NotEmpty(t, v2.GetString("lumera.json-rpc-ratelimit.proxy-address"),
		"disk: lumera rate limit proxy-address must be set")
	assert.True(t, v2.IsSet("tls.certificate-path"),
		"disk: tls section must be present")

	// ── Verify in-memory Viper was updated by doMigrateAppConfig ──────
	// The real freshV.ReadInConfig + AllKeys copy logic must have force-set
	// these keys into the original Viper instance.
	assert.Equal(t, lcfg.EVMChainID, v.GetUint64("evm.evm-chain-id"),
		"in-memory: evm-chain-id must be updated")
	assert.True(t, v.GetBool("json-rpc.enable"),
		"in-memory: json-rpc must be enabled after reload")
	assert.True(t, v.GetBool("json-rpc.enable-indexer"),
		"in-memory: json-rpc indexer must be enabled after reload")
	assert.NotEmpty(t, v.GetString("lumera.json-rpc-ratelimit.proxy-address"),
		"in-memory: lumera rate limit proxy-address must be set")

	// ── Operator's existing settings must be preserved ────────────────
	assert.True(t, v.GetBool("api.enable"),
		"operator's api.enable must be preserved in-memory")
	assert.Equal(t, "tcp://0.0.0.0:1317", v.GetString("api.address"),
		"operator's api.address must be preserved in-memory")
	assert.Equal(t, int64(3000), v.GetInt64("mempool.max-txs"),
		"operator's mempool.max-txs must be preserved in-memory")

	// Migration should be a no-op on second call.
	assert.False(t, needsConfigMigration(v),
		"after migration, needsConfigMigration must return false")
}

func TestMigrateAppConfig_FullyMigratedNegativeMaxTxsTriggersRepair(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	require.NoError(t, os.MkdirAll(configDir, 0o755))

	appToml := fmt.Sprintf(`
[mempool]
max-txs = -1

[evm]
evm-chain-id = %d

[json-rpc]
enable = false

[lumera.json-rpc-ratelimit]
proxy-address = "0.0.0.0:8547"

[tls]
certificate-path = ""
`, lcfg.EVMChainID)
	appCfgPath := filepath.Join(configDir, "app.toml")
	require.NoError(t, os.WriteFile(appCfgPath, []byte(appToml), 0o644))

	v := viper.New()
	v.SetConfigType("toml")
	v.SetConfigName("app")
	v.AddConfigPath(configDir)
	v.Set("chain-id", "lumera-mainnet-1")
	require.NoError(t, v.MergeInConfig())
	require.True(t, needsConfigMigration(v), "precondition: disabled mempool must need repair")

	require.NoError(t, doMigrateAppConfig(v, appCfgPath))

	v2 := viper.New()
	v2.SetConfigType("toml")
	v2.SetConfigName("app")
	v2.AddConfigPath(configDir)
	require.NoError(t, v2.MergeInConfig())

	assert.Equal(t, int64(10000), v.GetInt64("mempool.max-txs"),
		"in-memory disabled mempool must be repaired")
	assert.Equal(t, int64(10000), v2.GetInt64("mempool.max-txs"),
		"disk disabled mempool must be repaired")
	assert.False(t, v.GetBool("json-rpc.enable"),
		"explicit operator-disabled json-rpc must remain disabled")
	assert.False(t, needsConfigMigration(v),
		"after repair, needsConfigMigration must return false")
}

func TestMigrateAppConfig_LegacyNegativeMaxTxsUsesNetworkDefault(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		chainID          string
		chainIDInGenesis bool
		wantMaxTxs       int64
	}{
		{name: "devnet from viper", chainID: "lumera-devnet-1", wantMaxTxs: 5000},
		{name: "devnet from genesis", chainID: "lumera-devnet-1", chainIDInGenesis: true, wantMaxTxs: 5000},
		{name: "testnet from viper", chainID: "lumera-testnet-2", wantMaxTxs: 10000},
		{name: "mainnet from viper", chainID: "lumera-mainnet-1", wantMaxTxs: 10000},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			configDir := filepath.Join(tmpDir, "config")
			require.NoError(t, os.MkdirAll(configDir, 0o755))

			legacyToml := `
[api]
enable = true

[mempool]
max-txs = -1
`
			appCfgPath := filepath.Join(configDir, "app.toml")
			require.NoError(t, os.WriteFile(appCfgPath, []byte(legacyToml), 0o644))
			if tc.chainIDInGenesis {
				genesis := `{"chain_id":"` + tc.chainID + `"}`
				require.NoError(t, os.WriteFile(filepath.Join(configDir, "genesis.json"), []byte(genesis), 0o644))
			}

			v := viper.New()
			v.SetConfigType("toml")
			v.SetConfigName("app")
			v.AddConfigPath(configDir)
			if !tc.chainIDInGenesis {
				v.Set("chain-id", tc.chainID)
			}
			require.NoError(t, v.MergeInConfig())
			require.True(t, needsConfigMigration(v), "precondition: legacy config must need migration")

			require.NoError(t, doMigrateAppConfig(v, appCfgPath))

			v2 := viper.New()
			v2.SetConfigType("toml")
			v2.SetConfigName("app")
			v2.AddConfigPath(configDir)
			require.NoError(t, v2.MergeInConfig())

			assert.Equal(t, tc.wantMaxTxs, v.GetInt64("mempool.max-txs"),
				"in-memory legacy no-op mempool must be replaced for %s", tc.chainID)
			assert.Equal(t, tc.wantMaxTxs, v2.GetInt64("mempool.max-txs"),
				"disk legacy no-op mempool must be replaced for %s", tc.chainID)

			migratedToml, err := os.ReadFile(appCfgPath)
			require.NoError(t, err)
			migratedTomlStr := string(migratedToml)
			assert.Contains(t, migratedTomlStr, "[evm.mempool]")
			assert.Contains(t, migratedTomlStr, "global-slots = 5120")
			assert.Contains(t, migratedTomlStr, "global-queue = 1024")
			assert.NotContains(t, migratedTomlStr, "insert-queue-size")
		})
	}
}
