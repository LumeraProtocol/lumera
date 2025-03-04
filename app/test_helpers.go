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

	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	abci "github.com/cometbft/cometbft/abci/types"
	cmttypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
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
	"github.com/cosmos/cosmos-sdk/testutil/mock"
	"github.com/cosmos/cosmos-sdk/testutil/network"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module/testutil"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"
	"github.com/stretchr/testify/require"
)

func NewTestApp(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	loadLatest bool,
	appOpts servertypes.AppOptions,
	baseAppOptions ...func(*baseapp.BaseApp),
) (*App, error) {
	var (
		app        = &App{ScopedKeepers: make(map[string]capabilitykeeper.ScopedKeeper)}
		appBuilder *runtime.AppBuilder

		appConfig = depinject.Configs(
			AppConfig(),
			depinject.Supply(appOpts, logger, app.GetIBCKeeper, app.GetCapabilityScopedKeeper),
		)
	)

	if err := depinject.Inject(appConfig,
		&appBuilder,
		&app.appCodec,
		&app.legacyAmino,
		&app.txConfig,
		&app.interfaceRegistry,
		&app.AccountKeeper,
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
		&app.NFTKeeper,
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

func setup(t testing.TB, chainID string, withGenesis bool, invCheckPeriod uint, opts ...wasmkeeper.Option) (*App, GenesisState) {
	db := dbm.NewMemDB()
	nodeHome := t.TempDir()
	snapshotDir := filepath.Join(nodeHome, "data", "snapshots")

	snapshotDB, err := dbm.NewDB("metadata", dbm.GoLevelDBBackend, snapshotDir)
	require.NoError(t, err)
	t.Cleanup(func() { snapshotDB.Close() })
	require.NoError(t, err)

	appOptions := make(simtestutil.AppOptionsMap, 0)
	appOptions[flags.FlagHome] = nodeHome // ensure unique folder
	appOptions[server.FlagInvCheckPeriod] = invCheckPeriod
	app, err := New(log.NewNopLogger(), db, nil, true, appOptions, bam.SetChainID(chainID))
	if withGenesis {
		return app, app.DefaultGenesis()
	}
	return app, GenesisState{}
}

func SetupWithGenesisValSet(
	t *testing.T,
	valSet *cmttypes.ValidatorSet,
	genAccs []authtypes.GenesisAccount,
	chainID string,
	opts []wasmkeeper.Option,
	balances ...banktypes.Balance,
) *App {
	t.Helper()

	app, genesisState := setup(t, chainID, true, 5, opts...)
	genesisState, err := GenesisStateWithValSet(app.AppCodec(), genesisState, valSet, genAccs, balances...)
	require.NoError(t, err)

	stateBytes, err := json.MarshalIndent(genesisState, "", " ")
	require.NoError(t, err)

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
	require.NoError(t, err)

	_, err = app.FinalizeBlock(&abci.RequestFinalizeBlock{
		Height:             app.LastBlockHeight() + 1,
		Hash:               app.LastCommitID().Hash,
		NextValidatorsHash: valSet.Hash(),
	})
	require.NoError(t, err)

	return app
}

// Setup initializes a new WasmApp. A Nop logger is set in WasmApp.
func Setup(t *testing.T, opts ...wasmkeeper.Option) *App {
	t.Helper()

	privVal := mock.NewPV()
	pubKey, err := privVal.GetPubKey()
	require.NoError(t, err)

	// create validator set with single validator
	validator := cmttypes.NewValidator(pubKey, 1)
	valSet := cmttypes.NewValidatorSet([]*cmttypes.Validator{validator})

	// generate genesis account
	senderPrivKey := secp256k1.GenPrivKey()
	acc := authtypes.NewBaseAccount(senderPrivKey.PubKey().Address().Bytes(), senderPrivKey.PubKey(), 0, 0)
	balance := banktypes.Balance{
		Address: acc.GetAddress().String(),
		Coins:   sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(100000000000000))),
	}
	chainID := "testing"
	app := SetupWithGenesisValSet(t, valSet, []authtypes.GenesisAccount{acc}, chainID, opts, balance)

	return app
}

// SetupWithEmptyStore set up a wasmd app instance with empty DB
func SetupWithEmptyStore(t testing.TB) *App {
	app, _ := setup(t, "testing", false, 0)
	return app
}

// GenesisStateWithValSet returns a new genesis state with the validator set
// copied from simtestutil with delegation not added to supply
func GenesisStateWithValSet(
	codec codec.Codec,
	genesisState map[string]json.RawMessage,
	valSet *cmttypes.ValidatorSet,
	genAccs []authtypes.GenesisAccount,
	balances ...banktypes.Balance,
) (map[string]json.RawMessage, error) {
	// set genesis accounts
	authGenesis := authtypes.NewGenesisState(authtypes.DefaultParams(), genAccs)
	genesisState[authtypes.ModuleName] = codec.MustMarshalJSON(authGenesis)

	validators := make([]stakingtypes.Validator, 0, len(valSet.Validators))
	delegations := make([]stakingtypes.Delegation, 0, len(valSet.Validators))

	bondAmt := sdk.DefaultPowerReduction

	for _, val := range valSet.Validators {
		pk, err := cryptocodec.FromCmtPubKeyInterface(val.PubKey)
		if err != nil {
			return nil, fmt.Errorf("failed to convert pubkey: %w", err)
		}

		pkAny, err := codectypes.NewAnyWithValue(pk)
		if err != nil {
			return nil, fmt.Errorf("failed to create new any: %w", err)
		}

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
	stakingGenesis := stakingtypes.NewGenesisState(stakingtypes.DefaultParams(), validators, delegations)
	genesisState[stakingtypes.ModuleName] = codec.MustMarshalJSON(stakingGenesis)

	signingInfos := make([]slashingtypes.SigningInfo, len(valSet.Validators))
	for i, val := range valSet.Validators {
		signingInfos[i] = slashingtypes.SigningInfo{
			Address:              sdk.ConsAddress(val.Address).String(),
			ValidatorSigningInfo: slashingtypes.ValidatorSigningInfo{},
		}
	}
	slashingGenesis := slashingtypes.NewGenesisState(slashingtypes.DefaultParams(), signingInfos, nil)
	genesisState[slashingtypes.ModuleName] = codec.MustMarshalJSON(slashingGenesis)

	// add bonded amount to bonded pool module account
	balances = append(balances, banktypes.Balance{
		Address: authtypes.NewModuleAddress(stakingtypes.BondedPoolName).String(),
		Coins:   sdk.Coins{sdk.NewCoin(sdk.DefaultBondDenom, bondAmt.MulRaw(int64(len(valSet.Validators))))},
	})

	totalSupply := sdk.NewCoins()
	for _, b := range balances {
		// add genesis acc tokens to total supply
		totalSupply = totalSupply.Add(b.Coins...)
	}

	// update total supply
	bankGenesis := banktypes.NewGenesisState(banktypes.DefaultGenesisState().Params, balances, totalSupply, []banktypes.Metadata{}, []banktypes.SendEnabled{})
	genesisState[banktypes.ModuleName] = codec.MustMarshalJSON(bankGenesis)
	println(string(genesisState[banktypes.ModuleName]))
	return genesisState, nil
}

func NewTestNetworkFixture() network.TestFixture {
	dir, err := os.MkdirTemp("", "lumera")
	if err != nil {
		panic(fmt.Sprintf("failed creating temporary directory: %v", err))
	}
	defer os.RemoveAll(dir)

	// Create initial app instance
	app, err := New(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		nil,
		true,
		simtestutil.NewAppOptionsWithFlagHome(dir),
	)
	if err != nil {
		panic(fmt.Sprintf("failed creating app: %v", err))
	}

	// App constructor function for validators
	appCtr := func(val network.ValidatorI) servertypes.Application {
		app, err := New(
			val.GetCtx().Logger,
			dbm.NewMemDB(),
			nil,
			true,
			simtestutil.NewAppOptionsWithFlagHome(val.GetCtx().Config.RootDir),
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
