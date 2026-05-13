package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func TestPruneOldEpochsPrunesOnlyInactiveZeroTicketDeteriorationStates(t *testing.T) {
	f := initFixture(t)

	params := types.DefaultParams()
	params.KeepLastEpochEntries = 10
	currentEpoch := uint64(100)

	states := []types.TicketDeteriorationState{
		{TicketId: "old-zero-inactive", LastUpdatedEpoch: 89, DeteriorationScore: 0, ActiveHealOpId: 0},
		{TicketId: "boundary-zero-inactive", LastUpdatedEpoch: 90, DeteriorationScore: 0, ActiveHealOpId: 0},
		{TicketId: "recent-zero-inactive", LastUpdatedEpoch: 95, DeteriorationScore: 0, ActiveHealOpId: 0},
		{TicketId: "old-nonzero-inactive", LastUpdatedEpoch: 80, DeteriorationScore: 1, ActiveHealOpId: 0},
		{TicketId: "old-zero-active", LastUpdatedEpoch: 80, DeteriorationScore: 0, ActiveHealOpId: 7},
	}
	for _, state := range states {
		require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, state))
	}

	// st/tac/ has the same ticket-id key shape but is intentionally not pruned
	// in this phase until finalize-state safety is settled.
	require.NoError(t, f.keeper.SetTicketArtifactCountState(f.ctx, types.TicketArtifactCountState{
		TicketId:            "old-zero-inactive",
		IndexArtifactCount:  3,
		SymbolArtifactCount: 5,
	}))

	require.NoError(t, f.keeper.PruneOldEpochs(f.ctx, currentEpoch, params))

	require.False(t, f.keeper.HasTicketDeteriorationState(f.ctx, "old-zero-inactive"))
	for _, ticketID := range []string{
		"boundary-zero-inactive",
		"recent-zero-inactive",
		"old-nonzero-inactive",
		"old-zero-active",
	} {
		require.True(t, f.keeper.HasTicketDeteriorationState(f.ctx, ticketID), ticketID)
	}

	artifactCount, found := f.keeper.GetTicketArtifactCountState(f.ctx, "old-zero-inactive")
	require.True(t, found)
	require.Equal(t, uint32(3), artifactCount.IndexArtifactCount)
	require.Equal(t, uint32(5), artifactCount.SymbolArtifactCount)
}
