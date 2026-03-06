package types

import "cosmossdk.io/collections"

const (
	// ModuleName defines the module name
	ModuleName = "evmigration"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// GovModuleName duplicates the gov module's name to avoid a dependency with x/gov.
	GovModuleName = "gov"
)

var (
	// ParamsKey is the prefix to retrieve all Params
	ParamsKey = collections.NewPrefix("p_evmigration")

	// MigrationRecordKeyPrefix is the prefix for migration records keyed by legacy address.
	MigrationRecordKeyPrefix = collections.NewPrefix("mr_")

	// MigrationCounterKey stores the total_migrated counter.
	MigrationCounterKey = collections.NewPrefix("mc_")

	// ValidatorMigrationCounterKey stores the total_validators_migrated counter.
	ValidatorMigrationCounterKey = collections.NewPrefix("vmc_")

	// BlockMigrationCounterPrefix stores per-block migration count (keyed by block height).
	BlockMigrationCounterPrefix = collections.NewPrefix("bmc_")
)
