package types_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
	"github.com/stretchr/testify/require"
)

func TestGenesisState_Validate(t *testing.T) {
	tests := []struct {
		desc     string
		genState *types.GenesisState
		valid    bool
	}{
		{
			desc:     "default is valid",
			genState: types.DefaultGenesis(),
			valid:    true,
		},
		{
			desc: "valid genesis state with custom params",
			genState: &types.GenesisState{
				Params: types.NewParams(true, 1000000, 100, 3000),
			},
			valid: true,
		},
		{
			desc: "invalid: zero max_migrations_per_block",
			genState: &types.GenesisState{
				Params: types.NewParams(true, 1000000, 0, 2000),
			},
			valid: false,
		},
		{
			desc: "invalid: zero max_validator_delegations",
			genState: &types.GenesisState{
				Params: types.NewParams(true, 1000000, 50, 0),
			},
			valid: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.genState.Validate()
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
