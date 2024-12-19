package keeper_test

import (
	"testing"
	"time"

	"cosmossdk.io/math"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/pastelnetwork/pastel/x/supernode/keeper"
	supernodemocks "github.com/pastelnetwork/pastel/x/supernode/mocks"
	"github.com/pastelnetwork/pastel/x/supernode/types"
)

func TestMsgServer_RegisterSupernode(t *testing.T) {
	valAddr := sdk.ValAddress([]byte("validator"))
	creatorAddr := sdk.AccAddress(valAddr)

	otherValAddr := sdk.ValAddress([]byte("other-validator"))
	otherCreatorAddr := sdk.AccAddress(otherValAddr)

	testCases := []struct {
		name          string
		msg           *types.MsgRegisterSupernode
		mockSetup     func(*supernodemocks.MockStakingKeeper, *supernodemocks.MockSlashingKeeper, *supernodemocks.MockBankKeeper)
		expectedError error
	}{
		{
			name: "successful registration",
			msg: &types.MsgRegisterSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				IpAddress:        "192.168.1.1",
				Version:          "1.0.0",
			},
			mockSetup: func(sk *supernodemocks.MockStakingKeeper, slk *supernodemocks.MockSlashingKeeper, bk *supernodemocks.MockBankKeeper) {
				sk.EXPECT().
					Validator(gomock.Any(), valAddr).
					Return(&stakingtypes.Validator{
						OperatorAddress: valAddr.String(),
						Status:          stakingtypes.Bonded,
						Tokens:          math.NewInt(2000000),
						Jailed:          false,
					}, nil)
			},
			expectedError: nil,
		},
		{
			name: "invalid validator address",
			msg: &types.MsgRegisterSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: "invalid",
				IpAddress:        "192.168.1.1",
				Version:          "1.0.0",
			},
			expectedError: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "unauthorized",
			msg: &types.MsgRegisterSupernode{
				Creator:          otherCreatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				IpAddress:        "192.168.1.1",
				Version:          "1.0.0",
			},
			mockSetup: func(sk *supernodemocks.MockStakingKeeper, slk *supernodemocks.MockSlashingKeeper, bk *supernodemocks.MockBankKeeper) {
				sk.EXPECT().
					Validator(gomock.Any(), valAddr).
					Return(&stakingtypes.Validator{
						OperatorAddress: valAddr.String(),
						Status:          stakingtypes.Bonded,
						Tokens:          math.NewInt(2000000),
						Jailed:          false,
					}, nil)
			},
			expectedError: sdkerrors.ErrUnauthorized,
		},
		{
			name: "empty ip address",
			msg: &types.MsgRegisterSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				IpAddress:        "",
				Version:          "1.0.0",
			},
			mockSetup: func(sk *supernodemocks.MockStakingKeeper, slk *supernodemocks.MockSlashingKeeper, bk *supernodemocks.MockBankKeeper) {
				sk.EXPECT().
					Validator(gomock.Any(), valAddr).
					Return(&stakingtypes.Validator{
						OperatorAddress: valAddr.String(),
						Status:          stakingtypes.Bonded,
						Tokens:          math.NewInt(2000000),
						Jailed:          false,
					}, nil)
			},
			expectedError: types.ErrEmptyIPAddress,
		},
		{
			name: "validator not found",
			msg: &types.MsgRegisterSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				IpAddress:        "192.168.1.1",
				Version:          "1.0.0",
			},
			mockSetup: func(sk *supernodemocks.MockStakingKeeper, slk *supernodemocks.MockSlashingKeeper, bk *supernodemocks.MockBankKeeper) {
				sk.EXPECT().
					Validator(gomock.Any(), valAddr).
					Return(nil, sdkerrors.ErrNotFound)
			},
			expectedError: sdkerrors.ErrNotFound,
		},
		{
			name: "jailed validator",
			msg: &types.MsgRegisterSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				IpAddress:        "192.168.1.1",
				Version:          "1.0.0",
			},
			mockSetup: func(sk *supernodemocks.MockStakingKeeper, slk *supernodemocks.MockSlashingKeeper, bk *supernodemocks.MockBankKeeper) {
				sk.EXPECT().
					Validator(gomock.Any(), valAddr).
					Return(&stakingtypes.Validator{
						OperatorAddress: valAddr.String(),
						Status:          stakingtypes.Bonded,
						Tokens:          math.NewInt(2000000),
						Jailed:          true,
					}, nil)
			},
			expectedError: sdkerrors.ErrInvalidRequest,
		},
		{
			name: "validator not bonded and insufficient stake",
			msg: &types.MsgRegisterSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				IpAddress:        "192.168.1.1",
				Version:          "1.0.0",
			},
			mockSetup: func(sk *supernodemocks.MockStakingKeeper, slk *supernodemocks.MockSlashingKeeper, bk *supernodemocks.MockBankKeeper) {
				sk.EXPECT().
					Validator(gomock.Any(), valAddr).
					Return(&stakingtypes.Validator{
						OperatorAddress: valAddr.String(),
						Status:          stakingtypes.Unbonded, // not bonded
						Tokens:          math.NewInt(500000),   // less than 1,000,000 required
						Jailed:          false,
					}, nil)
			},
			expectedError: sdkerrors.ErrInvalidRequest,
		},
		{
			name: "validator not bonded but sufficient stake",
			msg: &types.MsgRegisterSupernode{
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				IpAddress:        "192.168.1.1",
				Version:          "1.0.0",
			},
			mockSetup: func(sk *supernodemocks.MockStakingKeeper, slk *supernodemocks.MockSlashingKeeper, bk *supernodemocks.MockBankKeeper) {
				sk.EXPECT().
					Validator(gomock.Any(), valAddr).
					Return(&stakingtypes.Validator{
						OperatorAddress: valAddr.String(),
						Status:          stakingtypes.Unbonded, // not bonded
						Tokens:          math.NewInt(2000000),  // meets requirement
						Jailed:          false,
					}, nil)
			},
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			stakingKeeper := supernodemocks.NewMockStakingKeeper(ctrl)
			slashingKeeper := supernodemocks.NewMockSlashingKeeper(ctrl)
			bankKeeper := supernodemocks.NewMockBankKeeper(ctrl)

			if tc.mockSetup != nil {
				tc.mockSetup(stakingKeeper, slashingKeeper, bankKeeper)
			}

			k, ctx := setupKeeperForTest(t, stakingKeeper, slashingKeeper, bankKeeper)
			msgServer := keeper.NewMsgServerImpl(k)

			_, err := msgServer.RegisterSupernode(sdk.WrapSDKContext(ctx), tc.msg)
			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func setupKeeperForTest(t testing.TB, stakingKeeper types.StakingKeeper, slashingKeeper types.SlashingKeeper,
	bankKeeper types.BankKeeper) (keeper.Keeper, sdk.Context) {

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	authority := authtypes.NewModuleAddress(govtypes.ModuleName).String()

	k := keeper.NewKeeper(
		cdc,
		runtime.NewKVStoreService(storeKey),
		log.NewNopLogger(),
		authority,
		bankKeeper,
		stakingKeeper,
		slashingKeeper,
	)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{}, false, log.NewNopLogger())
	ctx = ctx.WithBlockTime(time.Now())

	// Set default params
	params := types.DefaultParams()
	params.MinimumStakeForSn = 1000000
	err := k.SetParams(ctx, params)
	require.NoError(t, err)

	return k, ctx
}
