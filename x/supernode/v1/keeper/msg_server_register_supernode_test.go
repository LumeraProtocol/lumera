package keeper_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	keeper2 "github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	supernodemocks "github.com/LumeraProtocol/lumera/x/supernode/v1/mocks"
	types2 "github.com/LumeraProtocol/lumera/x/supernode/v1/types"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestMsgServer_RegisterSupernode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	valAddr := sdk.ValAddress([]byte("validator"))
	creatorAddr := sdk.AccAddress(valAddr)

	otherValAddr := sdk.ValAddress([]byte("other-validator"))
	otherCreatorAddr := sdk.AccAddress(otherValAddr)

	testCases := []struct {
		name          string
		msg           *types2.MsgRegisterSupernode
		mockSetup     func(sk *supernodemocks.MockStakingKeeper, slk *supernodemocks.MockSlashingKeeper, bk *supernodemocks.MockBankKeeper)
		expectedError error
	}{
		{
			name: "successful registration (bonded validator -> skip checks)",
			msg: &types2.MsgRegisterSupernode{
				SupernodeAccount: creatorAddr.String(),
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				IpAddress:        "192.168.1.1",
				P2PPort:          "26657",
			},
			mockSetup: func(sk *supernodemocks.MockStakingKeeper, slk *supernodemocks.MockSlashingKeeper, bk *supernodemocks.MockBankKeeper) {
				// Return a bonded validator => no min-stake check needed
				sk.EXPECT().
					Validator(gomock.Any(), valAddr).
					Return(&stakingtypes.Validator{
						OperatorAddress: valAddr.String(),
						Status:          stakingtypes.Bonded, // bonded => skip
						Tokens:          math.NewInt(2_000_000),
						DelegatorShares: math.LegacyNewDec(2_000_000), // typically matches tokens for ratio=1
						Jailed:          false,
					}, nil)
			},
			expectedError: nil,
		},
		{
			name: "invalid validator address",
			msg: &types2.MsgRegisterSupernode{
				SupernodeAccount: creatorAddr.String(),
				Creator:          creatorAddr.String(),
				ValidatorAddress: "invalid", // not bech32 => error
				IpAddress:        "192.168.1.1",
			},
			// no mock setup needed => fails earlier
			expectedError: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "unauthorized => msg.Creator != validator operator address",
			msg: &types2.MsgRegisterSupernode{
				SupernodeAccount: creatorAddr.String(),
				Creator:          otherCreatorAddr.String(), // different from valAddr
				ValidatorAddress: valAddr.String(),
				IpAddress:        "192.168.1.1",
			},
			mockSetup: func(sk *supernodemocks.MockStakingKeeper, slk *supernodemocks.MockSlashingKeeper, bk *supernodemocks.MockBankKeeper) {
				// No expectations here, because the code immediately returns unauthorized
				// before calling sk.Validator(...)
			},
			expectedError: sdkerrors.ErrUnauthorized,
		},
		{
			name: "empty ip address => error from supernode.Validate()",
			msg: &types2.MsgRegisterSupernode{
				SupernodeAccount: creatorAddr.String(),
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				IpAddress:        "", // triggers types.ErrEmptyIPAddress
			},
			mockSetup: func(sk *supernodemocks.MockStakingKeeper, slk *supernodemocks.MockSlashingKeeper, bk *supernodemocks.MockBankKeeper) {
				sk.EXPECT().
					Validator(gomock.Any(), valAddr).
					Return(&stakingtypes.Validator{
						OperatorAddress: valAddr.String(),
						Status:          stakingtypes.Bonded,
						Tokens:          math.NewInt(2_000_000),
						DelegatorShares: math.LegacyNewDec(2_000_000),
						Jailed:          false,
					}, nil)
			},
			expectedError: types2.ErrEmptyIPAddress,
		},
		{
			name: "validator not found",
			msg: &types2.MsgRegisterSupernode{
				SupernodeAccount: creatorAddr.String(),
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				IpAddress:        "192.168.1.1",
			},
			mockSetup: func(sk *supernodemocks.MockStakingKeeper, slk *supernodemocks.MockSlashingKeeper, bk *supernodemocks.MockBankKeeper) {
				sk.EXPECT().
					Validator(gomock.Any(), valAddr).
					Return(nil, sdkerrors.ErrNotFound)
			},
			expectedError: sdkerrors.ErrNotFound,
		},
		{
			name: "jailed validator => error",
			msg: &types2.MsgRegisterSupernode{
				SupernodeAccount: creatorAddr.String(),
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				IpAddress:        "192.168.1.1",
			},
			mockSetup: func(sk *supernodemocks.MockStakingKeeper, slk *supernodemocks.MockSlashingKeeper, bk *supernodemocks.MockBankKeeper) {
				sk.EXPECT().
					Validator(gomock.Any(), valAddr).
					Return(&stakingtypes.Validator{
						OperatorAddress: valAddr.String(),
						Status:          stakingtypes.Bonded,
						Tokens:          math.NewInt(2_000_000),
						DelegatorShares: math.LegacyNewDec(2_000_000),
						Jailed:          true, // triggers error
					}, nil)
			},
			expectedError: sdkerrors.ErrInvalidRequest,
		},
		{
			name: "validator unbonded, zero delegator shares => immediate error (no self-stake)",
			msg: &types2.MsgRegisterSupernode{
				SupernodeAccount: creatorAddr.String(),
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				IpAddress:        "192.168.1.1",
			},
			mockSetup: func(sk *supernodemocks.MockStakingKeeper, slk *supernodemocks.MockSlashingKeeper, bk *supernodemocks.MockBankKeeper) {
				// We'll set unbonded, delegatorShares=0 => triggers the new check
				sk.EXPECT().
					Validator(gomock.Any(), valAddr).
					Return(&stakingtypes.Validator{
						OperatorAddress: valAddr.String(),
						Status:          stakingtypes.Unbonded,
						Tokens:          math.NewInt(500_000),
						DelegatorShares: math.LegacyNewDec(0), // zero => error
						Jailed:          false,
					}, nil)
			},
			// Because DelegatorShares=0 => "has zero delegator shares" error
			expectedError: sdkerrors.ErrInvalidRequest,
		},
		{
			name: "validator unbonded and insufficient stake => fails eligibility check",
			msg: &types2.MsgRegisterSupernode{
				SupernodeAccount: creatorAddr.String(),
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				IpAddress:        "192.168.1.1",
			},
			mockSetup: func(sk *supernodemocks.MockStakingKeeper, slk *supernodemocks.MockSlashingKeeper, bk *supernodemocks.MockBankKeeper) {
				sk.EXPECT().
					Validator(gomock.Any(), valAddr).
					Return(&stakingtypes.Validator{
						OperatorAddress: valAddr.String(),
						Status:          stakingtypes.Unbonded,
						Tokens:          math.NewInt(50_000),        // below 1,000,000
						DelegatorShares: math.LegacyNewDec(500_000), // match tokens => ratio=1
						Jailed:          false,
					}, nil)
			},
			expectedError: sdkerrors.ErrInvalidRequest,
		},
		{
			name: "validator unbonded but sufficient stake => no error",
			msg: &types2.MsgRegisterSupernode{
				SupernodeAccount: creatorAddr.String(),
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				IpAddress:        "192.168.1.1",
				P2PPort:          "26657",
			},
			mockSetup: func(sk *supernodemocks.MockStakingKeeper, slk *supernodemocks.MockSlashingKeeper, bk *supernodemocks.MockBankKeeper) {
				sk.EXPECT().
					Validator(gomock.Any(), valAddr).
					Return(&stakingtypes.Validator{
						OperatorAddress: valAddr.String(),
						Status:          stakingtypes.Unbonded,
						Tokens:          math.NewInt(2_000_000),
						DelegatorShares: math.LegacyNewDec(2_000_000), // ratio=1
						Jailed:          false,
					}, nil)
			},
			expectedError: nil,
		},
		{
			name: "re-registration of disabled supernode",
			msg: &types2.MsgRegisterSupernode{
				SupernodeAccount: creatorAddr.String(),
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				IpAddress:        "192.168.1.2",
				P2PPort:          "26658",
			},
			mockSetup: func(sk *supernodemocks.MockStakingKeeper, slk *supernodemocks.MockSlashingKeeper, bk *supernodemocks.MockBankKeeper) {
				sk.EXPECT().
					Validator(gomock.Any(), valAddr).
					Return(&stakingtypes.Validator{
						OperatorAddress: valAddr.String(),
						Status:          stakingtypes.Bonded,
						Tokens:          math.NewInt(2_000_000),
						DelegatorShares: math.LegacyNewDec(2_000_000),
						Jailed:          false,
					}, nil)
			},
			expectedError: nil,
		},
		{
			name: "cannot register already active supernode",
			msg: &types2.MsgRegisterSupernode{
				SupernodeAccount: creatorAddr.String(),
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				IpAddress:        "192.168.1.2",
				P2PPort:          "26658",
			},
			mockSetup: func(sk *supernodemocks.MockStakingKeeper, slk *supernodemocks.MockSlashingKeeper, bk *supernodemocks.MockBankKeeper) {
			},
			expectedError: sdkerrors.ErrInvalidRequest,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// fresh controller for each sub-test
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			stakingKeeper := supernodemocks.NewMockStakingKeeper(ctrl)
			slashingKeeper := supernodemocks.NewMockSlashingKeeper(ctrl)
			bankKeeper := supernodemocks.NewMockBankKeeper(ctrl)

			stakingKeeper.EXPECT().
				Delegation(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) (stakingtypes.Delegation, bool) {
					return stakingtypes.Delegation{
						DelegatorAddress: delAddr.String(),
						ValidatorAddress: valAddr.String(),
						// Return 2,000,000 shares so we get 2,000,000 tokens
						Shares: math.LegacyNewDec(2_000_000),
					}, true
				}).
				AnyTimes()

			// If there's a mockSetup, run it
			if tc.mockSetup != nil {
				tc.mockSetup(stakingKeeper, slashingKeeper, bankKeeper)
			}

			k, sdkCtx := setupKeeperForTest(t, stakingKeeper, slashingKeeper, bankKeeper)
			msgServer := keeper2.NewMsgServerImpl(k)

			// Pre-setup for specific test cases
			if tc.name == "re-registration of disabled supernode" {
				// Create a disabled supernode
				disabledSupernode := types2.SuperNode{
					ValidatorAddress: valAddr.String(),
					SupernodeAccount: creatorAddr.String(),
					States: []*types2.SuperNodeStateRecord{
						{
							State:  types2.SuperNodeStateActive,
							Height: 100,
						},
						{
							State:  types2.SuperNodeStateDisabled,
							Height: 200,
						},
					},
					PrevIpAddresses: []*types2.IPAddressHistory{
						{
							Address: "192.168.1.1",
							Height:  100,
						},
					},
					PrevSupernodeAccounts: []*types2.SupernodeAccountHistory{
						{
							Account: creatorAddr.String(),
							Height:  100,
						},
					},
					P2PPort: "26657",
				}
				err := k.SetSuperNode(sdkCtx, disabledSupernode)
				require.NoError(t, err)
			}

			if tc.name == "cannot register already active supernode" {
				// Create an active supernode
				activeSupernode := types2.SuperNode{
					ValidatorAddress: valAddr.String(),
					SupernodeAccount: creatorAddr.String(),
					States: []*types2.SuperNodeStateRecord{
						{
							State:  types2.SuperNodeStateActive,
							Height: 100,
						},
					},
					PrevIpAddresses: []*types2.IPAddressHistory{
						{
							Address: "192.168.1.1",
							Height:  100,
						},
					},
					PrevSupernodeAccounts: []*types2.SupernodeAccountHistory{
						{
							Account: creatorAddr.String(),
							Height:  100,
						},
					},
					P2PPort: "26657",
				}
				err := k.SetSuperNode(sdkCtx, activeSupernode)
				require.NoError(t, err)
			}

			// Execute
			_, err := msgServer.RegisterSupernode(sdkCtx, tc.msg)

			// Assert
			if tc.expectedError != nil {
				if err != nil {
					fmt.Println("get this err: ", err.Error())
				} else {
					fmt.Println("get this err: ", "nil")
				}
				require.ErrorIs(t, err, tc.expectedError)
			} else {
				require.NoError(t, err)
				
				// Additional assertions for re-registration test
				if tc.name == "re-registration of disabled supernode" {
					// Verify the supernode is now active
					sn, found := k.QuerySuperNode(sdkCtx, valAddr)
					require.True(t, found)
					require.Len(t, sn.States, 3) // Initial active, disabled, then active again
					require.Equal(t, types2.SuperNodeStateActive, sn.States[2].State)
					
					// Verify IP address and account were NOT updated
					require.Equal(t, "192.168.1.1", sn.PrevIpAddresses[len(sn.PrevIpAddresses)-1].Address)
					require.Equal(t, creatorAddr.String(), sn.SupernodeAccount)
					require.Len(t, sn.PrevIpAddresses, 1) // No new IP history
					require.Len(t, sn.PrevSupernodeAccounts, 1) // No new account history
				}
			}
		})
	}
}

// setupKeeperForTest is your existing function
func setupKeeperForTest(
	t testing.TB,
	stakingKeeper types2.StakingKeeper,
	slashingKeeper types2.SlashingKeeper,
	bankKeeper types2.BankKeeper,
) (keeper2.Keeper, sdk.Context) {
	storeKey := storetypes.NewKVStoreKey(types2.StoreKey)
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	authority := authtypes.NewModuleAddress(govtypes.ModuleName).String()

	k := keeper2.NewKeeper(
		cdc,
		runtime.NewKVStoreService(storeKey),
		log.NewNopLogger(),
		authority,
		bankKeeper,
		stakingKeeper,
		slashingKeeper,
	)

	sdkCtx := sdk.NewContext(stateStore, cmtproto.Header{}, false, log.NewNopLogger())
	sdkCtx = sdkCtx.WithBlockTime(time.Now())

	// Set default params => min self-stake = 1,000,000
	params := types2.DefaultParams()
	params.MinimumStakeForSn = sdk.NewInt64Coin("ulume", 1_000_000)
	err := k.SetParams(sdkCtx, params)
	require.NoError(t, err)

	return k, sdkCtx
}
