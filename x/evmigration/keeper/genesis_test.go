package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"

	"github.com/stretchr/testify/require"
)

func TestGenesis(t *testing.T) {
	records := []types.MigrationRecord{
		{
			LegacyAddress:   "lumera1legacy1",
			NewAddress:      "lumera1new1",
			MigrationTime:   1000,
			MigrationHeight: 42,
		},
		{
			LegacyAddress:   "lumera1legacy2",
			NewAddress:      "lumera1new2",
			MigrationTime:   2000,
			MigrationHeight: 99,
		},
	}

	genesisState := types.GenesisState{
		Params:                  types.DefaultParams(),
		MigrationRecords:        records,
		TotalMigrated:           7,
		TotalValidatorsMigrated: 3,
	}

	f := initFixture(t)
	err := f.keeper.InitGenesis(f.ctx, genesisState)
	require.NoError(t, err)

	got, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.NotNil(t, got)

	// Params round-trip.
	require.EqualExportedValues(t, genesisState.Params, got.Params)

	// Migration records round-trip.
	require.Len(t, got.MigrationRecords, 2)
	require.Equal(t, records[0].LegacyAddress, got.MigrationRecords[0].LegacyAddress)
	require.Equal(t, records[0].NewAddress, got.MigrationRecords[0].NewAddress)
	require.Equal(t, records[0].MigrationTime, got.MigrationRecords[0].MigrationTime)
	require.Equal(t, records[0].MigrationHeight, got.MigrationRecords[0].MigrationHeight)
	require.Equal(t, records[1].LegacyAddress, got.MigrationRecords[1].LegacyAddress)
	require.Equal(t, records[1].NewAddress, got.MigrationRecords[1].NewAddress)

	// Counters round-trip.
	require.Equal(t, uint64(7), got.TotalMigrated)
	require.Equal(t, uint64(3), got.TotalValidatorsMigrated)
}

func TestGenesis_DefaultEmpty(t *testing.T) {
	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
	}

	f := initFixture(t)
	err := f.keeper.InitGenesis(f.ctx, genesisState)
	require.NoError(t, err)

	got, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.NotNil(t, got)

	require.EqualExportedValues(t, genesisState.Params, got.Params)
	require.Empty(t, got.MigrationRecords)
	require.Equal(t, uint64(0), got.TotalMigrated)
	require.Equal(t, uint64(0), got.TotalValidatorsMigrated)
}
