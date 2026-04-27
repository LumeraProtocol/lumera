package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	"github.com/stretchr/testify/require"
)

func TestGenesisParamsRoundTrip(t *testing.T) {
	f := initFixture(t)

	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
	}

	err := f.keeper.InitGenesis(f.ctx, genesisState)
	require.NoError(t, err)

	got, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.NotNil(t, got)

	require.EqualExportedValues(t, genesisState.Params, got.Params)
	require.Equal(t, uint64(1), got.NextEvidenceId)
	require.Equal(t, uint64(1), got.NextHealOpId)
	require.Empty(t, got.Evidence)
	require.Empty(t, got.NodeSuspicionStates)
	require.Empty(t, got.ReporterReliabilityStates)
	require.Empty(t, got.TicketDeteriorationStates)
	require.Empty(t, got.TicketArtifactCountStates)
	require.Empty(t, got.HealOps)
}

func TestGenesisEvidenceRoundTripSetsNextID(t *testing.T) {
	f := initFixture(t)

	ev := types.Evidence{
		EvidenceId:      7,
		SubjectAddress:  "lumera1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqg7l7x8",
		ReporterAddress: "lumera1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqg7l7x8",
		ActionId:        "action-1",
		EvidenceType:    types.EvidenceType_EVIDENCE_TYPE_ACTION_EXPIRED,
		Metadata:        []byte{1, 2, 3},
		ReportedHeight:  10,
	}

	genesisState := types.GenesisState{
		Params:   types.DefaultParams(),
		Evidence: []types.Evidence{ev},
	}

	err := f.keeper.InitGenesis(f.ctx, genesisState)
	require.NoError(t, err)

	got, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)

	require.Len(t, got.Evidence, 1)
	require.Equal(t, ev.EvidenceId, got.Evidence[0].EvidenceId)
	require.Equal(t, uint64(8), got.NextEvidenceId)
	require.Equal(t, uint64(1), got.NextHealOpId)
}

func TestGenesisStorageTruthPostponementRoundTrip(t *testing.T) {
	f := initFixture(t)

	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
		StorageTruthPostponements: []types.StorageTruthPostponement{
			{SupernodeAccount: "lumera1aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa5xm4ep", PostponedAtEpochId: 5},
			{SupernodeAccount: "lumera1bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbadc7mh", PostponedAtEpochId: 7},
		},
	}

	require.NoError(t, f.keeper.InitGenesis(f.ctx, genesisState))

	got, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.Len(t, got.StorageTruthPostponements, 2)

	// Validate round-trip: all entries are recovered (order may vary).
	byAccount := make(map[string]uint64, len(got.StorageTruthPostponements))
	for _, p := range got.StorageTruthPostponements {
		byAccount[p.SupernodeAccount] = p.PostponedAtEpochId
	}
	require.Equal(t, uint64(5), byAccount["lumera1aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa5xm4ep"])
	require.Equal(t, uint64(7), byAccount["lumera1bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbadc7mh"])
}

func TestGenesisRoundTripWithTicketArtifactCountStates(t *testing.T) {
	f := initFixture(t)

	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
		TicketArtifactCountStates: []types.TicketArtifactCountState{
			{
				TicketId:            "ticket-1",
				IndexArtifactCount:  32,
				SymbolArtifactCount: 128,
			},
		},
	}

	require.NoError(t, f.keeper.InitGenesis(f.ctx, genesisState))

	got, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.Len(t, got.TicketArtifactCountStates, 1)
	require.Equal(t, genesisState.TicketArtifactCountStates[0], got.TicketArtifactCountStates[0])
}
