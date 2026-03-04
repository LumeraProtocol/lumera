package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"cosmossdk.io/log"
	confixcmd "cosmossdk.io/tools/confix/cmd"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/debug"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/client/pruning"
	"github.com/cosmos/cosmos-sdk/client/rpc"
	"github.com/cosmos/cosmos-sdk/client/snapshot"
	"github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authcmd "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	genutilcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	evmserver "github.com/cosmos/evm/server"

	"github.com/CosmWasm/wasmd/x/wasm"
	wasmcli "github.com/CosmWasm/wasmd/x/wasm/client/cli"
	wasmKeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	"github.com/LumeraProtocol/lumera/app"
	appopenrpc "github.com/LumeraProtocol/lumera/app/openrpc"
	lcfg "github.com/LumeraProtocol/lumera/config"
	claimtypes "github.com/LumeraProtocol/lumera/x/claim/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
)

func initRootCmd(
	rootCmd *cobra.Command,
	txConfig client.TxConfig,
	basicManager module.BasicManager,
) {
	if err := appopenrpc.RegisterJSONRPCNamespace(); err != nil {
		panic(err)
	}

	rootCmd.AddCommand(
		initCmdWithEVMDefaults(basicManager),
		NewTestnetCmd(basicManager, banktypes.GenesisBalancesIterator{}),
		debugCommand(),
		confixcmd.ConfigCommand(),
		pruning.Cmd(newApp, app.DefaultNodeHome),
		snapshot.Cmd(newApp),
	)
	// Register --claims-path persistent flag
	rootCmd.PersistentFlags().String(claimtypes.FlagClaimsPath, "",
		fmt.Sprintf("Path to %s file or directory containing it", claimtypes.DefaultClaimsFileName))
	// Bind to viper
	_ = viper.BindPFlag(claimtypes.FlagClaimsPath, rootCmd.PersistentFlags().Lookup(claimtypes.FlagClaimsPath))

	evmserver.AddCommands(
		rootCmd,
		evmserver.NewDefaultStartOptions(newEVMApp, app.DefaultNodeHome),
		appExport,
		addModuleInitFlags,
	)

	// add keybase, auxiliary RPC, query, genesis, and tx child commands
	rootCmd.AddCommand(
		server.StatusCommand(),
		genesisCommand(txConfig, basicManager),
		queryCommand(),
		txCommand(),
		keys.Commands(),
	)
	wasmcli.ExtendUnsafeResetAllCmd(rootCmd)
}

func addModuleInitFlags(startCmd *cobra.Command) {
	wasm.AddModuleInitFlags(startCmd)
}

// initCmdWithEVMDefaults wraps the SDK init command and patches genesis defaults:
//   - chain bank metadata for EVM denom resolution
//   - consensus block max gas for EIP-1559 base fee calculations
func initCmdWithEVMDefaults(basicManager module.BasicManager) *cobra.Command {
	initCmd := genutilcli.InitCmd(basicManager, app.DefaultNodeHome)
	originalRunE := initCmd.RunE
	initCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := originalRunE(cmd, args); err != nil {
			return err
		}
		return patchInitGenesisBankMetadata(cmd)
	}
	return initCmd
}

func patchInitGenesisBankMetadata(cmd *cobra.Command) error {
	clientCtx := client.GetClientContextFromCmd(cmd)
	serverCtx := server.GetServerContextFromCmd(cmd)
	serverCtx.Config.SetRoot(clientCtx.HomeDir)
	genFile := serverCtx.Config.GenesisFile()

	appGenesis, err := genutiltypes.AppGenesisFromFile(genFile)
	if err != nil {
		return err
	}

	var appState map[string]json.RawMessage
	if err := json.Unmarshal(appGenesis.AppState, &appState); err != nil {
		return err
	}

	var bankGenesis banktypes.GenesisState
	clientCtx.Codec.MustUnmarshalJSON(appState[banktypes.ModuleName], &bankGenesis)
	bankGenesis.DenomMetadata = lcfg.UpsertChainBankMetadata(bankGenesis.DenomMetadata)
	appState[banktypes.ModuleName] = clientCtx.Codec.MustMarshalJSON(&bankGenesis)

	appStateBz, err := json.MarshalIndent(appState, "", " ")
	if err != nil {
		return err
	}

	appGenesis.AppState = appStateBz

	if appGenesis.Consensus == nil {
		appGenesis.Consensus = &genutiltypes.ConsensusGenesis{}
	}
	if appGenesis.Consensus.Params == nil {
		appGenesis.Consensus.Params = cmttypes.DefaultConsensusParams()
	}
	appGenesis.Consensus.Params.Block.MaxGas = lcfg.ChainDefaultConsensusMaxGas

	return genutil.ExportGenesisFile(appGenesis, genFile)
}

// genesisCommand builds genesis-related `lumerad genesis` command. Users may provide application specific commands as a parameter
func genesisCommand(txConfig client.TxConfig, basicManager module.BasicManager, cmds ...*cobra.Command) *cobra.Command {
	cmd := genutilcli.Commands(txConfig, basicManager, app.DefaultNodeHome)

	for _, subCmd := range cmds {
		cmd.AddCommand(subCmd)
	}
	return cmd
}

func queryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "query",
		Aliases:                    []string{"q"},
		Short:                      "Querying subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		rpc.WaitTxCmd(),
		rpc.ValidatorCommand(),
		server.QueryBlockCmd(),
		authcmd.QueryTxsByEventsCmd(),
		server.QueryBlocksCmd(),
		authcmd.QueryTxCmd(),
		server.QueryBlockResultsCmd(),
	)
	cmd.PersistentFlags().String(flags.FlagChainID, "", "The network chain ID")

	return cmd
}

func txCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "tx",
		Short:                      "Transactions subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		authcmd.GetSignCommand(),
		authcmd.GetSignBatchCommand(),
		authcmd.GetMultiSignCommand(),
		authcmd.GetMultiSignBatchCmd(),
		authcmd.GetValidateSignaturesCommand(),
		flags.LineBreak,
		authcmd.GetBroadcastCommand(),
		authcmd.GetEncodeCommand(),
		authcmd.GetDecodeCommand(),
		authcmd.GetSimulateCmd(),
	)
	cmd.PersistentFlags().String(flags.FlagChainID, "", "The network chain ID")

	return cmd
}

func debugCommand() *cobra.Command {
	debugCmd := debug.Cmd()
	debugCmd.AddCommand(debugResolveTypeURLCmd())

	return debugCmd
}

// newApp creates the application
func newApp(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	appOpts servertypes.AppOptions,
) servertypes.Application {
	baseappOptions := server.DefaultBaseappOptions(appOpts)
	wasmOpts := []wasmKeeper.Option{}

	return app.New(
		logger, db, traceStore, true,
		appOpts,
		wasmOpts,
		baseappOptions...,
	)
}

// newEVMApp creates the application with the cosmos/evm server.Application type.
func newEVMApp(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	appOpts servertypes.AppOptions,
) evmserver.Application {
	baseappOptions := server.DefaultBaseappOptions(appOpts)
	wasmOpts := []wasmKeeper.Option{}

	return app.New(
		logger, db, traceStore, true,
		appOpts,
		wasmOpts,
		baseappOptions...,
	)
}

// appExport creates a new app (optionally at a given height) and exports state.
func appExport(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	height int64,
	forZeroHeight bool,
	jailAllowedAddrs []string,
	appOpts servertypes.AppOptions,
	modulesToExport []string,
) (servertypes.ExportedApp, error) {
	var bApp *app.App

	// this check is necessary as we use the flag in x/upgrade.
	// we can exit more gracefully by checking the flag here.
	homePath, ok := appOpts.Get(flags.FlagHome).(string)
	if !ok || homePath == "" {
		return servertypes.ExportedApp{}, errors.New("application home not set")
	}

	viperAppOpts, ok := appOpts.(*viper.Viper)
	if !ok {
		return servertypes.ExportedApp{}, errors.New("appOpts is not viper.Viper")
	}

	appOpts = viperAppOpts
	wasmOpts := []wasmKeeper.Option{}
	if height != -1 {
		bApp = app.New(logger, db, traceStore, false, appOpts, wasmOpts)
		if err := bApp.LoadHeight(height); err != nil {
			return servertypes.ExportedApp{}, err
		}
	} else {
		bApp = app.New(logger, db, traceStore, true, appOpts, wasmOpts)
	}

	return bApp.ExportAppStateAndValidators(forZeroHeight, jailAllowedAddrs, modulesToExport)
}
