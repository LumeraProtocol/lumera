package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cosmos/cosmos-sdk/server"
	serverconfig "github.com/cosmos/cosmos-sdk/server/config"
	cosmosevmserverconfig "github.com/cosmos/evm/server/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	appopenrpc "github.com/LumeraProtocol/lumera/app/openrpc"
	lcfg "github.com/LumeraProtocol/lumera/config"
)

// migrateAppConfigIfNeeded checks whether the running app.toml is missing
// any EVM configuration sections added in the v1.12.0 upgrade and, if so,
// regenerates the file with Lumera defaults while preserving every existing
// operator setting. It also reloads the corrected values into the in-memory
// Viper instance so the current process uses them immediately (no restart
// needed).
//
// Background: the Cosmos SDK only writes app.toml when the file does not
// exist (server.InterceptConfigsPreRunHandler, util.go:284). Nodes that
// upgraded from a pre-EVM binary keep their old app.toml, which lacks
// [evm], [evm.mempool], [json-rpc], [tls], and [lumera.*] sections. The
// JSON-RPC backend reads evm-chain-id from app.toml.
func migrateAppConfigIfNeeded(cmd *cobra.Command) error {
	serverCtx := server.GetServerContextFromCmd(cmd)
	v := serverCtx.Viper

	if !needsConfigMigration(v) {
		return nil
	}

	rootDir := v.GetString("home")
	if rootDir == "" {
		rootDir = serverCtx.Config.RootDir
	}
	appCfgPath := filepath.Join(rootDir, "config", "app.toml")

	if _, err := os.Stat(appCfgPath); os.IsNotExist(err) {
		return nil
	}

	return doMigrateAppConfig(v, appCfgPath)
}

// doMigrateAppConfig is the core migration logic, separated from the cobra
// command plumbing so it can be tested directly with a real Viper instance
// and a temp app.toml file.
func doMigrateAppConfig(v *viper.Viper, appCfgPath string) error {
	// Build the canonical Lumera app config with correct defaults.
	_, defaultCfg := initAppConfig()
	fullCfg, ok := defaultCfg.(CustomAppConfig)
	if !ok {
		fullCfgPtr, ok2 := defaultCfg.(*CustomAppConfig)
		if !ok2 {
			return fmt.Errorf("unexpected initAppConfig return type: %T", defaultCfg)
		}
		fullCfg = *fullCfgPtr
	}

	// Unmarshal the existing Viper state into the full config struct.
	// This preserves every setting the operator already had (API, gRPC,
	// telemetry, etc.) while filling in EVM defaults for missing keys.
	if err := v.Unmarshal(&fullCfg); err != nil {
		return fmt.Errorf("failed to unmarshal existing app config: %w", err)
	}

	// Force the EVM chain ID to the Lumera constant — an operator should
	// never have a different value.
	fullCfg.EVM.EVMChainID = lcfg.EVMChainID

	// Only enable JSON-RPC and indexer when the section was never written
	// (i.e. the key is not present in Viper at all). If an operator
	// explicitly set json-rpc.enable = false, we respect that choice.
	if !v.IsSet("json-rpc.enable") {
		fullCfg.JSONRPC.Enable = true
	}
	if !v.IsSet("json-rpc.enable-indexer") {
		fullCfg.JSONRPC.EnableIndexer = true
	}
	// Ensure the "rpc" namespace is present (required for rpc_discover / OpenRPC).
	fullCfg.JSONRPC.API = appopenrpc.EnsureNamespaceEnabled(fullCfg.JSONRPC.API)
	// If the API list is empty (no [json-rpc] section at all), use the Lumera defaults.
	if len(fullCfg.JSONRPC.API) == 0 {
		fullCfg.JSONRPC.API = appopenrpc.EnsureNamespaceEnabled(
			cosmosevmserverconfig.GetDefaultAPINamespaces(),
		)
	}

	// Write the regenerated config with the full template to disk.
	customAppTemplate := serverconfig.DefaultConfigTemplate +
		cosmosevmserverconfig.DefaultEVMConfigTemplate +
		lumeraConfigTemplate
	serverconfig.SetConfigTemplate(customAppTemplate)
	serverconfig.WriteConfigFile(appCfgPath, fullCfg)

	// Reload the corrected config into the in-memory Viper so the current
	// process uses the migrated values immediately (not just on next restart).
	//
	// MergeInConfig does NOT override keys already present in Viper, so we
	// read the new file into a fresh Viper and then force-set every key that
	// was added or corrected by the migration.
	freshV := viper.New()
	freshV.SetConfigType("toml")
	freshV.SetConfigFile(appCfgPath)
	if err := freshV.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to reload migrated app.toml: %w", err)
	}

	// Force-set all EVM-related keys from the freshly written file into the
	// live Viper instance. This covers evm-chain-id, json-rpc.enable,
	// json-rpc.enable-indexer, and every other key the migration may have
	// added or corrected.
	for _, key := range freshV.AllKeys() {
		if !v.IsSet(key) || isEVMMigratedKey(key) {
			v.Set(key, freshV.Get(key))
		}
	}

	fmt.Fprintf(os.Stderr, "INFO: migrated app.toml — added EVM configuration sections (evm-chain-id=%d)\n", lcfg.EVMChainID)
	return nil
}

// isEVMMigratedKey returns true for keys that belong to sections added or
// corrected by the v1.12.0 config migration. These keys are always force-set
// into the live Viper after migration, overriding any stale in-memory values.
func isEVMMigratedKey(key string) bool {
	for _, prefix := range evmMigratedPrefixes {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

var evmMigratedPrefixes = []string{
	"evm.",
	"json-rpc.",
	"tls.",
	"lumera.",
}

// needsConfigMigration returns true if any v1.12.0 config section is missing
// or has an incorrect sentinel value. Checks multiple keys so that partial
// manual edits (e.g. operator set evm-chain-id but not [lumera.*]) are still
// caught.
//
// Important: this function must NOT trigger on intentional operator choices
// like json-rpc.enable = false. It only checks structural presence of
// sections via IsSet and mandatory-value correctness (chain ID).
func needsConfigMigration(v viperGetter) bool {
	// Wrong or missing EVM chain ID (0 = absent, 262144 = upstream default).
	chainID := v.GetUint64("evm.evm-chain-id")
	if chainID != lcfg.EVMChainID {
		return true
	}

	// [json-rpc] section absent — key was never written to app.toml.
	// We use IsSet to distinguish "never written" from "explicitly disabled."
	if !v.IsSet("json-rpc.enable") {
		return true
	}

	// [lumera.json-rpc-ratelimit] section absent (sentinel: proxy-address
	// will be empty string when the section was never written).
	if v.GetString("lumera.json-rpc-ratelimit.proxy-address") == "" {
		return true
	}

	// [tls] section absent — the key itself being unset means the section
	// was never written.
	if !v.IsSet("tls.certificate-path") {
		return true
	}

	return false
}

// viperGetter is the subset of *viper.Viper used by needsConfigMigration,
// extracted for testability.
type viperGetter interface {
	GetUint64(key string) uint64
	GetBool(key string) bool
	GetString(key string) string
	IsSet(key string) bool
}
