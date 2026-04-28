package types_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	"github.com/stretchr/testify/require"
)

func TestValidateScoreStatesGenesis_WindowStartEpochInFuture(t *testing.T) {
	const currentEpoch uint64 = 10

	t.Run("node suspicion future window rejected", func(t *testing.T) {
		g := types.GenesisState{
			NodeSuspicionStates: []types.NodeSuspicionState{
				{
					SupernodeAccount: "lumera1aaaa",
					LastUpdatedEpoch: 5,
					WindowStartEpoch: currentEpoch + 10,
				},
			},
		}
		err := types.ValidateScoreStatesGenesis(g, currentEpoch)
		require.Error(t, err)
		require.Contains(t, err.Error(), "WindowStartEpoch")
	})

	t.Run("reporter reliability future window rejected", func(t *testing.T) {
		g := types.GenesisState{
			ReporterReliabilityStates: []types.ReporterReliabilityState{
				{
					ReporterSupernodeAccount: "lumera1bbbb",
					LastUpdatedEpoch:         5,
					WindowStartEpoch:         currentEpoch + 10,
				},
			},
		}
		err := types.ValidateScoreStatesGenesis(g, currentEpoch)
		require.Error(t, err)
		require.Contains(t, err.Error(), "WindowStartEpoch")
	})

	t.Run("non-future window accepted", func(t *testing.T) {
		g := types.GenesisState{
			NodeSuspicionStates: []types.NodeSuspicionState{
				{
					SupernodeAccount: "lumera1aaaa",
					LastUpdatedEpoch: 5,
					WindowStartEpoch: currentEpoch,
				},
			},
		}
		require.NoError(t, types.ValidateScoreStatesGenesis(g, currentEpoch))
	})
}
