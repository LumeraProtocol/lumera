package keeper_test

import (
	"context"
	"errors"
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/pastelnetwork/pastel/x/supernode/keeper"
	supernodemocks "github.com/pastelnetwork/pastel/x/supernode/mocks"
)

func TestCheckValidatorSupernodeEligibility(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Example val address
	valOperatorAddr := sdk.ValAddress([]byte("valoper-test"))
	valAddrString := valOperatorAddr.String()

	// Test cases
	testCases := []struct {
		name                 string
		validator            *stakingtypes.Validator
		selfDelegationFound  bool
		selfDelegationShares sdkmath.LegacyDec

		expectErr bool
		errSubstr string
	}{
		{
			name: "validator is bonded => skip checks => no error",
			validator: &stakingtypes.Validator{
				OperatorAddress: valAddrString,
				Status:          stakingtypes.Bonded,
			},
			selfDelegationFound: false,
			expectErr:           false,
		},
		{
			name: "validator unbonded, but no self-delegation => error",
			validator: &stakingtypes.Validator{
				OperatorAddress: valAddrString,
				Status:          stakingtypes.Unbonded,
			},
			selfDelegationFound: false,
			expectErr:           true,
			errSubstr:           "no self-delegation",
		},
		{
			name: "validator unbonded, self-delegation < min => error",
			validator: &stakingtypes.Validator{
				OperatorAddress: valAddrString,
				Status:          stakingtypes.Unbonded,
				DelegatorShares: sdkmath.LegacyNewDec(1000000),
				Tokens:          sdkmath.NewInt(1000000),
			},
			selfDelegationFound:  true,
			selfDelegationShares: sdkmath.LegacyNewDec(500000),
			expectErr:            true,
			errSubstr:            "does not meet minimum self stake",
		},
		{
			name: "validator unbonded, self-delegation >= min => no error",
			validator: &stakingtypes.Validator{
				OperatorAddress: valAddrString,
				Status:          stakingtypes.Unbonded,
				DelegatorShares: sdkmath.LegacyNewDec(1000000),
				Tokens:          sdkmath.NewInt(1000000),
			},
			selfDelegationFound:  true,
			selfDelegationShares: sdkmath.LegacyNewDec(1000000),
			expectErr:            false,
		},
		{
			name: "delegation share 0, shouldn't panic => error",
			validator: &stakingtypes.Validator{
				OperatorAddress: valAddrString,
				Status:          stakingtypes.Unbonded,
				Tokens:          sdkmath.NewInt(1000000),
				DelegatorShares: sdkmath.LegacyNewDec(0),
			},
			selfDelegationFound:  true,
			selfDelegationShares: sdkmath.LegacyNewDec(500000),
			expectErr:            true,
			errSubstr:            "no self-stake available",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Mock out the Delegation(...) call

			stakingKeeper := supernodemocks.NewMockStakingKeeper(ctrl)
			slashingKeeper := supernodemocks.NewMockSlashingKeeper(ctrl)
			bankKeeper := supernodemocks.NewMockBankKeeper(ctrl)

			stakingKeeper.EXPECT().
				Delegation(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) (stakingtypes.Delegation, error) {
					if tc.selfDelegationFound {
						return stakingtypes.Delegation{
							DelegatorAddress: delAddr.String(),
							ValidatorAddress: valAddr.String(),
							Shares:           tc.selfDelegationShares,
						}, nil
					}
					return stakingtypes.Delegation{}, errors.New("no self-delegation")
				}).
				MaxTimes(1)

			k, ctx := setupKeeperForTest(t, stakingKeeper, slashingKeeper, bankKeeper)
			msgServer := keeper.NewMsgServerImpl(k)

			// Call your function
			err := msgServer.CheckValidatorSupernodeEligibility(ctx, tc.validator, valAddrString)
			if tc.expectErr {
				require.Error(t, err)
				if tc.errSubstr != "" {
					require.Contains(t, err.Error(), tc.errSubstr)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
