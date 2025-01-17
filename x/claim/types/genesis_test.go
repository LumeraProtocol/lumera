package types_test

import (
	"fmt"
	"testing"
	"time"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/pastelnetwork/pastel/x/claim/types"
	"github.com/stretchr/testify/require"
)

func TestGenesisState_Validate(t *testing.T) {
	// Setup common test values
	validEndTime := time.Now().Add(time.Hour * 48).Unix()

	// Create valid module account
	moduleAcc := authtypes.NewEmptyModuleAccount(
		types.ModuleName,
		authtypes.Minter,
		authtypes.Burner,
	)

	testCases := []struct {
		name     string
		genState *types.GenesisState
		expErr   bool
	}{
		{
			name:     "default genesis state",
			genState: types.DefaultGenesis(),
			expErr:   false,
		},
		{
			name: "valid genesis state - empty records",
			genState: types.NewGenesisState(
				types.Params{
					EnableClaims:      true,
					ClaimEndTime:      validEndTime,
					MaxClaimsPerBlock: 50,
				},
				[]types.ClaimRecord{}, // Should always be empty in genesis
				moduleAcc,
				types.DefaultClaimableAmountConst, // Fixed claimable amount
				types.DefaultClaimsDenom,
			),
			expErr: false,
		},
		{
			name: "invalid params - negative end time",
			genState: types.NewGenesisState(
				types.Params{
					EnableClaims:      true,
					ClaimEndTime:      -1,
					MaxClaimsPerBlock: 50,
				},
				[]types.ClaimRecord{},
				moduleAcc,
				types.DefaultClaimableAmountConst,
				types.DefaultClaimsDenom,
			),
			expErr: true,
		},
		{
			name: "invalid params - zero max claims",
			genState: types.NewGenesisState(
				types.Params{
					EnableClaims:      true,
					ClaimEndTime:      validEndTime,
					MaxClaimsPerBlock: 0,
				},
				[]types.ClaimRecord{},
				moduleAcc,
				types.DefaultClaimableAmountConst,
				types.DefaultClaimsDenom,
			),
			expErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.genState.Validate()

			if tc.expErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDefaultGenesis(t *testing.T) {
	genState := types.DefaultGenesis()

	require.NotNil(t, genState)
	require.Equal(t, types.DefaultParams(), genState.Params)
	require.Empty(t, genState.ClaimRecords, "Genesis claim records should be empty")
	require.Equal(t, uint64(types.DefaultClaimableAmountConst), genState.TotalClaimableAmount, fmt.Sprintf("total claimable amount should be fixed at %d", types.DefaultClaimableAmountConst))

	// Validate default genesis state
	require.NoError(t, genState.Validate())
}

func TestNewGenesisState(t *testing.T) {
	params := types.DefaultParams()
	moduleAcc := authtypes.NewEmptyModuleAccount(
		types.ModuleName,
		authtypes.Minter,
		authtypes.Burner,
	)

	genState := types.NewGenesisState(
		params,
		[]types.ClaimRecord{}, // Should be empty in genesis
		moduleAcc,
		types.DefaultClaimableAmountConst, // Fixed claimable amount
		types.DefaultClaimsDenom,
	)

	require.NotNil(t, genState)
	require.Equal(t, params, genState.Params)
	require.Empty(t, genState.ClaimRecords, "Genesis claim records should be empty")
	require.Equal(t, moduleAcc.String(), genState.ModuleAccount)
	require.Equal(t, uint64(types.DefaultClaimableAmountConst), genState.TotalClaimableAmount, fmt.Sprintf("Total claimable amount should be fixed at %d", types.DefaultClaimableAmountConst))
}
