package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"syscall"

	tmcmd "github.com/cometbft/cometbft/cmd/cometbft/commands"
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
	"github.com/cosmos/cosmos-sdk/version"
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

	addEVMServerCommands(
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

func addEVMServerCommands(
	rootCmd *cobra.Command,
	opts evmserver.StartOptions,
	appExport servertypes.AppExporter,
	addStartFlags servertypes.ModuleInitFlags,
) {
	cometbftCmd := &cobra.Command{
		Use:     "comet",
		Aliases: []string{"cometbft"},
		Short:   "CometBFT subcommands",
	}

	cometbftCmd.AddCommand(
		server.ShowNodeIDCmd(),
		server.ShowValidatorCmd(),
		server.ShowAddressCmd(),
		server.VersionCmd(),
		tmcmd.ResetAllCmd,
		tmcmd.ResetStateCmd,
		server.BootstrapStateCmd(opts.AppCreator),
	)

	startCmd := evmserver.StartCmd(opts)
	wrapJSONRPCAliasStartPreRun(startCmd)
	addStartFlags(startCmd)

	rootCmd.AddCommand(
		startCmd,
		cometbftCmd,
		server.ExportCmd(appExport, opts.DefaultNodeHome),
		version.NewVersionCommand(),
		server.NewRollbackCmd(opts.AppCreator, opts.DefaultNodeHome),
		evmserver.NewIndexTxCmd(),
	)
}

func wrapJSONRPCAliasStartPreRun(startCmd *cobra.Command) {
	originalPreRunE := startCmd.PreRunE
	startCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if originalPreRunE != nil {
			if err := originalPreRunE(cmd, args); err != nil {
				return err
			}
		}

		serverCtx := server.GetServerContextFromCmd(cmd)
		v := serverCtx.Viper
		if !v.GetBool("json-rpc.enable") {
			return nil
		}

		publicAddr := v.GetString("json-rpc.address")
		if publicAddr == "" {
			return nil
		}

		excludedAddrs := []string{
			v.GetString("json-rpc.ws-address"),
			v.GetString("json-rpc.metrics-address"),
			v.GetString("evm.geth-metrics-address"),
			v.GetString("api.address"),
			v.GetString("grpc.address"),
			v.GetString("lumera.json-rpc-ratelimit.proxy-address"),
		}
		if cometConfig := serverCtx.Config; cometConfig != nil {
			excludedAddrs = append(excludedAddrs,
				cometConfig.ProxyApp,
				cometConfig.PrivValidatorListenAddr,
			)
			if cometConfig.RPC != nil {
				excludedAddrs = append(excludedAddrs,
					cometConfig.RPC.ListenAddress,
					cometConfig.RPC.GRPCListenAddress,
					cometConfig.RPC.PprofListenAddress,
				)
			}
			if cometConfig.P2P != nil {
				excludedAddrs = append(excludedAddrs, cometConfig.P2P.ListenAddress)
			}
			if cometConfig.Instrumentation != nil {
				excludedAddrs = append(excludedAddrs, cometConfig.Instrumentation.PrometheusListenAddr)
			}
		}

		internalAddr, err := reserveLoopbackAddr(publicAddr, excludedAddrs...)
		if err != nil {
			return err
		}

		v.Set(app.JSONRPCAliasPublicAddrAppOpt, publicAddr)
		v.Set(app.JSONRPCAliasUpstreamAddrAppOpt, internalAddr)
		v.Set("json-rpc.address", internalAddr)
		return nil
	}
}

func reserveLoopbackAddr(publicAddr string, excludedAddrs ...string) (string, error) {
	internalAddr, err := loopbackAddrForPublic(publicAddr)
	if err != nil {
		return "", err
	}

	_, primaryPortText, err := net.SplitHostPort(internalAddr)
	if err != nil {
		return "", err
	}
	primaryPort, err := strconv.Atoi(primaryPortText)
	if err != nil {
		return "", err
	}

	excludedPorts := make(map[int]struct{}, len(excludedAddrs)+1)
	if publicPort, ok := portFromListenAddr(publicAddr); ok {
		excludedPorts[publicPort] = struct{}{}
	}
	for _, addr := range excludedAddrs {
		if port, ok := portFromListenAddr(addr); ok {
			excludedPorts[port] = struct{}{}
		}
	}

	// Verify a deterministic candidate is currently available. Closing it
	// before the native JSON-RPC server binds still leaves a small
	// external-process race, but the deterministic primary prevents sibling
	// lumerad processes with distinct public ports from selecting the same
	// upstream. If another service owns that primary, walk a deterministic
	// permutation of the unprivileged range instead of making an otherwise
	// valid public listener unusable. The relatively prime step keeps nearby
	// public ports from immediately falling back onto each other's primaries.
	const fallbackProbeStep = 7919
	for attempt := 0; attempt < unprivilegedPortCount; attempt++ {
		candidatePort := firstUnprivilegedPort +
			((primaryPort-firstUnprivilegedPort)+(attempt*fallbackProbeStep))%unprivilegedPortCount
		// The alias proxy and the daemon's other servers bind later in startup,
		// so never consume one of their configured ports as the native HTTP
		// server's upstream.
		if _, excluded := excludedPorts[candidatePort]; excluded {
			continue
		}

		candidateAddr := net.JoinHostPort("127.0.0.1", strconv.Itoa(candidatePort))
		ln, listenErr := net.Listen("tcp", candidateAddr)
		if listenErr != nil {
			if errors.Is(listenErr, syscall.EADDRINUSE) {
				continue
			}
			return "", fmt.Errorf("reserve internal JSON-RPC address %s: %w", candidateAddr, listenErr)
		}
		if closeErr := ln.Close(); closeErr != nil {
			return "", closeErr
		}
		return candidateAddr, nil
	}

	return "", fmt.Errorf("no unprivileged internal JSON-RPC port available for %s", publicAddr)
}

func portFromListenAddr(addr string) (int, bool) {
	addr = strings.TrimSpace(addr)
	if _, remainder, hasScheme := strings.Cut(addr, "://"); hasScheme {
		addr = remainder
	}
	_, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, false
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return 0, false
	}
	return port, true
}

const (
	firstUnprivilegedPort = 1024
	lastUnprivilegedPort  = 65535
	unprivilegedPortCount = lastUnprivilegedPort - firstUnprivilegedPort + 1
)

func loopbackAddrForPublic(publicAddr string) (string, error) {
	_, portText, err := net.SplitHostPort(publicAddr)
	if err != nil {
		return "", fmt.Errorf("parse public JSON-RPC address %q: %w", publicAddr, err)
	}
	publicPort, err := strconv.Atoi(portText)
	if err != nil || publicPort < 1 || publicPort > 65535 {
		return "", fmt.Errorf("invalid public JSON-RPC port %q", portText)
	}

	// Rotate within the unprivileged TCP port range. Integration fixtures use
	// ephemeral public ports, so rotating across the full 1..65535 range could
	// map them below 1024 and fail for non-root processes.
	internalPort := firstUnprivilegedPort +
		((publicPort - 1 + 32768) % unprivilegedPortCount)
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(internalPort)), nil
}

func addModuleInitFlags(startCmd *cobra.Command) {
	wasm.AddModuleInitFlags(startCmd)

	// Claim module flags for genesis CSV loading.
	// Registered on the start command so cobra accepts them, then bound to global
	// viper so x/claim's InitGenesis (which uses viper.GetBool/GetString) sees them.
	startCmd.Flags().String(claimtypes.FlagClaimsPath, "",
		fmt.Sprintf("Path to %s file or directory containing it", claimtypes.DefaultClaimsFileName))
	startCmd.Flags().Bool(claimtypes.FlagSkipClaimsCheck, true,
		"Skip claims.csv loading at genesis (default true; set false to load claim records)")
	_ = viper.BindPFlag(claimtypes.FlagClaimsPath, startCmd.Flags().Lookup(claimtypes.FlagClaimsPath))
	_ = viper.BindPFlag(claimtypes.FlagSkipClaimsCheck, startCmd.Flags().Lookup(claimtypes.FlagSkipClaimsCheck))
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
