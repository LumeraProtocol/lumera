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

func TestValidateScoreStatesGenesis_TicketArtifactCountStates(t *testing.T) {
	const currentEpoch uint64 = 10

	t.Run("empty ticket id rejected", func(t *testing.T) {
		g := types.GenesisState{
			TicketArtifactCountStates: []types.TicketArtifactCountState{
				{IndexArtifactCount: 1, SymbolArtifactCount: 2},
			},
		}
		err := types.ValidateScoreStatesGenesis(g, currentEpoch)
		require.Error(t, err)
		require.Contains(t, err.Error(), "empty ticket id")
	})

	t.Run("zero counts rejected", func(t *testing.T) {
		g := types.GenesisState{
			TicketArtifactCountStates: []types.TicketArtifactCountState{
				{TicketId: "ticket-zero"},
			},
		}
		err := types.ValidateScoreStatesGenesis(g, currentEpoch)
		require.Error(t, err)
		require.Contains(t, err.Error(), "zero index and symbol counts")
	})

	t.Run("nonzero count accepted", func(t *testing.T) {
		g := types.GenesisState{
			TicketArtifactCountStates: []types.TicketArtifactCountState{
				{TicketId: "ticket-ok", IndexArtifactCount: 1},
			},
		}
		require.NoError(t, types.ValidateScoreStatesGenesis(g, currentEpoch))
	})
}
