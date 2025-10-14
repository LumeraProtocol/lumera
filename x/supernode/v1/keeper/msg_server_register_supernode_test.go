package keeper_test

import (
	"context"
	"fmt"
	"testing"
	"time"

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

	"github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	supernodemocks "github.com/LumeraProtocol/lumera/x/supernode/v1/mocks"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
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
		msg           *types.MsgRegisterSupernode
		mockSetup     func(sk *supernodemocks.MockStakingKeeper, slk *supernodemocks.MockSlashingKeeper, bk *supernodemocks.MockBankKeeper)
		expectedError error
	}{
		{
			name: "successful registration (bonded validator -> skip checks)",
			msg: &types.MsgRegisterSupernode{
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
			msg: &types.MsgRegisterSupernode{
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
			msg: &types.MsgRegisterSupernode{
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
			msg: &types.MsgRegisterSupernode{
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
			expectedError: types.ErrEmptyIPAddress,
		},
		{
			name: "validator not found",
			msg: &types.MsgRegisterSupernode{
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
			msg: &types.MsgRegisterSupernode{
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
			msg: &types.MsgRegisterSupernode{
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
			msg: &types.MsgRegisterSupernode{
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
			msg: &types.MsgRegisterSupernode{
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
			msg: &types.MsgRegisterSupernode{
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
			msg: &types.MsgRegisterSupernode{
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
		{
			name: "cannot re-register STOPPED supernode",
			msg: &types.MsgRegisterSupernode{
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
		{
			name: "cannot re-register PENALIZED supernode",
			msg: &types.MsgRegisterSupernode{
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
		{
			name: "re-registration ignores new parameters (IP, account, port)",
			msg: &types.MsgRegisterSupernode{
				SupernodeAccount: otherCreatorAddr.String(), // Different account - should be ignored
				Creator:          creatorAddr.String(),
				ValidatorAddress: valAddr.String(),
				IpAddress:        "10.0.0.1", // Different IP - should be ignored
				P2PPort:          "9999",     // Different port - should be ignored
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
			name: "re-registration fails when validator becomes jailed",
			msg: &types.MsgRegisterSupernode{
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
						Jailed:          true, // Validator became jailed
					}, nil)
			},
			expectedError: sdkerrors.ErrInvalidRequest,
		},
		{
			name: "re-registration fails when validator loses eligibility",
			msg: &types.MsgRegisterSupernode{
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
						Status:          stakingtypes.Unbonded,
						Tokens:          math.NewInt(500_000), // Below minimum stake
						DelegatorShares: math.LegacyNewDec(500_000),
						Jailed:          false,
					}, nil)

				// Set up delegation mock to return insufficient stake for the existing account
				// This will be called for both the validator operator account and the supernode account
				sk.EXPECT().
					Delegation(gomock.Any(), gomock.Any(), valAddr).
					DoAndReturn(func(_ context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) (stakingtypes.Delegation, bool) {
						return stakingtypes.Delegation{
							DelegatorAddress: delAddr.String(),
							ValidatorAddress: valAddr.String(),
							Shares:           math.LegacyNewDec(250_000), // Below minimum (1,000,000)
						}, true
					}).
					AnyTimes()
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

			// Set up default delegation mock - can be overridden by specific test setup
			if tc.name != "re-registration fails when validator loses eligibility" {
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
			}

			// If there's a mockSetup, run it
			if tc.mockSetup != nil {
				tc.mockSetup(stakingKeeper, slashingKeeper, bankKeeper)
			}

			k, sdkCtx := setupKeeperForTest(t, stakingKeeper, slashingKeeper, bankKeeper)
			msgServer := keeper.NewMsgServerImpl(k)

			// Pre-setup for specific test cases
			if tc.name == "re-registration of disabled supernode" {
				// Create a disabled supernode
				disabledSupernode := types.SuperNode{
					ValidatorAddress: valAddr.String(),
					SupernodeAccount: creatorAddr.String(),
					States: []*types.SuperNodeStateRecord{
						{
							State:  types.SuperNodeStateActive,
							Height: 100,
						},
						{
							State:  types.SuperNodeStateDisabled,
							Height: 200,
						},
					},
					PrevIpAddresses: []*types.IPAddressHistory{
						{
							Address: "192.168.1.1",
							Height:  100,
						},
					},
					PrevSupernodeAccounts: []*types.SupernodeAccountHistory{
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
				activeSupernode := types.SuperNode{
					ValidatorAddress: valAddr.String(),
					SupernodeAccount: creatorAddr.String(),
					States: []*types.SuperNodeStateRecord{
						{
							State:  types.SuperNodeStateActive,
							Height: 100,
						},
					},
					PrevIpAddresses: []*types.IPAddressHistory{
						{
							Address: "192.168.1.1",
							Height:  100,
						},
					},
					PrevSupernodeAccounts: []*types.SupernodeAccountHistory{
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

			if tc.name == "cannot re-register STOPPED supernode" {
				// Create a stopped supernode
				stoppedSupernode := types.SuperNode{
					ValidatorAddress: valAddr.String(),
					SupernodeAccount: creatorAddr.String(),
					States: []*types.SuperNodeStateRecord{
						{
							State:  types.SuperNodeStateActive,
							Height: 100,
						},
						{
							State:  types.SuperNodeStateStopped,
							Height: 200,
						},
					},
					PrevIpAddresses: []*types.IPAddressHistory{
						{
							Address: "192.168.1.1",
							Height:  100,
						},
					},
					PrevSupernodeAccounts: []*types.SupernodeAccountHistory{
						{
							Account: creatorAddr.String(),
							Height:  100,
						},
					},
					P2PPort: "26657",
				}
				err := k.SetSuperNode(sdkCtx, stoppedSupernode)
				require.NoError(t, err)
			}

			if tc.name == "cannot re-register PENALIZED supernode" {
				// Create a penalized supernode
				penalizedSupernode := types.SuperNode{
					ValidatorAddress: valAddr.String(),
					SupernodeAccount: creatorAddr.String(),
					States: []*types.SuperNodeStateRecord{
						{
							State:  types.SuperNodeStateActive,
							Height: 100,
						},
						{
							State:  types.SuperNodeStatePenalized,
							Height: 200,
						},
					},
					PrevIpAddresses: []*types.IPAddressHistory{
						{
							Address: "192.168.1.1",
							Height:  100,
						},
					},
					PrevSupernodeAccounts: []*types.SupernodeAccountHistory{
						{
							Account: creatorAddr.String(),
							Height:  100,
						},
					},
					P2PPort: "26657",
				}
				err := k.SetSuperNode(sdkCtx, penalizedSupernode)
				require.NoError(t, err)
			}

			if tc.name == "re-registration ignores new parameters (IP, account, port)" ||
				tc.name == "re-registration fails when validator becomes jailed" ||
				tc.name == "re-registration fails when validator loses eligibility" {
				// Create a disabled supernode
				disabledSupernode := types.SuperNode{
					ValidatorAddress: valAddr.String(),
					SupernodeAccount: creatorAddr.String(), // Original account
					States: []*types.SuperNodeStateRecord{
						{
							State:  types.SuperNodeStateActive,
							Height: 100,
						},
						{
							State:  types.SuperNodeStateDisabled,
							Height: 200,
						},
					},
					PrevIpAddresses: []*types.IPAddressHistory{
						{
							Address: "192.168.1.1", // Original IP
							Height:  100,
						},
					},
					PrevSupernodeAccounts: []*types.SupernodeAccountHistory{
						{
							Account: creatorAddr.String(), // Original account
							Height:  100,
						},
					},
					P2PPort: "26657", // Original port
				}
				err := k.SetSuperNode(sdkCtx, disabledSupernode)
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

				// Additional assertions for re-registration tests
				if tc.name == "re-registration of disabled supernode" {
					// Verify the supernode is now active
					sn, found := k.QuerySuperNode(sdkCtx, valAddr)
					require.True(t, found)
					require.Len(t, sn.States, 3) // Initial active, disabled, then active again
					require.Equal(t, types.SuperNodeStateActive, sn.States[2].State)

					// Verify IP address and account were NOT updated
					require.Equal(t, "192.168.1.1", sn.PrevIpAddresses[len(sn.PrevIpAddresses)-1].Address)
					require.Equal(t, creatorAddr.String(), sn.SupernodeAccount)
					require.Len(t, sn.PrevIpAddresses, 1)       // No new IP history
					require.Len(t, sn.PrevSupernodeAccounts, 1) // No new account history

					// Verify event attributes are present and correct
					evs := sdkCtx.EventManager().Events()
					foundEvt := false
                    for _, e := range evs {
                        if e.Type != types.EventTypeSupernodeRegistered {
                            continue
                        }
                        kv := map[string]string{}
                        for _, a := range e.Attributes {
                            kv[string(a.Key)] = string(a.Value)
                        }
                        
                        rereg := kv[types.AttributeKeyReRegistered] == "true"
                        oldst := kv[types.AttributeKeyOldState] == types.SuperNodeStateDisabled.String()
                        ipok := kv[types.AttributeKeyIPAddress] == "192.168.1.1"
                        accok := kv[types.AttributeKeySupernodeAccount] == creatorAddr.String()
                        p2pok := kv[types.AttributeKeyP2PPort] == "26657"
                        valok := kv[types.AttributeKeyValidatorAddress] == valAddr.String()
                        htok := kv[types.AttributeKeyHeight] == fmt.Sprintf("%d", sdkCtx.BlockHeight())
                        
                        if rereg && oldst && ipok && accok && p2pok && valok && htok {
                            foundEvt = true
                            break
                        }
                    }
					require.True(t, foundEvt, "re-registration event with expected attributes not found")
				}

				if tc.name == "re-registration ignores new parameters (IP, account, port)" {
					// Verify the supernode is now active but parameters remain unchanged
					sn, found := k.QuerySuperNode(sdkCtx, valAddr)
					require.True(t, found)
					require.Len(t, sn.States, 3) // Initial active, disabled, then active again
					require.Equal(t, types.SuperNodeStateActive, sn.States[2].State)

					// Verify ALL original parameters were preserved (not updated)
					require.Equal(t, "192.168.1.1", sn.PrevIpAddresses[len(sn.PrevIpAddresses)-1].Address) // Original IP kept
					require.Equal(t, creatorAddr.String(), sn.SupernodeAccount)                            // Original account kept
					require.Equal(t, "26657", sn.P2PPort)                                                  // Original port kept
					require.Len(t, sn.PrevIpAddresses, 1)                                                  // No new IP history
					require.Len(t, sn.PrevSupernodeAccounts, 1)                                            // No new account history

					// Verify event attributes are present and correct
					evs := sdkCtx.EventManager().Events()
					foundEvt := false
                    for _, e := range evs {
                        if e.Type != types.EventTypeSupernodeRegistered {
                            continue
                        }
                        kv := map[string]string{}
                        for _, a := range e.Attributes {
                            kv[string(a.Key)] = string(a.Value)
                        }
                        
                        rereg := kv[types.AttributeKeyReRegistered] == "true"
                        oldst := kv[types.AttributeKeyOldState] == types.SuperNodeStateDisabled.String()
                        ipok := kv[types.AttributeKeyIPAddress] == "192.168.1.1"
                        accok := kv[types.AttributeKeySupernodeAccount] == creatorAddr.String()
                        p2pok := kv[types.AttributeKeyP2PPort] == "26657"
                        valok := kv[types.AttributeKeyValidatorAddress] == valAddr.String()
                        htok := kv[types.AttributeKeyHeight] == fmt.Sprintf("%d", sdkCtx.BlockHeight())
                        
                        if rereg && oldst && ipok && accok && p2pok && valok && htok {
                            foundEvt = true
                            break
                        }
                    }
					require.True(t, foundEvt, "re-registration event with expected attributes not found")
				}
			}
		})
	}
}

// setupKeeperForTest is your existing function
func setupKeeperForTest(
	t testing.TB,
	stakingKeeper types.StakingKeeper,
	slashingKeeper types.SlashingKeeper,
	bankKeeper types.BankKeeper,
) (keeper.Keeper, sdk.Context) {
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

	sdkCtx := sdk.NewContext(stateStore, cmtproto.Header{}, false, log.NewNopLogger())
	sdkCtx = sdkCtx.WithBlockTime(time.Now())

	// Set default params => min self-stake = 1,000,000
	params := types.DefaultParams()
	params.MinimumStakeForSn = sdk.NewInt64Coin("ulume", 1_000_000)
	err := k.SetParams(sdkCtx, params)
	require.NoError(t, err)

	return k, sdkCtx
}
