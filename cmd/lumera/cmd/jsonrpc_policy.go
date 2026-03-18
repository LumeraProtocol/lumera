package cmd

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/x/genutil/types"
	srvflags "github.com/cosmos/evm/server/flags"
	"github.com/spf13/cobra"

	"github.com/LumeraProtocol/lumera/app"
	"github.com/LumeraProtocol/lumera/app/upgrades"
)

var mainnetDisallowedJSONRPCNamespaces = []string{"admin", "debug", "personal"}

func validateStartJSONRPCNamespacePolicy(cmd *cobra.Command) error {
	if !isRootStartCommand(cmd) {
		return nil
	}

	serverCtx := server.GetServerContextFromCmd(cmd)
	if !serverCtx.Viper.GetBool(srvflags.JSONRPCEnable) {
		return nil
	}

	chainID, err := currentChainID(serverCtx)
	if err != nil {
		return err
	}

	return validateJSONRPCNamespacePolicy(chainID, serverCtx.Viper.GetStringSlice(srvflags.JSONRPCAPI))
}

func validateJSONRPCNamespacePolicy(chainID string, namespaces []string) error {
	if !upgrades.IsMainnet(chainID) {
		return nil
	}

	var forbidden []string
	for _, namespace := range namespaces {
		namespace = strings.TrimSpace(strings.ToLower(namespace))
		if slices.Contains(mainnetDisallowedJSONRPCNamespaces, namespace) && !slices.Contains(forbidden, namespace) {
			forbidden = append(forbidden, namespace)
		}
	}

	if len(forbidden) == 0 {
		return nil
	}

	return fmt.Errorf(
		"json-rpc namespaces %q are disabled on mainnet chain %q; remove them from json-rpc.api",
		forbidden,
		chainID,
	)
}

func currentChainID(serverCtx *server.Context) (string, error) {
	if chainID := strings.TrimSpace(serverCtx.Viper.GetString(flags.FlagChainID)); chainID != "" {
		return chainID, nil
	}

	genesisFile := serverCtx.Config.GenesisFile()
	reader, err := os.Open(genesisFile)
	if err != nil {
		return "", fmt.Errorf("open genesis file %q: %w", genesisFile, err)
	}
	defer reader.Close()

	chainID, err := types.ParseChainIDFromGenesis(reader)
	if err != nil {
		return "", fmt.Errorf("parse chain-id from genesis file %q: %w", genesisFile, err)
	}

	return chainID, nil
}

func isRootStartCommand(cmd *cobra.Command) bool {
	return cmd.Name() == "start" && cmd.Parent() != nil && cmd.Parent().Name() == app.Name+"d"
}
