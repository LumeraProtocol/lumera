package cmd

import (
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
