package wasm_test

import (
	"encoding/binary"
	"encoding/json"
	"testing"
	"time"
	"fmt"
	"os"
	"bytes"
	"path/filepath"

	"github.com/stretchr/testify/require"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/x/evidence"
	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	"cosmossdk.io/x/tx/signing"
	storetypes "cosmossdk.io/store/types"
	"cosmossdk.io/x/upgrade"
	upgradekeeper "cosmossdk.io/x/upgrade/keeper"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	secp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/std"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdkaddress "github.com/cosmos/cosmos-sdk/types/address"
	"github.com/cosmos/cosmos-sdk/types/module"

	"github.com/cosmos/cosmos-sdk/x/auth"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	"github.com/cosmos/cosmos-sdk/x/bank"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/distribution"
	distributionkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	evidencetypes "cosmossdk.io/x/evidence/types"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govtypesv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	govclient "github.com/cosmos/cosmos-sdk/x/gov/client"
	"github.com/cosmos/cosmos-sdk/x/mint"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/cosmos/cosmos-sdk/x/params"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	paramsclient "github.com/cosmos/cosmos-sdk/x/params/client"
	proto "github.com/cosmos/gogoproto/proto"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cometbft/cometbft/crypto"
	"github.com/cometbft/cometbft/crypto/ed25519"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"

	ibc "github.com/cosmos/ibc-go/v10/modules/core"
	ibctransfer "github.com/cosmos/ibc-go/v10/modules/apps/transfer"
	ibctransfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	wasmKeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"

	lcfg "github.com/LumeraProtocol/lumera/config"
	ibcmock "github.com/LumeraProtocol/lumera/tests/ibctesting/mock"
	"github.com/LumeraProtocol/lumera/tests/testdata"
)

const (
	// TestDataDir is the directory where test data files are stored
	TestDataDir = "../../testdata"
)

func GetTestDataFilePath(filename string) string {
	return filepath.Join(TestDataDir, filename)
}

// ensure store code returns the expected response
func assertStoreCodeResponse(t *testing.T, data []byte, expected uint64) {
	var pStoreResp wasmtypes.MsgStoreCodeResponse
	require.NoError(t, pStoreResp.Unmarshal(data))
	require.Equal(t, pStoreResp.CodeID, expected)
}

// ensure execution returns the expected data
func assertExecuteResponse(t *testing.T, data, expected []byte) {
	var pExecResp wasmtypes.MsgExecuteContractResponse
	require.NoError(t, pExecResp.Unmarshal(data))
	require.Equal(t, pExecResp.Data, expected)
}

// ensures this returns a valid bech32 address and returns it
func parseInitResponse(t *testing.T, data []byte) string {
	var pInstResp wasmtypes.MsgInstantiateContractResponse
	require.NoError(t, pInstResp.Unmarshal(data))
	require.NotEmpty(t, pInstResp.Address)
	addr := pInstResp.Address
	// ensure this is a valid sdk address
	_, err := sdk.AccAddressFromBech32(addr)
	require.NoError(t, err)
	return addr
}

func must[t any](s t, err error) t {
	if err != nil {
		panic(err)
	}
	return s
}

func mustUnmarshal(t *testing.T, data []byte, res interface{}) {
	t.Helper()
	err := json.Unmarshal(data, res)
	require.NoError(t, err)
}

func mustMarshal(t *testing.T, r interface{}) []byte {
	t.Helper()
	bz, err := json.Marshal(r)
	require.NoError(t, err)
	return bz
}

// this will commit the current set, update the block height and set historic info
// basically, letting two blocks pass
func nextBlock(ctx sdk.Context, stakingKeeper *stakingkeeper.Keeper) sdk.Context {
	if _, err := stakingKeeper.EndBlocker(ctx); err != nil {
		panic(err)
	}
	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1)
	_ = stakingKeeper.BeginBlocker(ctx)
	return ctx
}

func setValidatorRewards(ctx sdk.Context, stakingKeeper *stakingkeeper.Keeper, distKeeper distributionkeeper.Keeper, valAddr sdk.ValAddress, reward string) {
	// allocate some rewards
	vali, err := stakingKeeper.Validator(ctx, valAddr)
	if err != nil {
		panic(err)
	}
	amount, err := sdkmath.LegacyNewDecFromStr(reward)
	if err != nil {
		panic(err)
	}
	payout := sdk.DecCoins{{Denom: lcfg.ChainDenom, Amount: amount}}
	err = distKeeper.AllocateTokensToValidator(ctx, vali, payout)
	if err != nil {
		panic(err)
	}
}

// adds a few validators and returns a list of validators that are registered
func addValidator(t *testing.T, ctx sdk.Context, stakingKeeper *stakingkeeper.Keeper, faucet *wasmKeeper.TestFaucet, value sdk.Coin) sdk.ValAddress {
	owner := faucet.NewFundedRandomAccount(ctx, value)

	privKey := secp256k1.GenPrivKey()
	pubKey := privKey.PubKey()
	valAddr := sdk.ValAddress(owner)

	pkAny, err := codectypes.NewAnyWithValue(pubKey)
	require.NoError(t, err)
	msg := &stakingtypes.MsgCreateValidator{
		Description: stakingtypes.Description{
			Moniker: "Validator power",
		},
		Commission: stakingtypes.CommissionRates{
			Rate:          sdkmath.LegacyMustNewDecFromStr("0.1"),
			MaxRate:       sdkmath.LegacyMustNewDecFromStr("0.2"),
			MaxChangeRate: sdkmath.LegacyMustNewDecFromStr("0.01"),
		},
		MinSelfDelegation: sdkmath.OneInt(),
		DelegatorAddress:  owner.String(),
		ValidatorAddress:  valAddr.String(),
		Pubkey:            pkAny,
		Value:             value,
	}
	_, err = stakingkeeper.NewMsgServerImpl(stakingKeeper).CreateValidator(ctx, msg)
	require.NoError(t, err)
	return valAddr
}

// reflectEncoders needs to be registered in test setup to handle custom message callbacks
func reflectEncoders(cdc codec.Codec) *wasmKeeper.MessageEncoders {
	return &wasmKeeper.MessageEncoders{
		Custom: fromReflectRawMsg(cdc),
	}
}

/**** Code to support custom messages *****/

type reflectCustomMsg struct {
	Debug string `json:"debug,omitempty"`
	Raw   []byte `json:"raw,omitempty"`
}

// fromReflectRawMsg decodes msg.Data to an sdk.Msg using proto Any and json encoding.
// this needs to be registered on the Encoders
func fromReflectRawMsg(cdc codec.Codec) wasmKeeper.CustomEncoder {
	return func(_sender sdk.AccAddress, msg json.RawMessage) ([]sdk.Msg, error) {
		var custom reflectCustomMsg
		err := json.Unmarshal(msg, &custom)
		if err != nil {
			return nil, errorsmod.Wrap(sdkerrors.ErrJSONUnmarshal, err.Error())
		}
		if custom.Raw != nil {
			var codecAny codectypes.Any
			if err := cdc.UnmarshalJSON(custom.Raw, &codecAny); err != nil {
				return nil, errorsmod.Wrap(sdkerrors.ErrJSONUnmarshal, err.Error())
			}
			var msg sdk.Msg
			if err := cdc.UnpackAny(&codecAny, &msg); err != nil {
				return nil, err
			}
			return []sdk.Msg{msg}, nil
		}
		if custom.Debug != "" {
			return nil, errorsmod.Wrapf(wasmtypes.ErrInvalidMsg, "Custom Debug: %s", custom.Debug)
		}
		return nil, errorsmod.Wrap(wasmtypes.ErrInvalidMsg, "Unknown Custom message variant")
	}
}

var moduleBasics = module.NewBasicManager(
	auth.AppModuleBasic{},
	bank.AppModuleBasic{},
	staking.AppModuleBasic{},
	mint.AppModuleBasic{},
	distribution.AppModuleBasic{},
	gov.NewAppModuleBasic([]govclient.ProposalHandler{
		paramsclient.ProposalHandler,
	}),
	params.AppModuleBasic{},
	slashing.AppModuleBasic{},
	ibc.AppModuleBasic{},
	upgrade.AppModuleBasic{},
	evidence.AppModuleBasic{},
	ibctransfer.AppModuleBasic{},
	vesting.AppModuleBasic{},
)

func MakeEncodingConfig(t testing.TB) moduletestutil.TestEncodingConfig {
	encodingConfig := moduletestutil.MakeTestEncodingConfig(
		auth.AppModule{},
		bank.AppModule{},
		staking.AppModule{},
		mint.AppModule{},
		slashing.AppModule{},
		gov.AppModule{},
		ibc.AppModule{},
		ibctransfer.AppModule{},
		vesting.AppModule{},
	)
	signingOpts := signing.Options{
		AddressCodec: addresscodec.NewBech32Codec(lcfg.AccountAddressPrefix),
		ValidatorAddressCodec: addresscodec.NewBech32Codec(lcfg.ValidatorAddressPrefix),
	}

	protoFiles, err := proto.MergedRegistry()
	require.NoError(t, err)

	interfaceRegistry, err := codectypes.NewInterfaceRegistryWithOptions(codectypes.InterfaceRegistryOptions{
		ProtoFiles: protoFiles,
		SigningOptions: signingOpts,
	})
	require.NoError(t, err)

	protoCodec := codec.NewProtoCodec(interfaceRegistry)
	encodingConfig.InterfaceRegistry = interfaceRegistry
	encodingConfig.Codec = protoCodec
	std.RegisterInterfaces(interfaceRegistry)
	moduleBasics.RegisterInterfaces(interfaceRegistry)
	// add wasmd types
	wasmtypes.RegisterInterfaces(interfaceRegistry)
	wasmtypes.RegisterLegacyAminoCodec(encodingConfig.Amino)

	return encodingConfig
}

// CreateDefaultTestInput common settings for CreateTestInput
func CreateDefaultTestInput(t testing.TB) (sdk.Context, wasmKeeper.TestKeepers) {
	return CreateTestInput(t, false, []string{"staking"})
}

// CreateTestInput encoders can be nil to accept the defaults, or set it to override some of the message handlers (like default)
func CreateTestInput(t testing.TB, isCheckTx bool, availableCapabilities []string, opts ...wasmKeeper.Option) (sdk.Context, wasmKeeper.TestKeepers) {
	// Load default wasm config
	return createTestInput(t, isCheckTx, availableCapabilities, wasmtypes.DefaultNodeConfig(), wasmtypes.VMConfig{}, dbm.NewMemDB(), opts...)
}

// encoders can be nil to accept the defaults, or set it to override some of the message handlers (like default)
func createTestInput(
	t testing.TB,
	isCheckTx bool,
	availableCapabilities []string,
	nodeConfig wasmtypes.NodeConfig,
	vmConfig wasmtypes.VMConfig,
	db dbm.DB,
	opts ...wasmKeeper.Option,
) (sdk.Context, wasmKeeper.TestKeepers) {
	tempDir := t.TempDir()

	keys := storetypes.NewKVStoreKeys(
		authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey,
		minttypes.StoreKey, distributiontypes.StoreKey, slashingtypes.StoreKey,
		govtypes.StoreKey, paramstypes.StoreKey, ibcexported.StoreKey, upgradetypes.StoreKey,
		evidencetypes.StoreKey, ibctransfertypes.StoreKey,
		authzkeeper.StoreKey,
		wasmtypes.StoreKey,
	)
	// Use test logger at info level to keep errors/warnings
	logger := log.NewTestLoggerInfo(t)
	ms := store.NewCommitMultiStore(db, logger, storemetrics.NewNoOpMetrics())
	for _, v := range keys {
		ms.MountStoreWithDB(v, storetypes.StoreTypeIAVL, db)
	}
	tkeys := storetypes.NewTransientStoreKeys(paramstypes.TStoreKey)
	for _, v := range tkeys {
		ms.MountStoreWithDB(v, storetypes.StoreTypeTransient, db)
	}

	require.NoError(t, ms.LoadLatestVersion())

	ctx := sdk.NewContext(ms, tmproto.Header{
		Height: 1234567,
		Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
	}, isCheckTx, logger)
	ctx = wasmtypes.WithTXCounter(ctx, 0)

	encodingConfig := MakeEncodingConfig(t)
	appCodec, legacyAmino := encodingConfig.Codec, encodingConfig.Amino

	paramsKeeper := paramskeeper.NewKeeper(
		appCodec,
		legacyAmino,
		keys[paramstypes.StoreKey],
		tkeys[paramstypes.StoreKey],
	)
	for _, m := range []string{
		authtypes.ModuleName,
		banktypes.ModuleName,
		stakingtypes.ModuleName,
		minttypes.ModuleName,
		distributiontypes.ModuleName,
		slashingtypes.ModuleName,
		ibctransfertypes.ModuleName,
		ibcexported.ModuleName,
		govtypes.ModuleName,
		wasmtypes.ModuleName,
	} {
		paramsKeeper.Subspace(m)
	}
	subspace := func(m string) paramstypes.Subspace {
		r, ok := paramsKeeper.GetSubspace(m)
		require.True(t, ok)
		return r
	}
	maccPerms := map[string][]string{ // module account permissions
		authtypes.FeeCollectorName:     nil,
		distributiontypes.ModuleName:   nil,
		minttypes.ModuleName:           {authtypes.Minter},
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
		govtypes.ModuleName:            {authtypes.Burner},
		ibctransfertypes.ModuleName:    {authtypes.Minter, authtypes.Burner},
		wasmtypes.ModuleName:           {authtypes.Burner},
	}

	accountKeeper := authkeeper.NewAccountKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[authtypes.StoreKey]),
		authtypes.ProtoBaseAccount,
		maccPerms,
		authcodec.NewBech32Codec(lcfg.AccountAddressPrefix),
		lcfg.AccountAddressPrefix,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	blockedAddrs := make(map[string]bool)
	for acc := range maccPerms {
		blockedAddrs[authtypes.NewModuleAddress(acc).String()] = true
	}
	require.NoError(t, accountKeeper.Params.Set(ctx, authtypes.DefaultParams()))

	bankKeeper := bankkeeper.NewBaseKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[banktypes.StoreKey]),
		accountKeeper,
		blockedAddrs,
		authtypes.NewModuleAddress(banktypes.ModuleName).String(),
		logger,
	)
	require.NoError(t, bankKeeper.SetParams(ctx, banktypes.DefaultParams()))

	stakingKeeper := stakingkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[stakingtypes.StoreKey]),
		accountKeeper,
		bankKeeper,
		authtypes.NewModuleAddress(stakingtypes.ModuleName).String(),
		authcodec.NewBech32Codec(sdk.GetConfig().GetBech32ValidatorAddrPrefix()),
		authcodec.NewBech32Codec(sdk.GetConfig().GetBech32ConsensusAddrPrefix()),
	)
	stakingParams := wasmKeeper.TestingStakeParams
	stakingParams.BondDenom = lcfg.ChainDenom
	require.NoError(t, stakingKeeper.SetParams(ctx, stakingParams))

	distKeeper := distributionkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[distributiontypes.StoreKey]),
		accountKeeper,
		bankKeeper,
		stakingKeeper,
		authtypes.FeeCollectorName,
		authtypes.NewModuleAddress(distributiontypes.ModuleName).String(),
	)
	require.NoError(t, distKeeper.Params.Set(ctx, distributiontypes.DefaultParams()))
	require.NoError(t, distKeeper.FeePool.Set(ctx, distributiontypes.InitialFeePool()))
	stakingKeeper.SetHooks(distKeeper.Hooks())

	upgradeKeeper := upgradekeeper.NewKeeper(
		map[int64]bool{},
		runtime.NewKVStoreService(keys[upgradetypes.StoreKey]),
		appCodec,
		tempDir,
		nil,
		authtypes.NewModuleAddress(upgradetypes.ModuleName).String(),
	)

	faucet := wasmKeeper.NewTestFaucet(t, ctx, bankKeeper, minttypes.ModuleName, sdk.NewInt64Coin(lcfg.ChainDenom, 100_000_000_000))

	// set some funds to pay out validators, based on code from:
	// https://github.com/cosmos/cosmos-sdk/blob/fea231556aee4d549d7551a6190389c4328194eb/x/distribution/keeper/keeper_test.go#L50-L57
	distrAcc := distKeeper.GetDistributionAccount(ctx)
	faucet.Fund(ctx, distrAcc.GetAddress(), sdk.NewInt64Coin(lcfg.ChainDenom, 2_000_000))
	accountKeeper.SetModuleAccount(ctx, distrAcc)

	ibcKeeper := ibckeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[ibcexported.StoreKey]),
		subspace(ibcexported.ModuleName),
		upgradeKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	querier := baseapp.NewGRPCQueryRouter()
	querier.SetInterfaceRegistry(encodingConfig.InterfaceRegistry)
	msgRouter := baseapp.NewMsgServiceRouter()
	msgRouter.SetInterfaceRegistry(encodingConfig.InterfaceRegistry)

	//cfg := sdk.GetConfig()
	//cfg.SetAddressVerifier(types.VerifyAddressLen())

	keeper := wasmKeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[wasmtypes.StoreKey]),
		accountKeeper,
		bankKeeper,
		stakingKeeper,
		distributionkeeper.NewQuerier(distKeeper),
		ibcKeeper.ChannelKeeper, // ICS4Wrapper
		ibcKeeper.ChannelKeeper,
		ibcKeeper.ChannelKeeperV2,
		ibcmock.MockIBCTransferKeeper{},
		msgRouter,
		querier,
		tempDir,
		nodeConfig,
		vmConfig,
		availableCapabilities,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		opts...,
	)
	require.NoError(t, keeper.SetParams(ctx, wasmtypes.DefaultParams()))

	// add wasm handler so we can loop-back (contracts calling contracts)
	contractKeeper := wasmKeeper.NewDefaultPermissionKeeper(&keeper)

	govKeeper := govkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[govtypes.StoreKey]),
		accountKeeper,
		bankKeeper,
		stakingKeeper,
		distKeeper,
		msgRouter,
		govtypes.DefaultConfig(),
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	govParams := govtypesv1.DefaultParams()
	govParams.MinDeposit = sdk.NewCoins(sdk.NewCoin(lcfg.ChainDenom, govtypesv1.DefaultMinDepositTokens))
	govParams.ExpeditedMinDeposit = sdk.NewCoins(sdk.NewCoin(lcfg.ChainDenom, govtypesv1.DefaultMinExpeditedDepositTokens))
	require.NoError(t, govKeeper.Params.Set(ctx, govParams))

	am := module.NewManager( // minimal module set that we use for message/ query tests
		bank.NewAppModule(appCodec, bankKeeper, accountKeeper, subspace(banktypes.ModuleName)),
		staking.NewAppModule(appCodec, stakingKeeper, accountKeeper, bankKeeper, subspace(stakingtypes.ModuleName)),
		distribution.NewAppModule(appCodec, distKeeper, accountKeeper, bankKeeper, stakingKeeper, subspace(distributiontypes.ModuleName)),
		gov.NewAppModule(appCodec, govKeeper, accountKeeper, bankKeeper, subspace(govtypes.ModuleName)),
	)
	am.RegisterServices(module.NewConfigurator(appCodec, msgRouter, querier)) //nolint:errcheck
	wasmtypes.RegisterMsgServer(msgRouter, wasmKeeper.NewMsgServerImpl(&keeper))
	wasmtypes.RegisterQueryServer(querier, wasmKeeper.NewGrpcQuerier(appCodec, runtime.NewKVStoreService(keys[wasmtypes.ModuleName]), keeper, keeper.QueryGasLimit()))

	keepers := wasmKeeper.TestKeepers{
		AccountKeeper:  accountKeeper,
		StakingKeeper:  stakingKeeper,
		DistKeeper:     distKeeper,
		ContractKeeper: contractKeeper,
		WasmKeeper:     &keeper,
		BankKeeper:     bankKeeper,
		GovKeeper:      govKeeper,
		IBCKeeper:      ibcKeeper,
		Router:         msgRouter,
		EncodingConfig: encodingConfig,
		Faucet:         faucet,
		MultiStore:     ms,
		WasmStoreKey:   keys[wasmtypes.StoreKey],
	}
	return ctx, keepers
}

var _ wasmKeeper.MessageRouter = MessageRouterFunc(nil)

type MessageRouterFunc func(ctx sdk.Context, req sdk.Msg) (*sdk.Result, error)

func (m MessageRouterFunc) Handler(_ sdk.Msg) baseapp.MsgServiceHandler {
	return m
}

func handleStoreCode(ctx sdk.Context, k wasmtypes.ContractOpsKeeper, msg *wasmtypes.MsgStoreCode) (*sdk.Result, error) {
	senderAddr, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		return nil, errorsmod.Wrap(err, "sender")
	}
	codeID, _, err := k.Create(ctx, senderAddr, msg.WASMByteCode, msg.InstantiatePermission)
	if err != nil {
		return nil, err
	}

	return &sdk.Result{
		Data:   []byte(fmt.Sprintf("%d", codeID)),
		Events: ctx.EventManager().ABCIEvents(),
	}, nil
}

func handleInstantiate(ctx sdk.Context, k wasmtypes.ContractOpsKeeper, msg *wasmtypes.MsgInstantiateContract) (*sdk.Result, error) {
	senderAddr, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		return nil, errorsmod.Wrap(err, "sender")
	}
	var adminAddr sdk.AccAddress
	if msg.Admin != "" {
		if adminAddr, err = sdk.AccAddressFromBech32(msg.Admin); err != nil {
			return nil, errorsmod.Wrap(err, "admin")
		}
	}

	contractAddr, _, err := k.Instantiate(ctx, msg.CodeID, senderAddr, adminAddr, msg.Msg, msg.Label, msg.Funds)
	if err != nil {
		return nil, err
	}

	return &sdk.Result{
		Data:   contractAddr,
		Events: ctx.EventManager().Events().ToABCIEvents(),
	}, nil
}

func handleExecute(ctx sdk.Context, k wasmtypes.ContractOpsKeeper, msg *wasmtypes.MsgExecuteContract) (*sdk.Result, error) {
	senderAddr, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		return nil, errorsmod.Wrap(err, "sender")
	}
	contractAddr, err := sdk.AccAddressFromBech32(msg.Contract)
	if err != nil {
		return nil, errorsmod.Wrap(err, "admin")
	}
	data, err := k.Execute(ctx, contractAddr, senderAddr, msg.Msg, msg.Funds)
	if err != nil {
		return nil, err
	}

	return &sdk.Result{
		Data:   data,
		Events: ctx.EventManager().Events().ToABCIEvents(),
	}, nil
}

func RandomAccountAddress(_ testing.TB) sdk.AccAddress {
	_, addr := keyPubAddr()
	return addr
}

// DeterministicAccountAddress  creates a test address with v repeated to valid address size
func DeterministicAccountAddress(_ testing.TB, v byte) sdk.AccAddress {
	return bytes.Repeat([]byte{v}, sdkaddress.Len)
}

func RandomBech32AccountAddress(t testing.TB) string {
	return RandomAccountAddress(t).String()
}

type ExampleContract struct {
	InitialAmount sdk.Coins
	Creator       crypto.PrivKey
	CreatorAddr   sdk.AccAddress
	CodeID        uint64
	Checksum      []byte
}

func StoreHackatomExampleContract(t testing.TB, ctx sdk.Context, keepers wasmKeeper.TestKeepers) ExampleContract {
	return StoreExampleContractWasm(t, ctx, keepers, testdata.HackatomContractWasm())
}

func StoreBurnerExampleContract(t testing.TB, ctx sdk.Context, keepers wasmKeeper.TestKeepers) ExampleContract {
	return StoreExampleContractWasm(t, ctx, keepers, testdata.BurnerContractWasm())
}

func StoreIBCReflectContract(t testing.TB, ctx sdk.Context, keepers wasmKeeper.TestKeepers) ExampleContract {
	return StoreExampleContractWasm(t, ctx, keepers, testdata.IBCReflectContractWasm())
}

func StoreReflectContract(t testing.TB, ctx sdk.Context, keepers wasmKeeper.TestKeepers) ExampleContract {
	return StoreExampleContractWasm(t, ctx, keepers, testdata.ReflectContractWasm())
}

func StoreExampleContract(t testing.TB, ctx sdk.Context, keepers wasmKeeper.TestKeepers, wasmFile string) ExampleContract {
	wasmCode, err := os.ReadFile(wasmFile)
	require.NoError(t, err)
	return StoreExampleContractWasm(t, ctx, keepers, wasmCode)
}

func StoreExampleContractWasm(t testing.TB, ctx sdk.Context, keepers wasmKeeper.TestKeepers, wasmCode []byte) ExampleContract {
	anyAmount := sdk.NewCoins(sdk.NewInt64Coin("denom", 1000))
	creator, creatorAddr := keyPubAddr()
	fundAccounts(t, ctx, keepers.AccountKeeper, keepers.BankKeeper, creatorAddr, anyAmount)

	codeID, _, err := keepers.ContractKeeper.Create(ctx, creatorAddr, wasmCode, nil)
	require.NoError(t, err)
	hash := keepers.WasmKeeper.GetCodeInfo(ctx, codeID).CodeHash
	return ExampleContract{anyAmount, creator, creatorAddr, codeID, hash}
}

var wasmIdent = []byte("\x00\x61\x73\x6D")

type HackatomExampleInstance struct {
	ExampleContract
	Contract        sdk.AccAddress
	Verifier        crypto.PrivKey
	VerifierAddr    sdk.AccAddress
	Beneficiary     crypto.PrivKey
	BeneficiaryAddr sdk.AccAddress
	Label           string
	Deposit         sdk.Coins
}

// InstantiateHackatomExampleContract load and instantiate the "./testdata/hackatom.wasm" contract
func InstantiateHackatomExampleContract(t testing.TB, ctx sdk.Context, keepers wasmKeeper.TestKeepers) HackatomExampleInstance {
	contract := StoreHackatomExampleContract(t, ctx, keepers)

	verifier, verifierAddr := keyPubAddr()
	fundAccounts(t, ctx, keepers.AccountKeeper, keepers.BankKeeper, verifierAddr, contract.InitialAmount)

	beneficiary, beneficiaryAddr := keyPubAddr()
	initMsgBz, err := json.Marshal(HackatomExampleInitMsg{
		Verifier:    verifierAddr,
		Beneficiary: beneficiaryAddr,
	})
	require.NoError(t, err)
	initialAmount := sdk.NewCoins(sdk.NewInt64Coin("denom", 100))

	adminAddr := contract.CreatorAddr
	label := "hackatom contract"
	contractAddr, _, err := keepers.ContractKeeper.Instantiate(ctx, contract.CodeID, contract.CreatorAddr, adminAddr, initMsgBz, label, initialAmount)
	require.NoError(t, err)
	return HackatomExampleInstance{
		ExampleContract: contract,
		Contract:        contractAddr,
		Verifier:        verifier,
		VerifierAddr:    verifierAddr,
		Beneficiary:     beneficiary,
		BeneficiaryAddr: beneficiaryAddr,
		Label:           label,
		Deposit:         initialAmount,
	}
}

type ExampleInstance struct {
	ExampleContract
	Contract sdk.AccAddress
	Label    string
	Deposit  sdk.Coins
}

type HackatomExampleInitMsg struct {
	Verifier    sdk.AccAddress `json:"verifier"`
	Beneficiary sdk.AccAddress `json:"beneficiary"`
}

func (m HackatomExampleInitMsg) GetBytes(t testing.TB) []byte {
	initMsgBz, err := json.Marshal(m)
	require.NoError(t, err)
	return initMsgBz
}

type IBCReflectExampleInstance struct {
	Contract      sdk.AccAddress
	Admin         sdk.AccAddress
	CodeID        uint64
	ReflectCodeID uint64
}

func (m IBCReflectExampleInstance) GetBytes(t testing.TB) []byte {
	initMsgBz, err := json.Marshal(m)
	require.NoError(t, err)
	return initMsgBz
}

// InstantiateIBCReflectContract load and instantiate the "./testdata/ibc_reflect.wasm" contract
func InstantiateIBCReflectContract(t testing.TB, ctx sdk.Context, keepers wasmKeeper.TestKeepers) IBCReflectExampleInstance {
	reflectID := StoreReflectContract(t, ctx, keepers).CodeID
	ibcReflectID := StoreIBCReflectContract(t, ctx, keepers).CodeID

	initMsgBz, err := json.Marshal(IBCReflectInitMsg{
		ReflectCodeID: reflectID,
	})
	require.NoError(t, err)
	adminAddr := RandomAccountAddress(t)

	contractAddr, _, err := keepers.ContractKeeper.Instantiate(ctx, ibcReflectID, adminAddr, adminAddr, initMsgBz, "ibc-reflect-factory", nil)
	require.NoError(t, err)
	return IBCReflectExampleInstance{
		Admin:         adminAddr,
		Contract:      contractAddr,
		CodeID:        ibcReflectID,
		ReflectCodeID: reflectID,
	}
}

type IBCReflectInitMsg struct {
	ReflectCodeID uint64 `json:"reflect_code_id"`
}

func (m IBCReflectInitMsg) GetBytes(t testing.TB) []byte {
	initMsgBz, err := json.Marshal(m)
	require.NoError(t, err)
	return initMsgBz
}

type BurnerExampleInitMsg struct {
	Payout sdk.AccAddress `json:"payout"`
	Delete uint32         `json:"delete"`
}

func (m BurnerExampleInitMsg) GetBytes(t testing.TB) []byte {
	initMsgBz, err := json.Marshal(m)
	require.NoError(t, err)
	return initMsgBz
}

func fundAccounts(t testing.TB, ctx sdk.Context, am authkeeper.AccountKeeper, bank bankkeeper.Keeper, addr sdk.AccAddress, coins sdk.Coins) {
	acc := am.NewAccountWithAddress(ctx, addr)
	am.SetAccount(ctx, acc)
	wasmKeeper.NewTestFaucet(t, ctx, bank, minttypes.ModuleName, coins...).Fund(ctx, addr, coins...)
}

var keyCounter uint64

// we need to make this deterministic (same every test run), as encoded address size and thus gas cost,
// depends on the actual bytes (due to ugly CanonicalAddress encoding)
func keyPubAddr() (crypto.PrivKey, sdk.AccAddress) {
	keyCounter++
	seed := make([]byte, 8)
	binary.BigEndian.PutUint64(seed, keyCounter)

	key := ed25519.GenPrivKeyFromSecret(seed)
	pub := key.PubKey()
	addr := sdk.AccAddress(pub.Address())
	return key, addr
}
