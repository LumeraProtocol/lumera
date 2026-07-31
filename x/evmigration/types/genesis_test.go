package types_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestGenesisState_Validate(t *testing.T) {
	canaryAddress := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
	canaryParams := types.NewParams(true, 1000000, 100, 3000, 20)
	canaryParams.CanaryLegacyAddresses = []string{canaryAddress}
	invalidCanaryParams := canaryParams
	invalidCanaryParams.CanaryLegacyAddresses = []string{"not-an-address"}
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
				Params: types.NewParams(true, 1000000, 100, 3000, 20),
			},
			valid: true,
		},
		{
			desc:     "valid genesis state with canary",
			genState: &types.GenesisState{Params: canaryParams},
			valid:    true,
		},
		{
			desc:     "invalid genesis state with malformed canary",
			genState: &types.GenesisState{Params: invalidCanaryParams},
			valid:    false,
		},
		{
			desc: "invalid: zero max_migrations_per_block",
			genState: &types.GenesisState{
				Params: types.NewParams(true, 1000000, 0, 2000, 20),
			},
			valid: false,
		},
		{
			desc: "invalid: zero max_validator_delegations",
			genState: &types.GenesisState{
				Params: types.NewParams(true, 1000000, 50, 0, 20),
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
