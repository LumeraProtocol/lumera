package cmd

import (
	"strings"
	"testing"

	"github.com/cosmos/cosmos-sdk/client/flags"
	evmhd "github.com/cosmos/evm/crypto/hd"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// TestNewRootCmdStartWiresEVMFlags verifies `start` command includes Cosmos EVM
// server flags required by JSON-RPC and indexer startup path.
func TestNewRootCmdStartWiresEVMFlags(t *testing.T) {
	t.Parallel()

	rootCmd := NewRootCmd()
	startCmd := mustFindSubcommand(t, rootCmd, "start")

	require.NotNil(t, startCmd.Flags().Lookup("json-rpc.enable"))
	require.NotNil(t, startCmd.Flags().Lookup("json-rpc.enable-indexer"))
	require.NotNil(t, startCmd.Flags().Lookup("json-rpc.address"))
	require.NotNil(t, startCmd.Flags().Lookup("json-rpc.ws-address"))
}

// TestNewRootCmdDefaultKeyTypeOverridden verifies recursive default overrides
// set EthSecp256k1 key type across key-management and testnet commands.
func TestNewRootCmdDefaultKeyTypeOverridden(t *testing.T) {
	t.Parallel()

	rootCmd := NewRootCmd()
	expectedAlgo := string(evmhd.EthSecp256k1Type)

	keysAddCmd := mustFindSubcommand(t, mustFindSubcommand(t, rootCmd, "keys"), "add")
	keyTypeFlag := keysAddCmd.Flags().Lookup(flags.FlagKeyType)
	require.NotNil(t, keyTypeFlag)
	require.Equal(t, expectedAlgo, keyTypeFlag.DefValue)

	testnetStartCmd := mustFindSubcommand(t, mustFindSubcommand(t, rootCmd, "testnet"), "start")
	testnetKeyTypeFlag := testnetStartCmd.Flags().Lookup(flags.FlagKeyType)
	require.NotNil(t, testnetKeyTypeFlag)
	require.Equal(t, expectedAlgo, testnetKeyTypeFlag.DefValue)
}

func mustFindSubcommand(t *testing.T, cmd *cobra.Command, useToken string) *cobra.Command {
	t.Helper()

	for _, sub := range cmd.Commands() {
		token := strings.Fields(sub.Use)
		if len(token) > 0 && token[0] == useToken {
			return sub
		}
	}

	t.Fatalf("subcommand %q not found under %q", useToken, cmd.Use)
	return nil
}
