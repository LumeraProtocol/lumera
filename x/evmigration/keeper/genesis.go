package keeper

import (
	"context"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func (k Keeper) InitGenesis(ctx context.Context, genState types.GenesisState) error {
	if err := k.Params.Set(ctx, genState.Params); err != nil {
		return err
	}

	for _, record := range genState.MigrationRecords {
		if err := k.MigrationRecords.Set(ctx, record.LegacyAddress, record); err != nil {
			return err
		}
	}

	if err := k.MigrationCounter.Set(ctx, genState.TotalMigrated); err != nil {
		return err
	}

	return k.ValidatorMigrationCounter.Set(ctx, genState.TotalValidatorsMigrated)
}

// ExportGenesis returns the module's exported genesis.
func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	params, err := k.Params.Get(ctx)
	if err != nil {
		return nil, err
	}

	var records []types.MigrationRecord
	err = k.MigrationRecords.Walk(ctx, nil, func(_ string, record types.MigrationRecord) (bool, error) {
		records = append(records, record)
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	totalMigrated, err := k.MigrationCounter.Get(ctx)
	if err != nil {
		return nil, err
	}

	totalValMigrated, err := k.ValidatorMigrationCounter.Get(ctx)
	if err != nil {
		return nil, err
	}

	return &types.GenesisState{
		Params:                   params,
		MigrationRecords:         records,
		TotalMigrated:            totalMigrated,
		TotalValidatorsMigrated:  totalValMigrated,
	}, nil
}
