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
	require.Empty(t, got.Evidence)
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
}
