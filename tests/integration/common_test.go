package integration

import (
	"encoding/json"
	"testing"
	"time"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	"cosmossdk.io/x/evidence"
	"cosmossdk.io/x/feegrant"
	"cosmossdk.io/x/upgrade"

	"github.com/CosmWasm/wasmd/x/wasm/keeper/wasmtesting"
	"github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	secp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	"github.com/cosmos/cosmos-sdk/x/distribution"
	"github.com/cosmos/cosmos-sdk/x/gov"
	"github.com/cosmos/cosmos-sdk/x/mint"
	"github.com/cosmos/cosmos-sdk/x/params"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	"github.com/cosmos/cosmos-sdk/x/staking"
	"github.com/cosmos/ibc-go/modules/capability"
	"github.com/cosmos/ibc-go/v8/modules/apps/transfer"
	"github.com/stretchr/testify/require"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	evidencetypes "cosmossdk.io/x/evidence/types"
	upgradekeeper "cosmossdk.io/x/upgrade/keeper"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	wasmKeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	distributionkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govclient "github.com/cosmos/cosmos-sdk/x/gov/client"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	paramsclient "github.com/cosmos/cosmos-sdk/x/params/client"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	ibc "github.com/cosmos/ibc-go/v8/modules/core"
	ibcexported "github.com/cosmos/ibc-go/v8/modules/core/exported"
	ibckeeper "github.com/cosmos/ibc-go/v8/modules/core/keeper"
)

const pastelBech32MainPrefix = "pastel"

// ensure store code returns the expected response
func assertStoreCodeResponse(t *testing.T, data []byte, expected uint64) {
	var pStoreResp types.MsgStoreCodeResponse
	require.NoError(t, pStoreResp.Unmarshal(data))
	require.Equal(t, pStoreResp.CodeID, expected)
}

// ensure execution returns the expected data
func assertExecuteResponse(t *testing.T, data, expected []byte) {
	var pExecResp types.MsgExecuteContractResponse
	require.NoError(t, pExecResp.Unmarshal(data))
	require.Equal(t, pExecResp.Data, expected)
}

// ensures this returns a valid bech32 address and returns it
func parseInitResponse(t *testing.T, data []byte) string {
	var pInstResp types.MsgInstantiateContractResponse
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
	payout := sdk.DecCoins{{Denom: "stake", Amount: amount}}
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
			return nil, errorsmod.Wrapf(types.ErrInvalidMsg, "Custom Debug: %s", custom.Debug)
		}
		return nil, errorsmod.Wrap(types.ErrInvalidMsg, "Unknown Custom message variant")
	}
}

var moduleBasics = module.NewBasicManager(
	auth.AppModuleBasic{},
	bank.AppModuleBasic{},
	capability.AppModuleBasic{},
	staking.AppModuleBasic{},
	mint.AppModuleBasic{},
	distribution.AppModuleBasic{},
	gov.NewAppModuleBasic([]govclient.ProposalHandler{
		paramsclient.ProposalHandler,
	}),
	params.AppModuleBasic{},
	crisis.AppModuleBasic{},
	slashing.AppModuleBasic{},
	ibc.AppModuleBasic{},
	upgrade.AppModuleBasic{},
	evidence.AppModuleBasic{},
	transfer.AppModuleBasic{},
	vesting.AppModuleBasic{},
)

func MakeEncodingConfig(_ testing.TB) moduletestutil.TestEncodingConfig {
	encodingConfig := moduletestutil.MakeTestEncodingConfig(
		auth.AppModule{},
		bank.AppModule{},
		staking.AppModule{},
		mint.AppModule{},
		slashing.AppModule{},
		gov.AppModule{},
		crisis.AppModule{},
		ibc.AppModule{},
		transfer.AppModule{},
		vesting.AppModule{},
	)
	amino := encodingConfig.Amino
	interfaceRegistry := encodingConfig.InterfaceRegistry

	std.RegisterInterfaces(interfaceRegistry)

	moduleBasics.RegisterInterfaces(interfaceRegistry)
	// add wasmd types
	types.RegisterInterfaces(interfaceRegistry)
	types.RegisterLegacyAminoCodec(amino)

	return encodingConfig
}

// CreateDefaultTestInput common settings for CreateTestInput
func CreateDefaultTestInput(t testing.TB) (sdk.Context, wasmKeeper.TestKeepers) {
	return CreateTestInput(t, false, []string{"staking"})
}

// CreateTestInput encoders can be nil to accept the defaults, or set it to override some of the message handlers (like default)
func CreateTestInput(t testing.TB, isCheckTx bool, availableCapabilities []string, opts ...wasmKeeper.Option) (sdk.Context, wasmKeeper.TestKeepers) {
	// Load default wasm config
	return createTestInput(t, isCheckTx, availableCapabilities, types.DefaultWasmConfig(), dbm.NewMemDB(), opts...)
}

// encoders can be nil to accept the defaults, or set it to override some of the message handlers (like default)
func createTestInput(
	t testing.TB,
	isCheckTx bool,
	availableCapabilities []string,
	wasmConfig types.WasmConfig,
	db dbm.DB,
	opts ...wasmKeeper.Option,
) (sdk.Context, wasmKeeper.TestKeepers) {
	tempDir := t.TempDir()

	keys := storetypes.NewKVStoreKeys(
		authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey,
		minttypes.StoreKey, distributiontypes.StoreKey, slashingtypes.StoreKey,
		govtypes.StoreKey, paramstypes.StoreKey, ibcexported.StoreKey, upgradetypes.StoreKey,
		evidencetypes.StoreKey, ibctransfertypes.StoreKey,
		capabilitytypes.StoreKey, feegrant.StoreKey, authzkeeper.StoreKey,
		types.StoreKey,
	)
	logger := log.NewTestLogger(t)
	ms := store.NewCommitMultiStore(db, logger, storemetrics.NewNoOpMetrics())
	for _, v := range keys {
		ms.MountStoreWithDB(v, storetypes.StoreTypeIAVL, db)
	}
	tkeys := storetypes.NewTransientStoreKeys(paramstypes.TStoreKey)
	for _, v := range tkeys {
		ms.MountStoreWithDB(v, storetypes.StoreTypeTransient, db)
	}

	memKeys := storetypes.NewMemoryStoreKeys(capabilitytypes.MemStoreKey)
	for _, v := range memKeys {
		ms.MountStoreWithDB(v, storetypes.StoreTypeMemory, db)
	}

	require.NoError(t, ms.LoadLatestVersion())

	ctx := sdk.NewContext(ms, tmproto.Header{
		Height: 1234567,
		Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
	}, isCheckTx, log.NewNopLogger())
	ctx = types.WithTXCounter(ctx, 0)

	encodingConfig := wasmKeeper.MakeEncodingConfig(t)
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
		crisistypes.ModuleName,
		ibctransfertypes.ModuleName,
		capabilitytypes.ModuleName,
		ibcexported.ModuleName,
		govtypes.ModuleName,
		types.ModuleName,
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
		types.ModuleName:               {authtypes.Burner},
	}

	accountKeeper := authkeeper.NewAccountKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[authtypes.StoreKey]),
		authtypes.ProtoBaseAccount,
		maccPerms,
		authcodec.NewBech32Codec(pastelBech32MainPrefix),
		pastelBech32MainPrefix,
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
	stakingtypes.DefaultParams()
	require.NoError(t, stakingKeeper.SetParams(ctx, wasmKeeper.TestingStakeParams))

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

	faucet := wasmKeeper.NewTestFaucet(t, ctx, bankKeeper, minttypes.ModuleName, sdk.NewCoin("stake", sdkmath.NewInt(100_000_000_000)))

	// set some funds ot pay out validatores, based on code from:
	// https://github.com/cosmos/cosmos-sdk/blob/fea231556aee4d549d7551a6190389c4328194eb/x/distribution/keeper/keeper_test.go#L50-L57
	distrAcc := distKeeper.GetDistributionAccount(ctx)
	faucet.Fund(ctx, distrAcc.GetAddress(), sdk.NewCoin("stake", sdkmath.NewInt(2000000)))
	accountKeeper.SetModuleAccount(ctx, distrAcc)

	capabilityKeeper := capabilitykeeper.NewKeeper(
		appCodec,
		keys[capabilitytypes.StoreKey],
		memKeys[capabilitytypes.MemStoreKey],
	)
	scopedIBCKeeper := capabilityKeeper.ScopeToModule(ibcexported.ModuleName)
	scopedWasmKeeper := capabilityKeeper.ScopeToModule(types.ModuleName)

	ibcKeeper := ibckeeper.NewKeeper(
		appCodec,
		keys[ibcexported.StoreKey],
		subspace(ibcexported.ModuleName),
		stakingKeeper,
		upgradeKeeper,
		scopedIBCKeeper,
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
		runtime.NewKVStoreService(keys[types.StoreKey]),
		accountKeeper,
		bankKeeper,
		stakingKeeper,
		distributionkeeper.NewQuerier(distKeeper),
		ibcKeeper.ChannelKeeper, // ICS4Wrapper
		ibcKeeper.ChannelKeeper,
		ibcKeeper.PortKeeper,
		scopedWasmKeeper,
		wasmtesting.MockIBCTransferKeeper{},
		msgRouter,
		querier,
		tempDir,
		wasmConfig,
		availableCapabilities,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		opts...,
	)
	require.NoError(t, keeper.SetParams(ctx, types.DefaultParams()))

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
	require.NoError(t, govKeeper.Params.Set(ctx, govv1.DefaultParams()))

	am := module.NewManager( // minimal module set that we use for message/ query tests
		bank.NewAppModule(appCodec, bankKeeper, accountKeeper, subspace(banktypes.ModuleName)),
		staking.NewAppModule(appCodec, stakingKeeper, accountKeeper, bankKeeper, subspace(stakingtypes.ModuleName)),
		distribution.NewAppModule(appCodec, distKeeper, accountKeeper, bankKeeper, stakingKeeper, subspace(distributiontypes.ModuleName)),
		gov.NewAppModule(appCodec, govKeeper, accountKeeper, bankKeeper, subspace(govtypes.ModuleName)),
	)
	am.RegisterServices(module.NewConfigurator(appCodec, msgRouter, querier)) //nolint:errcheck
	types.RegisterMsgServer(msgRouter, wasmKeeper.NewMsgServerImpl(&keeper))
	types.RegisterQueryServer(querier, wasmKeeper.NewGrpcQuerier(appCodec, runtime.NewKVStoreService(keys[types.ModuleName]), keeper, keeper.QueryGasLimit()))

	keepers := wasmKeeper.TestKeepers{
		AccountKeeper:    accountKeeper,
		StakingKeeper:    stakingKeeper,
		DistKeeper:       distKeeper,
		ContractKeeper:   contractKeeper,
		WasmKeeper:       &keeper,
		BankKeeper:       bankKeeper,
		GovKeeper:        govKeeper,
		IBCKeeper:        ibcKeeper,
		Router:           msgRouter,
		EncodingConfig:   encodingConfig,
		Faucet:           faucet,
		MultiStore:       ms,
		ScopedWasmKeeper: scopedWasmKeeper,
		WasmStoreKey:     keys[types.StoreKey],
	}
	return ctx, keepers
}
