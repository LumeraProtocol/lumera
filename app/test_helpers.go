package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cosmossdk.io/depinject"
	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	pruningtypes "cosmossdk.io/store/pruning/types"

	abci "github.com/cometbft/cometbft/abci/types"
	cmttypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	bam "github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/testutil/network"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module/testutil"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govtypesv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	ibcporttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"

	lcfg "github.com/LumeraProtocol/lumera/config"
	ibcmock "github.com/LumeraProtocol/lumera/tests/ibctesting/mock"
	claimtypes "github.com/LumeraProtocol/lumera/x/claim/types"
)

const (
	MockPort = ibcmock.ModuleName
)

func NewTestApp(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	loadLatest bool,
	appOpts servertypes.AppOptions,
	baseAppOptions ...func(*bam.BaseApp),
) (*App, error) {
	var (
		app        = &App{}
		appBuilder *runtime.AppBuilder

		appConfig = depinject.Configs(
			AppConfig(appOpts),
			depinject.Supply(
				appOpts,
				logger,
				app.GetIBCKeeper,
			),
		)
	)

	if err := depinject.Inject(appConfig,
		&appBuilder,
		&app.appCodec,
		&app.legacyAmino,
		&app.txConfig,
		&app.interfaceRegistry,
		&app.AuthKeeper,
		&app.BankKeeper,
		&app.StakingKeeper,
		&app.DistrKeeper,
		&app.ConsensusParamsKeeper,
		&app.SlashingKeeper,
		&app.MintKeeper,
		&app.GovKeeper,
		&app.CrisisKeeper,
		&app.UpgradeKeeper,
		&app.ParamsKeeper,
		&app.AuthzKeeper,
		&app.EvidenceKeeper,
		&app.FeeGrantKeeper,
		&app.GroupKeeper,
		&app.CircuitBreakerKeeper,
		&app.LumeraidKeeper,
	); err != nil {
		return nil, err
	}

	app.App = appBuilder.Build(db, traceStore, baseAppOptions...)
	if app.App == nil {
		return nil, errors.New("failed to initialize BaseApp")
	}

	if loadLatest {
		if err := app.LoadLatestVersion(); err != nil {
			return nil, err
		}
	}

	return app, nil
}

//// Setup initializes a new App instance for testing.
//func Setup(t *testing.T) *simapp.SimApp {
//	//db := dbm.NewMemDB()
//	//
//	//logger := log.NewNopLogger()
//	//
//	//app, err := NewTestApp(logger, db, nil, true, EmptyAppOptions{})
//	//if err != nil {
//	//	t.Fatalf("failed to initialize test app: %v", err)
//	//}
//	//
//	//return app
//
//	app := simapp.Setup(t, false)
//	return app
//}

// EmptyAppOptions is a mock implementation of AppOptions with no options set.
type EmptyAppOptions struct{}

// Get implements the AppOptions interface but always returns nil for testing.
func (ao EmptyAppOptions) Get(_ string) interface{} {
	return nil
}

// InitModuleAccounts initializes module accounts with their permissions for tests.
func InitModuleAccounts(ctx sdk.Context, ak authkeeper.AccountKeeper) {
	for acc, perms := range GetMaccPerms() {
		if ak.GetModuleAccount(ctx, acc) == nil {
			ak.SetModuleAccount(ctx, authtypes.NewEmptyModuleAccount(acc, perms...))
		}
	}
}

// SignAndDeliverWithoutCommit signs and delivers a transaction. No commit
func SignAndDeliverWithoutCommit(t *testing.T, txCfg client.TxConfig, app *bam.BaseApp, msgs []sdk.Msg, fees sdk.Coins, chainID string, accNums, accSeqs []uint64, blockTime time.Time, priv ...cryptotypes.PrivKey) (*abci.ResponseFinalizeBlock, error) {
	tx, err := simtestutil.GenSignedMockTx(
		rand.New(rand.NewSource(time.Now().UnixNano())),
		txCfg,
		msgs,
		fees,
		simtestutil.DefaultGenTxGas,
		chainID,
		accNums,
		accSeqs,
		priv...,
	)
	require.NoError(t, err)

	bz, err := txCfg.TxEncoder()(tx)
	require.NoError(t, err)

	return app.FinalizeBlock(&abci.RequestFinalizeBlock{
		Height: app.LastBlockHeight() + 1,
		Time:   blockTime,
		Txs:    [][]byte{bz},
	})
}

func GetDefaultWasmOptions() []wasmkeeper.Option {
	return []wasmkeeper.Option{
		wasmkeeper.WithMessageHandlerDecorator(func(old wasmkeeper.Messenger) wasmkeeper.Messenger {
			return old
		}),
		wasmkeeper.WithQueryPlugins(nil),
	}
}

func setup(t testing.TB, chainID string, withGenesis bool, invCheckPeriod uint, wasmOpts ...wasmkeeper.Option) (*App, GenesisState) {
	db := dbm.NewMemDB()
	nodeHome := t.TempDir()
	snapshotDir := filepath.Join(nodeHome, "data", "snapshots")

	snapshotDB, err := dbm.NewDB("metadata", dbm.GoLevelDBBackend, snapshotDir)
	require.NoError(t, err)
	t.Cleanup(func() { snapshotDB.Close() })
	require.NoError(t, err)

	appOptions := make(simtestutil.AppOptionsMap, 0)
	appOptions[flags.FlagHome] = nodeHome // ensure unique folder
	appOptions[FlagWasmHomeDir] = nodeHome
	appOptions[server.FlagInvCheckPeriod] = invCheckPeriod
	appOptions[IBCModuleRegisterFnOption] = func(ibcRouter *ibcporttypes.Router) {
		// Register the mock IBC module for testing
		ibcRouter.AddRoute(MockPort, ibcmock.NewMockIBCModule(nil, MockPort))
	}

	app := New(log.NewNopLogger(), db, nil, true, appOptions, wasmOpts, bam.SetChainID(chainID))
	if withGenesis {
		return app, app.DefaultGenesis()
	}
	return app, GenesisState{}
}

// Setup initializes a new WasmApp. A Nop logger is set in WasmApp.
func Setup(tb testing.TB, wasmOpts ...wasmkeeper.Option) *App {
	tb.Helper()

	privVal := cmttypes.NewMockPV()
	pubKey, err := privVal.GetPubKey()
	require.NoError(tb, err)

	// create validator set with single validator
	validator := cmttypes.NewValidator(pubKey, 1)
	valSet := cmttypes.NewValidatorSet([]*cmttypes.Validator{validator})

	// generate genesis account
	senderPrivKey := secp256k1.GenPrivKey()
	acc := authtypes.NewBaseAccount(senderPrivKey.PubKey().Address().Bytes(), senderPrivKey.PubKey(), 0, 0)
	genBals := []banktypes.Balance{}
	balance := banktypes.Balance{
		Address: acc.GetAddress().String(),
		Coins:   sdk.NewCoins(sdk.NewInt64Coin(lcfg.ChainDenom, 100_000_000_000_000)),
	}
	genBals = append(genBals, balance)
	chainID := "testing"

	app := SetupWithGenesisValSet(tb, valSet, []authtypes.GenesisAccount{acc}, chainID, sdk.DefaultPowerReduction, genBals, wasmOpts...)

	return app
}

// SetupWithGenesisValSet initializes a new Lumera App with a validator set and genesis accounts
// that also act as delegators. For simplicity, each validator is bonded with a delegation
// of one consensus engine unit in the default token of the WasmApp from first genesis
// account. A Nop logger is set in WasmApp.
func SetupWithGenesisValSet(
	tb testing.TB,
	valSet *cmttypes.ValidatorSet,
	genAccs []authtypes.GenesisAccount,
	chainID string,
	powerReduction sdkmath.Int,
	balances []banktypes.Balance,
	wasmOpts ...wasmkeeper.Option,
) *App {
	tb.Helper()

	app, genesisState := setup(tb, chainID, true, 5, wasmOpts...)
	genesisState = GenesisStateWithValSet(tb, app.AppCodec(), genesisState, valSet, genAccs, balances...)

	stateBytes, err := json.MarshalIndent(genesisState, "", " ")
	require.NoError(tb, err)

	viper.Set(claimtypes.FlagSkipClaimsCheck, true)

	// init chain will set the validator set and initialize the genesis accounts
	consensusParams := simtestutil.DefaultConsensusParams
	consensusParams.Block.MaxGas = 100 * simtestutil.DefaultGenTxGas
	_, err = app.InitChain(&abci.RequestInitChain{
		ChainId:         chainID,
		Time:            time.Now().UTC(),
		Validators:      []abci.ValidatorUpdate{},
		ConsensusParams: consensusParams,
		InitialHeight:   app.LastBlockHeight() + 1,
		AppStateBytes:   stateBytes,
	})
	require.NoError(tb, err)

	_, err = app.FinalizeBlock(&abci.RequestFinalizeBlock{
		Height:             app.LastBlockHeight() + 1,
		Hash:               app.LastCommitID().Hash,
		NextValidatorsHash: valSet.Hash(),
	})
	require.NoError(tb, err)

	return app
}

// SetupWithEmptyStore set up a wasmd app instance with empty DB
func SetupWithEmptyStore(tb testing.TB) *App {
	app, _ := setup(tb, "testing", false, 0)
	return app
}

// GenesisStateWithSingleValidator initializes GenesisState with a single validator and genesis accounts
// that also act as delegators.
func GenesisStateWithSingleValidator(tb testing.TB, app *App) GenesisState {
	tb.Helper()

	privVal := cmttypes.NewMockPV()
	pubKey, err := privVal.GetPubKey()
	require.NoError(tb, err)

	// create validator set with single validator
	validator := cmttypes.NewValidator(pubKey, 1)
	valSet := cmttypes.NewValidatorSet([]*cmttypes.Validator{validator})

	// generate genesis account
	senderPrivKey := secp256k1.GenPrivKey()
	acc := authtypes.NewBaseAccount(senderPrivKey.PubKey().Address().Bytes(), senderPrivKey.PubKey(), 0, 0)
	balances := []banktypes.Balance{
		{
			Address: acc.GetAddress().String(),
			Coins:   sdk.NewCoins(sdk.NewCoin(lcfg.ChainDenom, sdkmath.NewInt(100000000000000))),
		},
	}

	genesisState := app.DefaultGenesis()
	genesisState = GenesisStateWithValSet(tb, app.AppCodec(), genesisState, valSet, []authtypes.GenesisAccount{acc}, balances...)

	return genesisState
}

// AddTestAddrsIncremental constructs and returns accNum amount of accounts with an
// initial balance of accAmt in random order
func AddTestAddrsIncremental(app *App, ctx sdk.Context, accNum int, accAmt sdkmath.Int) []sdk.AccAddress {
	return addTestAddrs(app, ctx, accNum, accAmt, simtestutil.CreateIncrementalAccounts)
}

func addTestAddrs(app *App, ctx sdk.Context, accNum int, accAmt sdkmath.Int, strategy simtestutil.GenerateAccountStrategy) []sdk.AccAddress {
	testAddrs := strategy(accNum)
	bondDenom, err := app.StakingKeeper.BondDenom(ctx)
	if err != nil {
		panic(err)
	}

	initCoins := sdk.NewCoins(sdk.NewCoin(bondDenom, accAmt))

	for _, addr := range testAddrs {
		initAccountWithCoins(app, ctx, addr, initCoins)
	}

	return testAddrs
}

func initAccountWithCoins(app *App, ctx sdk.Context, addr sdk.AccAddress, coins sdk.Coins) {
	err := app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coins)
	if err != nil {
		panic(err)
	}

	err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, addr, coins)
	if err != nil {
		panic(err)
	}
}

// GenesisStateWithValSet returns a new genesis state with the validator set
// copied from simtestutil with delegation not added to supply
func GenesisStateWithValSet(
	tb testing.TB,
	codec codec.Codec,
	genesisState map[string]json.RawMessage,
	valSet *cmttypes.ValidatorSet,
	genAccs []authtypes.GenesisAccount,
	balances ...banktypes.Balance,
) map[string]json.RawMessage {
	// set genesis accounts
	authGenesis := authtypes.NewGenesisState(authtypes.DefaultParams(), genAccs)
	genesisState[authtypes.ModuleName] = codec.MustMarshalJSON(authGenesis)

	validators := make([]stakingtypes.Validator, 0, len(valSet.Validators))
	delegations := make([]stakingtypes.Delegation, 0, len(valSet.Validators))

	bondAmt := sdk.DefaultPowerReduction

	for _, val := range valSet.Validators {
		pk, err := cryptocodec.FromCmtPubKeyInterface(val.PubKey)
		require.NoError(tb, err, "failed to convert pubkey")

		pkAny, err := codectypes.NewAnyWithValue(pk)
		require.NoError(tb, err, "failed to create new any")

		validator := stakingtypes.Validator{
			OperatorAddress:   sdk.ValAddress(val.Address).String(),
			ConsensusPubkey:   pkAny,
			Jailed:            false,
			Status:            stakingtypes.Bonded,
			Tokens:            bondAmt,
			DelegatorShares:   sdkmath.LegacyOneDec(),
			Description:       stakingtypes.Description{},
			UnbondingHeight:   int64(0),
			UnbondingTime:     time.Unix(0, 0).UTC(),
			Commission:        stakingtypes.NewCommission(sdkmath.LegacyZeroDec(), sdkmath.LegacyZeroDec(), sdkmath.LegacyZeroDec()),
			MinSelfDelegation: sdkmath.ZeroInt(),
		}

		validators = append(validators, validator)
		delegations = append(delegations, stakingtypes.NewDelegation(genAccs[0].GetAddress().String(), sdk.ValAddress(val.Address).String(), sdkmath.LegacyOneDec()))
	}

	// set validators and delegations
	stakingParams := stakingtypes.DefaultParams()
	stakingParams.BondDenom = lcfg.ChainDenom
	stakingGenesis := stakingtypes.NewGenesisState(stakingParams, validators, delegations)
	genesisState[stakingtypes.ModuleName] = codec.MustMarshalJSON(stakingGenesis)

	bondDenom := stakingParams.BondDenom

	// ensure inflation mints the chain denom
	mintGenesis := minttypes.DefaultGenesisState()
	mintGenesis.Params.MintDenom = lcfg.ChainDenom
	genesisState[minttypes.ModuleName] = codec.MustMarshalJSON(mintGenesis)

	signingInfos := make([]slashingtypes.SigningInfo, len(valSet.Validators))
	for i, val := range valSet.Validators {
		signingInfos[i] = slashingtypes.SigningInfo{
			Address:              sdk.ConsAddress(val.Address).String(),
			ValidatorSigningInfo: slashingtypes.ValidatorSigningInfo{},
		}
	}
	slashingGenesis := slashingtypes.NewGenesisState(slashingtypes.DefaultParams(), signingInfos, nil)

	govParams := govtypesv1.DefaultParams()
	govParams.MinDeposit = sdk.NewCoins(sdk.NewCoin(lcfg.ChainDenom, govtypesv1.DefaultMinDepositTokens))
	govParams.ExpeditedMinDeposit = sdk.NewCoins(sdk.NewCoin(lcfg.ChainDenom, govtypesv1.DefaultMinExpeditedDepositTokens))

	govGenesis := govtypesv1.DefaultGenesisState()
	if bz, ok := genesisState[govtypes.ModuleName]; ok && len(bz) != 0 {
		require.NoError(tb, codec.UnmarshalJSON(bz, govGenesis))
	}
	govGenesis.Params = &govParams
	genesisState[govtypes.ModuleName] = codec.MustMarshalJSON(govGenesis)

	genesisState[slashingtypes.ModuleName] = codec.MustMarshalJSON(slashingGenesis)

	// add bonded amount to bonded pool module account
	balances = append(balances, banktypes.Balance{
		Address: authtypes.NewModuleAddress(stakingtypes.BondedPoolName).String(),
		Coins:   sdk.Coins{sdk.NewCoin(bondDenom, bondAmt.MulRaw(int64(len(valSet.Validators))))},
	})

	totalSupply := sdk.NewCoins()
	for _, b := range balances {
		// add genesis acc tokens to total supply
		totalSupply = totalSupply.Add(b.Coins...)
	}

	// update total supply
	bankGenesis := banktypes.NewGenesisState(banktypes.DefaultGenesisState().Params, balances, totalSupply, []banktypes.Metadata{}, []banktypes.SendEnabled{})
	genesisState[banktypes.ModuleName] = codec.MustMarshalJSON(bankGenesis)

	return genesisState
}

func NewTestNetworkFixture() network.TestFixture {
	dir, err := os.MkdirTemp("", "lumera")
	if err != nil {
		panic(fmt.Sprintf("failed creating temporary directory: %v", err))
	}
	defer os.RemoveAll(dir)

	// Create initial app instance
	app := New(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		nil,
		true,
		simtestutil.NewAppOptionsWithFlagHome(dir),
		GetDefaultWasmOptions(),
	)
	if err != nil {
		panic(fmt.Sprintf("failed creating app: %v", err))
	}

	// App constructor function for validators
	appCtr := func(val network.ValidatorI) servertypes.Application {
		app := New(
			val.GetCtx().Logger,
			dbm.NewMemDB(),
			nil,
			true,
			simtestutil.NewAppOptionsWithFlagHome(val.GetCtx().Config.RootDir),
			GetDefaultWasmOptions(),
			bam.SetPruning(pruningtypes.NewPruningOptionsFromString(val.GetAppConfig().Pruning)),
			bam.SetMinGasPrices(val.GetAppConfig().MinGasPrices),
			bam.SetChainID(val.GetCtx().Viper.GetString(flags.FlagChainID)),
		)
		if err != nil {
			panic(fmt.Sprintf("failed creating validator app: %v", err))
		}
		return app
	}

	return network.TestFixture{
		AppConstructor: appCtr,
		GenesisState:   app.DefaultGenesis(),
		EncodingConfig: testutil.TestEncodingConfig{
			InterfaceRegistry: app.InterfaceRegistry(),
			Codec:             app.AppCodec(),
			TxConfig:          app.TxConfig(),
			Amino:             app.LegacyAmino(),
		},
	}
}
