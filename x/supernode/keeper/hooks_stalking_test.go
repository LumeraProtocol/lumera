package keeper_test

import (
	supernodemocks "github.com/pastelnetwork/pastel/x/supernode/mocks"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/pastelnetwork/pastel/x/supernode/keeper"
	"github.com/stretchr/testify/require"
)

func TestSupernodeHooks(t *testing.T) {
	ctl := gomock.NewController(t)
	defer ctl.Finish()

	mockKeeper := supernodemocks.NewMockSupernodeKeeper(ctl)
	hooks := keeper.NewSupernodeHooks(mockKeeper)
	ctx := sdk.Context{}

	tests := []struct {
		name          string
		setupMock     func()
		hook          func() error
		expectedError string
	}{
		{
			name: "AfterValidatorCreated - meets requirements",
			setupMock: func() {
				mockKeeper.EXPECT().MeetsSuperNodeRequirements(gomock.Any(), sdk.ValAddress("validator1")).Return(true)
				mockKeeper.EXPECT().EnableSuperNode(gomock.Any(), sdk.ValAddress("validator1")).Return(nil)
			},
			hook: func() error {
				return hooks.AfterValidatorCreated(ctx, sdk.ValAddress("validator1"))
			},
			expectedError: "",
		},
		{
			name: "BeforeValidatorModified - disable supernode",
			setupMock: func() {
				mockKeeper.EXPECT().MeetsSuperNodeRequirements(gomock.Any(), sdk.ValAddress("validator1")).Return(false)
				mockKeeper.EXPECT().IsSuperNodeActive(gomock.Any(), sdk.ValAddress("validator1")).Return(true)
				mockKeeper.EXPECT().DisableSuperNode(gomock.Any(), sdk.ValAddress("validator1")).Return(nil)
			},
			hook: func() error {
				return hooks.BeforeValidatorModified(ctx, sdk.ValAddress("validator1"))
			},
			expectedError: "",
		},
		{
			name: "BeforeDelegationSharesModified - enable supernode",
			setupMock: func() {
				mockKeeper.EXPECT().MeetsSuperNodeRequirements(gomock.Any(), sdk.ValAddress("validator1")).Return(true)
				mockKeeper.EXPECT().IsSuperNodeActive(gomock.Any(), sdk.ValAddress("validator1")).Return(false)
				mockKeeper.EXPECT().EnableSuperNode(gomock.Any(), sdk.ValAddress("validator1")).Return(nil)
			},
			hook: func() error {
				return hooks.BeforeDelegationSharesModified(ctx, sdk.AccAddress("delegator1"), sdk.ValAddress("validator1"))
			},
			expectedError: "",
		},
		{
			name: "AfterValidatorRemoved - disable supernode",
			setupMock: func() {
				mockKeeper.EXPECT().IsSuperNodeActive(gomock.Any(), sdk.ValAddress("validator1")).Return(true)
				mockKeeper.EXPECT().DisableSuperNode(gomock.Any(), sdk.ValAddress("validator1")).Return(nil)
			},
			hook: func() error {
				return hooks.AfterValidatorRemoved(ctx, sdk.ConsAddress("cons1"), sdk.ValAddress("validator1"))
			},
			expectedError: "",
		},
		{
			name: "AfterValidatorBeginUnbonding - disable supernode",
			setupMock: func() {
				mockKeeper.EXPECT().IsSuperNodeActive(gomock.Any(), sdk.ValAddress("validator1")).Return(true)
				mockKeeper.EXPECT().DisableSuperNode(gomock.Any(), sdk.ValAddress("validator1")).Return(nil)
			},
			hook: func() error {
				return hooks.AfterValidatorBeginUnbonding(ctx, sdk.ConsAddress("cons1"), sdk.ValAddress("validator1"))
			},
			expectedError: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock()
			err := tc.hook()

			if tc.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectedError)
			}
		})
	}
}
